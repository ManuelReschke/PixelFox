#!/usr/bin/env bash

set -euo pipefail

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Fehler: '$1' wurde nicht gefunden." >&2
    exit 1
  fi
}

run_privileged() {
  if [[ "${EUID}" -eq 0 ]]; then
    "$@"
    return
  fi
  if command -v sudo >/dev/null 2>&1; then
    sudo "$@"
    return
  fi
  "$@"
}

ask_default() {
  local prompt="$1"
  local default="$2"
  local answer=""
  read -r -p "${prompt} [${default}]: " answer
  if [[ -z "${answer}" ]]; then
    answer="${default}"
  fi
  printf '%s' "${answer}"
}

ask_required() {
  local prompt="$1"
  local answer=""
  while true; do
    read -r -p "${prompt}: " answer
    if [[ -n "${answer}" ]]; then
      printf '%s' "${answer}"
      return
    fi
    echo "Bitte einen Wert eingeben."
  done
}

ask_optional() {
  local prompt="$1"
  local default="${2:-}"
  local answer=""
  if [[ -n "${default}" ]]; then
    read -r -p "${prompt} [${default}]: " answer
    if [[ -z "${answer}" ]]; then
      answer="${default}"
    fi
  else
    read -r -p "${prompt}: " answer
  fi
  printf '%s' "${answer}"
}

ask_secret_required() {
  local prompt="$1"
  local answer=""
  while true; do
    read -r -s -p "${prompt}: " answer
    echo
    if [[ -n "${answer}" ]]; then
      if [[ "${answer}" =~ [[:space:]] ]]; then
        echo "Passwort darf keine Leerzeichen enthalten."
        continue
      fi
      printf '%s' "${answer}"
      return
    fi
    echo "Bitte einen Wert eingeben."
  done
}

ask_secret_optional() {
  local prompt="$1"
  local answer=""
  read -r -s -p "${prompt} (leer lassen fuer none): " answer
  echo
  if [[ "${answer}" =~ [[:space:]] ]]; then
    echo "Passwort darf keine Leerzeichen enthalten." >&2
    exit 1
  fi
  printf '%s' "${answer}"
}

ask_yes_no() {
  local prompt="$1"
  local default="$2"
  local answer=""
  local normalized_default=""

  case "${default}" in
    y|Y) normalized_default="y" ;;
    n|N) normalized_default="n" ;;
    *)
      echo "Interner Fehler: ungueltiger Default fuer ask_yes_no" >&2
      exit 1
      ;;
  esac

  while true; do
    if [[ "${normalized_default}" == "y" ]]; then
      read -r -p "${prompt} [Y/n]: " answer
    else
      read -r -p "${prompt} [y/N]: " answer
    fi

    if [[ -z "${answer}" ]]; then
      answer="${normalized_default}"
    fi

    case "${answer}" in
      y|Y) printf 'y'; return ;;
      n|N) printf 'n'; return ;;
      *) echo "Bitte mit y oder n antworten." ;;
    esac
  done
}

random_secret() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -base64 33 | tr -dc 'A-Za-z0-9' | head -c 32
    return
  fi
  tr -dc 'A-Za-z0-9' </dev/urandom | head -c 32
}

mask_secret() {
  local value="$1"
  if [[ -z "${value}" ]]; then
    printf '(leer)'
    return
  fi
  if ((${#value} <= 4)); then
    printf '****'
    return
  fi
  printf '%s***%s' "${value:0:2}" "${value: -2}"
}

trim() {
  local value="$1"
  value="${value#"${value%%[![:space:]]*}"}"
  value="${value%"${value##*[![:space:]]}"}"
  printf '%s' "${value}"
}

dotenv_get() {
  local file="$1"
  local key="$2"
  if [[ ! -f "${file}" ]]; then
    return 1
  fi
  local line
  line="$(grep -E "^${key}=" "${file}" | tail -n 1 || true)"
  if [[ -z "${line}" ]]; then
    return 1
  fi
  printf '%s' "${line#*=}"
}

ensure_docker_compose_ready() {
  require_cmd docker
  if ! docker compose version >/dev/null 2>&1; then
    echo "Fehler: docker compose plugin nicht verfuegbar." >&2
    exit 1
  fi
}

write_file_secure() {
  local path="$1"
  local content="$2"
  umask 077
  cat >"${path}" <<EOF
${content}
EOF
  umask 022
}
