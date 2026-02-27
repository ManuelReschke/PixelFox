#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# shellcheck source=./deploy_common.sh
source "${SCRIPT_DIR}/deploy_common.sh"

DB_TEMPLATE="${REPO_ROOT}/docker/prod/db.compose.yml"
if [[ ! -f "${DB_TEMPLATE}" ]]; then
  echo "Fehler: ${DB_TEMPLATE} nicht gefunden." >&2
  exit 1
fi

ensure_docker_compose_ready

echo "Interaktiver Deploy: MySQL (DB-Server)"
echo

STACK_DIR="$(ask_default "DB Stack Verzeichnis" "/srv/mysql")"
DATA_DIR="$(ask_default "DB Daten-Verzeichnis" "/srv/mysql/data")"
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

APPLY_UFW="n"
UFW_ALLOWED_IPS_CSV=""
UFW_ENABLE="n"
if [[ "$(ask_yes_no "UFW-Regeln fuer DB (3306) setzen?" "n")" == "y" ]]; then
  APPLY_UFW="y"
  UFW_ALLOWED_IPS_CSV="$(ask_required "Erlaubte Source-IP(s) (CSV, z.B. 1.2.3.4,5.6.7.8)")"
  UFW_ENABLE="$(ask_yes_no "UFW aktivieren, falls noch deaktiviert?" "n")"
fi

echo
echo "Zusammenfassung:"
echo "  Stack Dir:          ${STACK_DIR}"
echo "  Data Dir:           ${DATA_DIR}"
echo "  DB Name/User:       ${DB_NAME} / ${DB_USER}"
echo "  Root Passwort:      $(mask_secret "${DB_ROOT_PASSWORD}")"
echo "  User Passwort:      $(mask_secret "${DB_PASSWORD}")"
if [[ "${APPLY_UFW}" == "y" ]]; then
  echo "  UFW Regeln:         aktiv"
  echo "  Erlaubte IPs:       ${UFW_ALLOWED_IPS_CSV}"
else
  echo "  UFW Regeln:         keine Aenderung"
fi
echo

if [[ "$(ask_yes_no "Deployment jetzt ausfuehren?" "y")" != "y" ]]; then
  echo "Abgebrochen."
  exit 0
fi

echo
echo "Erstelle Verzeichnisse..."
run_privileged mkdir -p "${STACK_DIR}" "${DATA_DIR}"
if [[ "${EUID}" -ne 0 ]]; then
  run_privileged chown -R "$(id -un):$(id -gn)" "${STACK_DIR}" || true
fi

COMPOSE_FILE="${STACK_DIR}/docker-compose.yml"
ENV_FILE="${STACK_DIR}/.env"

echo "Schreibe Compose und .env..."
cp "${DB_TEMPLATE}" "${COMPOSE_FILE}"
write_file_secure "${ENV_FILE}" "DB_ROOT_PASSWORD=${DB_ROOT_PASSWORD}
DB_NAME=${DB_NAME}
DB_USER=${DB_USER}
DB_PASSWORD=${DB_PASSWORD}
DB_DATA_DIR=${DATA_DIR}"

echo "Deploye DB..."
docker compose -f "${COMPOSE_FILE}" --env-file "${ENV_FILE}" pull || true
docker compose -f "${COMPOSE_FILE}" --env-file "${ENV_FILE}" up -d
docker compose -f "${COMPOSE_FILE}" --env-file "${ENV_FILE}" ps

echo "MySQL Healthcheck..."
if docker compose -f "${COMPOSE_FILE}" --env-file "${ENV_FILE}" exec -T db \
  mysqladmin ping -h 127.0.0.1 -p"${DB_ROOT_PASSWORD}" --silent >/dev/null 2>&1; then
  echo "  MySQL erreichbar."
else
  echo "  Warnung: MySQL noch nicht erreichbar (Start kann noch laufen)."
fi

if [[ "${APPLY_UFW}" == "y" ]]; then
  if ! command -v ufw >/dev/null 2>&1; then
    echo "Warnung: ufw nicht gefunden, Regeln wurden uebersprungen."
  else
    echo "Setze UFW-Regeln..."
    run_privileged ufw allow OpenSSH
    run_privileged ufw deny 3306/tcp || true
    IFS=',' read -r -a allowed_ips <<<"${UFW_ALLOWED_IPS_CSV}"
    for raw_ip in "${allowed_ips[@]}"; do
      ip="$(trim "${raw_ip}")"
      if [[ -n "${ip}" ]]; then
        run_privileged ufw allow from "${ip}" to any port 3306 proto tcp
      fi
    done
    if [[ "${UFW_ENABLE}" == "y" ]]; then
      run_privileged ufw --force enable
    fi
    run_privileged ufw status
  fi
fi

echo
echo "Fertig."
echo "DB Stack: ${STACK_DIR}"
echo "DB .env:  ${ENV_FILE}"
echo
echo "Fuer App/Storage:"
echo "  DB_HOST=<IP dieses DB-VPS>"
echo "  DB_PORT=3306"
echo "  DB_NAME=${DB_NAME}"
echo "  DB_USER=${DB_USER}"
echo "  DB_PASSWORD=${DB_PASSWORD}"
