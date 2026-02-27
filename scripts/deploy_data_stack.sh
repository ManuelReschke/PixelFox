#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

DB_TEMPLATE="${REPO_ROOT}/docker/prod/db.compose.yml"
CACHE_TEMPLATE="${REPO_ROOT}/docker/prod/cache.compose.yml"

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

if [[ ! -f "${DB_TEMPLATE}" || ! -f "${CACHE_TEMPLATE}" ]]; then
  echo "Fehler: docker/prod Templates nicht gefunden. Skript aus dem Repo ausfuehren." >&2
  exit 1
fi

require_cmd docker
if ! docker compose version >/dev/null 2>&1; then
  echo "Fehler: docker compose plugin nicht verfuegbar." >&2
  exit 1
fi

echo "Interaktiver Deploy: MySQL + Dragonfly (Data-Server)"
echo

DB_STACK_DIR="$(ask_default "DB Stack Verzeichnis" "/srv/mysql")"
DB_DATA_DIR="$(ask_default "DB Daten-Verzeichnis" "/srv/mysql/data")"
DB_NAME="$(ask_default "DB Name" "pixelfox_db")"
DB_USER="$(ask_default "DB User" "pixelfox")"

if [[ "$(ask_yes_no "DB Root Passwort automatisch generieren?" "y")" == "y" ]]; then
  DB_ROOT_PASSWORD="$(random_secret)"
else
  DB_ROOT_PASSWORD="$(ask_secret_required "DB Root Passwort")"
fi

if [[ "$(ask_yes_no "DB User Passwort automatisch generieren?" "y")" == "y" ]]; then
  DB_PASSWORD="$(random_secret)"
else
  DB_PASSWORD="$(ask_secret_required "DB User Passwort")"
fi

echo
CACHE_STACK_DIR="$(ask_default "Cache Stack Verzeichnis" "/srv/dragonfly")"
CACHE_DATA_DIR="$(ask_default "Cache Daten-Verzeichnis" "/srv/dragonfly/data")"

CACHE_AUTH_ENABLED="n"
CACHE_PASSWORD=""
if [[ "$(ask_yes_no "Cache Passwortschutz aktivieren (empfohlen)?" "y")" == "y" ]]; then
  CACHE_AUTH_ENABLED="y"
  if [[ "$(ask_yes_no "Cache Passwort automatisch generieren?" "y")" == "y" ]]; then
    CACHE_PASSWORD="$(random_secret)"
  else
    CACHE_PASSWORD="$(ask_secret_required "Cache Passwort")"
  fi
fi

echo
APPLY_UFW="$(ask_yes_no "UFW-Regeln fuer DB/Cache setzen?" "n")"
UFW_ALLOWED_IPS_CSV=""
UFW_ENABLE="n"
if [[ "${APPLY_UFW}" == "y" ]]; then
  UFW_ALLOWED_IPS_CSV="$(ask_required "Erlaubte Source-IP(s) fuer App/Storage (CSV, z.B. 1.2.3.4,5.6.7.8)")"
  UFW_ENABLE="$(ask_yes_no "UFW aktivieren, falls noch deaktiviert?" "n")"
fi

echo
echo "Zusammenfassung:"
echo "  DB Stack Dir:        ${DB_STACK_DIR}"
echo "  DB Data Dir:         ${DB_DATA_DIR}"
echo "  DB Name/User:        ${DB_NAME} / ${DB_USER}"
echo "  DB Root Passwort:    $(mask_secret "${DB_ROOT_PASSWORD}")"
echo "  DB User Passwort:    $(mask_secret "${DB_PASSWORD}")"
echo "  Cache Stack Dir:     ${CACHE_STACK_DIR}"
echo "  Cache Data Dir:      ${CACHE_DATA_DIR}"
if [[ "${CACHE_AUTH_ENABLED}" == "y" ]]; then
  echo "  Cache Auth:          aktiv"
  echo "  Cache Passwort:      $(mask_secret "${CACHE_PASSWORD}")"
else
  echo "  Cache Auth:          deaktiviert"
fi
if [[ "${APPLY_UFW}" == "y" ]]; then
  echo "  UFW Regeln:          aktiv"
  echo "  Erlaubte IPs:        ${UFW_ALLOWED_IPS_CSV}"
else
  echo "  UFW Regeln:          keine Aenderung"
fi
echo

if [[ "$(ask_yes_no "Deployment jetzt ausfuehren?" "y")" != "y" ]]; then
  echo "Abgebrochen."
  exit 0
fi

echo
echo "Erstelle Verzeichnisse..."
run_privileged mkdir -p "${DB_STACK_DIR}" "${DB_DATA_DIR}" "${CACHE_STACK_DIR}" "${CACHE_DATA_DIR}"
if [[ "${EUID}" -ne 0 ]]; then
  run_privileged chown -R "$(id -un):$(id -gn)" "${DB_STACK_DIR}" "${CACHE_STACK_DIR}" || true
fi

