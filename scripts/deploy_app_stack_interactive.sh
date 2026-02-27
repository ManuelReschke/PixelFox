#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# shellcheck source=./deploy_common.sh
source "${SCRIPT_DIR}/deploy_common.sh"

APP_TEMPLATE="${REPO_ROOT}/docker/prod/app.compose.yml"
if [[ ! -f "${APP_TEMPLATE}" ]]; then
  echo "Fehler: ${APP_TEMPLATE} nicht gefunden." >&2
  exit 1
fi

ensure_docker_compose_ready

echo "Interaktiver Deploy: App-Server"
echo

STACK_DIR="$(ask_default "App Stack Verzeichnis" "/srv/pixelfox")"
UPLOADS_DIR="$(ask_default "Uploads-Verzeichnis" "/srv/pixelfox/uploads")"
TMP_DIR="$(ask_default "Temp-Verzeichnis" "/srv/pixelfox/tmp")"

APP_IMAGE="$(ask_default "App Image" "registry.example.com/pixelfox/app:latest")"
PUBLIC_DOMAIN="$(ask_default "Public Domain" "https://pixelfox.cc")"

DB_HOST="$(ask_required "DB Host (IP oder DNS)")"
DB_PORT="$(ask_default "DB Port" "3306")"
DB_NAME="$(ask_default "DB Name" "pixelfox_db")"
DB_USER="$(ask_default "DB User" "pixelfox")"
DB_PASSWORD="$(ask_secret_required "DB Passwort")"

CACHE_HOST="$(ask_required "Cache Host (IP oder DNS)")"
CACHE_PORT="$(ask_default "Cache Port" "6379")"
CACHE_PASSWORD="$(ask_secret_optional "Cache Passwort")"

if [[ "$(ask_yes_no "UPLOAD_TOKEN_SECRET automatisch generieren?" "y")" == "y" ]]; then
  UPLOAD_TOKEN_SECRET="$(random_secret)"
else
  UPLOAD_TOKEN_SECRET="$(ask_secret_required "UPLOAD_TOKEN_SECRET")"
fi

if [[ "$(ask_yes_no "REPLICATION_SECRET automatisch generieren?" "y")" == "y" ]]; then
  REPLICATION_SECRET="$(random_secret)"
else
  REPLICATION_SECRET="$(ask_secret_required "REPLICATION_SECRET")"
fi

SMTP_HOST="$(ask_optional "SMTP Host (optional)" "smtp.mailgun.org")"
SMTP_PORT="$(ask_optional "SMTP Port (optional)" "587")"
SMTP_USERNAME="$(ask_optional "SMTP Username (optional)")"
SMTP_PASSWORD="$(ask_secret_optional "SMTP Passwort")"
SMTP_SENDER="$(ask_optional "SMTP Sender (optional)" "postmaster@your-domain")"

HCAPTCHA_SITEKEY="$(ask_optional "HCAPTCHA_SITEKEY (optional)")"
HCAPTCHA_SECRET="$(ask_secret_optional "HCAPTCHA_SECRET")"
METRICS_PW="$(ask_secret_optional "PROTECTED_ROUTE_METRICS_PW (optional)")"

echo
echo "Zusammenfassung:"
echo "  Stack Dir:              ${STACK_DIR}"
echo "  Uploads/Tmp:            ${UPLOADS_DIR} / ${TMP_DIR}"
echo "  App Image:              ${APP_IMAGE}"
echo "  Public Domain:          ${PUBLIC_DOMAIN}"
echo "  DB:                     ${DB_HOST}:${DB_PORT} (${DB_NAME}/${DB_USER})"
echo "  Cache:                  ${CACHE_HOST}:${CACHE_PORT}"
echo "  Cache Passwort:         $(mask_secret "${CACHE_PASSWORD}")"
echo "  UPLOAD_TOKEN_SECRET:    $(mask_secret "${UPLOAD_TOKEN_SECRET}")"
echo "  REPLICATION_SECRET:     $(mask_secret "${REPLICATION_SECRET}")"
echo

if [[ "$(ask_yes_no "Deployment jetzt ausfuehren?" "y")" != "y" ]]; then
  echo "Abgebrochen."
  exit 0
fi

echo
echo "Erstelle Verzeichnisse..."
run_privileged mkdir -p "${STACK_DIR}" "${UPLOADS_DIR}" "${TMP_DIR}"
if [[ "${EUID}" -ne 0 ]]; then
  run_privileged chown -R "$(id -un):$(id -gn)" "${STACK_DIR}" || true
fi

COMPOSE_FILE="${STACK_DIR}/docker-compose.yml"
ENV_FILE="${STACK_DIR}/.env"

echo "Schreibe Compose und .env..."
cp "${APP_TEMPLATE}" "${COMPOSE_FILE}"
write_file_secure "${ENV_FILE}" "APP_IMAGE=${APP_IMAGE}
UPLOADS_DIR=${UPLOADS_DIR}
TMP_DIR=${TMP_DIR}
PUBLIC_DOMAIN=${PUBLIC_DOMAIN}
APP_ENV=prod
APP_HOST=0.0.0.0
APP_PORT=4000
DB_HOST=${DB_HOST}
DB_PORT=${DB_PORT}
DB_NAME=${DB_NAME}
DB_USER=${DB_USER}
DB_PASSWORD=${DB_PASSWORD}
CACHE_HOST=${CACHE_HOST}
CACHE_PORT=${CACHE_PORT}
CACHE_PASSWORD=${CACHE_PASSWORD}
UPLOAD_TOKEN_SECRET=${UPLOAD_TOKEN_SECRET}
REPLICATION_SECRET=${REPLICATION_SECRET}
SMTP_HOST=${SMTP_HOST}
SMTP_PORT=${SMTP_PORT}
SMTP_USERNAME=${SMTP_USERNAME}
SMTP_PASSWORD=${SMTP_PASSWORD}
SMTP_SENDER=${SMTP_SENDER}
HCAPTCHA_SITEKEY=${HCAPTCHA_SITEKEY}
HCAPTCHA_SECRET=${HCAPTCHA_SECRET}
PROTECTED_ROUTE_METRICS_PW=${METRICS_PW}"

echo "Deploye App..."
docker compose -f "${COMPOSE_FILE}" --env-file "${ENV_FILE}" pull || true
docker compose -f "${COMPOSE_FILE}" --env-file "${ENV_FILE}" up -d
docker compose -f "${COMPOSE_FILE}" --env-file "${ENV_FILE}" ps

echo
echo "Fertig."
echo "App Stack: ${STACK_DIR}"
echo ".env:      ${ENV_FILE}"
echo
echo "Naechster Schritt:"
echo "  Proxy deployen (separates Skript): scripts/deploy_proxy_stack_interactive.sh"
