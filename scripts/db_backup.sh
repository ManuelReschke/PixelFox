#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

COMMAND="${1:-help}"
if [[ $# -gt 0 ]]; then
  shift
fi

DB_SERVICE="${DB_SERVICE:-db}"
COMPOSE_FILE="${DB_COMPOSE_FILE:-${REPO_ROOT}/docker-compose.yml}"
ENV_FILE="${DB_ENV_FILE:-${REPO_ROOT}/.env}"
BACKUP_DIR="${DB_BACKUP_DIR:-${REPO_ROOT}/tmp/db_backups}"
KEEP_LAST="${DB_BACKUP_KEEP_LAST:-20}"
BACKUP_PREFIX="${DB_BACKUP_PREFIX:-pixelfox_db}"

print_usage() {
  cat <<'EOF'
Usage:
  scripts/db_backup.sh backup [--backup-dir DIR] [--keep-last N] [--compose-file FILE] [--env-file FILE] [--service NAME]
  scripts/db_backup.sh restore <FILE.sql|FILE.sql.gz> [--compose-file FILE] [--env-file FILE] [--service NAME]
  scripts/db_backup.sh list [--backup-dir DIR]
  scripts/db_backup.sh latest [--backup-dir DIR]
  scripts/db_backup.sh help

Environment overrides:
  DB_BACKUP_DIR, DB_BACKUP_KEEP_LAST, DB_BACKUP_PREFIX
  DB_COMPOSE_FILE, DB_ENV_FILE, DB_SERVICE

Examples:
  scripts/db_backup.sh backup --backup-dir /srv/backups/pixelfox
  scripts/db_backup.sh restore /srv/backups/pixelfox/pixelfox_db_20260228_020000.sql.gz
EOF
}

die() {
  printf 'Error: %s\n' "$*" >&2
  exit 1
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || die "Required command not found: $1"
}

compose_runner() {
  local args=()

  if [[ -n "${COMPOSE_FILE}" ]]; then
    [[ -f "${COMPOSE_FILE}" ]] || die "Compose file not found: ${COMPOSE_FILE}"
    args+=(-f "${COMPOSE_FILE}")
  fi

  if [[ -n "${ENV_FILE}" ]]; then
    [[ -f "${ENV_FILE}" ]] || die "Env file not found: ${ENV_FILE}"
    args+=(--env-file "${ENV_FILE}")
  fi

  if docker compose version >/dev/null 2>&1; then
    docker compose "${args[@]}" "$@"
    return 0
  fi

  if command -v docker-compose >/dev/null 2>&1; then
    docker-compose "${args[@]}" "$@"
    return 0
  fi

  die "Neither 'docker compose' nor 'docker-compose' is available"
}

is_integer() {
  [[ "$1" =~ ^[0-9]+$ ]]
}

latest_backup_file() {
  if [[ ! -d "${BACKUP_DIR}" ]]; then
    return 1
  fi

  local latest
  latest="$(find "${BACKUP_DIR}" -maxdepth 1 -type f \( -name "${BACKUP_PREFIX}_*.sql" -o -name "${BACKUP_PREFIX}_*.sql.gz" \) -print | sort | tail -n 1)"
  if [[ -z "${latest}" ]]; then
    return 1
  fi

  printf '%s\n' "${latest}"
}

parse_common_flags() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
    --backup-dir)
      shift
      [[ $# -gt 0 ]] || die "--backup-dir requires a value"
      BACKUP_DIR="$1"
      ;;
    --keep-last)
      shift
      [[ $# -gt 0 ]] || die "--keep-last requires a value"
      KEEP_LAST="$1"
      ;;
    --compose-file)
      shift
      [[ $# -gt 0 ]] || die "--compose-file requires a value"
      COMPOSE_FILE="$1"
      ;;
    --env-file)
      shift
      [[ $# -gt 0 ]] || die "--env-file requires a value"
      ENV_FILE="$1"
      ;;
    --service)
      shift
      [[ $# -gt 0 ]] || die "--service requires a value"
      DB_SERVICE="$1"
      ;;
    *)
      die "Unknown option: $1"
      ;;
    esac
    shift
  done
}

do_backup() {
  parse_common_flags "$@"

  require_command docker
  require_command gzip
  require_command find
  require_command mktemp

  is_integer "${KEEP_LAST}" || die "Keep-last value must be an integer"
  [[ "${KEEP_LAST}" -ge 1 ]] || die "Keep-last value must be >= 1"

  mkdir -p "${BACKUP_DIR}"

  local timestamp output_file temp_file
  timestamp="$(date +%Y%m%d_%H%M%S)"
  output_file="${BACKUP_DIR}/${BACKUP_PREFIX}_${timestamp}.sql.gz"
  temp_file="$(mktemp "${BACKUP_DIR}/.${BACKUP_PREFIX}_${timestamp}.tmp.XXXXXX")"

  trap 'rm -f "${temp_file}"' EXIT

  compose_runner exec -T "${DB_SERVICE}" sh -lc \
    'exec mysqldump --single-transaction --quick --routines --triggers --events --set-gtid-purged=OFF -uroot -p"$MYSQL_ROOT_PASSWORD" --databases "$MYSQL_DATABASE"' \
    | gzip -9 >"${temp_file}"

  mv "${temp_file}" "${output_file}"
  trap - EXIT

  # Keep only the newest N backups.
  local -a backups
  mapfile -t backups < <(
    find "${BACKUP_DIR}" -maxdepth 1 -type f \
      \( -name "${BACKUP_PREFIX}_*.sql" -o -name "${BACKUP_PREFIX}_*.sql.gz" \) \
      -print | sort
  )

  local total remove_count
  total="${#backups[@]}"
  if (( total > KEEP_LAST )); then
    remove_count=$((total - KEEP_LAST))
    for ((i = 0; i < remove_count; i++)); do
      rm -f "${backups[i]}"
    done
  fi

  printf 'Backup created: %s\n' "${output_file}"
}

do_restore() {
  [[ $# -ge 1 ]] || die "restore requires a backup file path"
  local input_file="$1"
  shift

  parse_common_flags "$@"

  require_command docker
  require_command gzip

  [[ -f "${input_file}" ]] || die "Backup file not found: ${input_file}"

  case "${input_file}" in
  *.sql.gz)
    gzip -dc "${input_file}" | compose_runner exec -T "${DB_SERVICE}" sh -lc \
      'exec mysql -uroot -p"$MYSQL_ROOT_PASSWORD"'
    ;;
  *.sql)
    compose_runner exec -T "${DB_SERVICE}" sh -lc \
      'exec mysql -uroot -p"$MYSQL_ROOT_PASSWORD"' <"${input_file}"
    ;;
  *)
    die "Unsupported file extension. Use .sql or .sql.gz"
    ;;
  esac

  printf 'Restore completed from: %s\n' "${input_file}"
}

do_list() {
  parse_common_flags "$@"

  if [[ ! -d "${BACKUP_DIR}" ]]; then
    printf 'No backup directory: %s\n' "${BACKUP_DIR}"
    return 0
  fi

  find "${BACKUP_DIR}" -maxdepth 1 -type f \
    \( -name "${BACKUP_PREFIX}_*.sql" -o -name "${BACKUP_PREFIX}_*.sql.gz" \) \
    -print | sort
}

do_latest() {
  parse_common_flags "$@"

  if ! latest_backup_file; then
    die "No backup file found in ${BACKUP_DIR}"
  fi
}

case "${COMMAND}" in
backup)
  do_backup "$@"
  ;;
restore)
  do_restore "$@"
  ;;
list)
  do_list "$@"
  ;;
latest)
  do_latest "$@"
  ;;
help | -h | --help)
  print_usage
  ;;
*)
  die "Unknown command: ${COMMAND}. Run 'scripts/db_backup.sh help'"
  ;;
esac