DB_COMPOSE_FILE="${DB_STACK_DIR}/docker-compose.yml"
DB_ENV_FILE="${DB_STACK_DIR}/.env"
CACHE_COMPOSE_FILE="${CACHE_STACK_DIR}/docker-compose.yml"
CACHE_ENV_FILE="${CACHE_STACK_DIR}/.env"

echo "Schreibe DB Compose + .env..."
cp "${DB_TEMPLATE}" "${DB_COMPOSE_FILE}"
umask 077
cat >"${DB_ENV_FILE}" <<EOF
DB_ROOT_PASSWORD=${DB_ROOT_PASSWORD}
DB_NAME=${DB_NAME}
DB_USER=${DB_USER}
DB_PASSWORD=${DB_PASSWORD}
DB_DATA_DIR=${DB_DATA_DIR}
EOF
umask 022

echo "Schreibe Cache Compose + .env..."
if [[ "${CACHE_AUTH_ENABLED}" == "y" ]]; then
  cat >"${CACHE_COMPOSE_FILE}" <<'EOF'
services:
  cache:
    image: docker.dragonflydb.io/dragonflydb/dragonfly
    container_name: pxlfox-cache
    restart: always
    command: ["dragonfly", "--cache_mode=true", "--requirepass", "${CACHE_PASSWORD}"]
    ports:
      - "6379:6379"
    volumes:
      - ${CACHE_DATA_DIR:-/srv/dragonfly/data}:/data
    env_file:
      - .env
EOF
else
  cp "${CACHE_TEMPLATE}" "${CACHE_COMPOSE_FILE}"
fi

umask 077
cat >"${CACHE_ENV_FILE}" <<EOF
CACHE_DATA_DIR=${CACHE_DATA_DIR}
CACHE_PASSWORD=${CACHE_PASSWORD}
EOF
umask 022

echo "Deploye DB..."
docker compose -f "${DB_COMPOSE_FILE}" --env-file "${DB_ENV_FILE}" pull || true
docker compose -f "${DB_COMPOSE_FILE}" --env-file "${DB_ENV_FILE}" up -d

echo "Deploye Cache..."
docker compose -f "${CACHE_COMPOSE_FILE}" --env-file "${CACHE_ENV_FILE}" pull || true
docker compose -f "${CACHE_COMPOSE_FILE}" --env-file "${CACHE_ENV_FILE}" up -d

echo "Pruefe Containerstatus..."
docker compose -f "${DB_COMPOSE_FILE}" --env-file "${DB_ENV_FILE}" ps
docker compose -f "${CACHE_COMPOSE_FILE}" --env-file "${CACHE_ENV_FILE}" ps

echo "MySQL Healthcheck..."
if docker compose -f "${DB_COMPOSE_FILE}" --env-file "${DB_ENV_FILE}" exec -T db \
  mysqladmin ping -h 127.0.0.1 -p"${DB_ROOT_PASSWORD}" --silent >/dev/null 2>&1; then
  echo "  MySQL erreichbar."
else
  echo "  Warnung: MySQL Healthcheck noch nicht erfolgreich (evtl. Start laeuft noch)."
fi

if [[ "${APPLY_UFW}" == "y" ]]; then
  if ! command -v ufw >/dev/null 2>&1; then
    echo "Warnung: ufw nicht gefunden, Regeln wurden uebersprungen."
  else
    echo "Setze UFW-Regeln..."
    run_privileged ufw allow OpenSSH
    run_privileged ufw deny 3306/tcp || true
    run_privileged ufw deny 6379/tcp || true

    IFS=',' read -r -a allowed_ips <<<"${UFW_ALLOWED_IPS_CSV}"
    for raw_ip in "${allowed_ips[@]}"; do
      ip="$(trim "${raw_ip}")"
      if [[ -z "${ip}" ]]; then
        continue
      fi
      run_privileged ufw allow from "${ip}" to any port 3306 proto tcp
      run_privileged ufw allow from "${ip}" to any port 6379 proto tcp
    done

    if [[ "${UFW_ENABLE}" == "y" ]]; then
      run_privileged ufw --force enable
    fi
    run_privileged ufw status
  fi
fi

echo
echo "Fertig."
echo "DB .env:     ${DB_ENV_FILE}"
echo "Cache .env:  ${CACHE_ENV_FILE}"
echo
echo "Werte fuer App/Storage Nodes:"
echo "  DB_HOST=<IP dieses Data-VPS>"
echo "  DB_PORT=3306"
echo "  DB_NAME=${DB_NAME}"
echo "  DB_USER=${DB_USER}"
echo "  DB_PASSWORD=${DB_PASSWORD}"
echo "  CACHE_HOST=<IP dieses Data-VPS>"
echo "  CACHE_PORT=6379"
if [[ "${CACHE_AUTH_ENABLED}" == "y" ]]; then
  echo "  CACHE_PASSWORD=${CACHE_PASSWORD}"
else
  echo "  CACHE_PASSWORD=<leer>"
fi
