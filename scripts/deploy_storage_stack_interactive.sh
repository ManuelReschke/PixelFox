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

echo "Interaktiver Deploy: Storage-Node (s1/s2/...)"
echo

STACK_DIR="$(ask_default "Storage Stack Verzeichnis" "/srv/pixelfox-s1")"
UPLOADS_DIR="$(ask_default "Uploads-Verzeichnis" "/srv/pixelfox-s1/uploads")"
TMP_DIR="$(ask_default "Temp-Verzeichnis" "/srv/pixelfox-s1/tmp")"

NODE_ID="$(ask_default "Node ID" "s1")"
PUBLIC_DOMAIN="$(ask_default "Public Domain (Storage Host)" "https://images-s1.pixelfox.cc")"
APP_IMAGE="$(ask_default "App Image" "registry.example.com/pixelfox/app:latest")"

REFERENCE_ENV="$(ask_optional "Optional: bestehende App .env fuer Defaults" "/srv/pixelfox/.env")"

default_db_host=""
default_db_port="3306"
default_db_name="pixelfox_db"
default_db_user="pixelfox"
default_db_password=""
default_cache_host=""
default_cache_port="6379"
default_cache_password=""
default_upload_secret=""
default_replication_secret=""

if [[ -f "${REFERENCE_ENV}" ]]; then
  if [[ "$(ask_yes_no "Werte aus ${REFERENCE_ENV} als Defaults uebernehmen?" "y")" == "y" ]]; then
    default_db_host="$(dotenv_get "${REFERENCE_ENV}" "DB_HOST" || true)"
    default_db_port="$(dotenv_get "${REFERENCE_ENV}" "DB_PORT" || printf '%s' "${default_db_port}")"
    default_db_name="$(dotenv_get "${REFERENCE_ENV}" "DB_NAME" || printf '%s' "${default_db_name}")"
    default_db_user="$(dotenv_get "${REFERENCE_ENV}" "DB_USER" || printf '%s' "${default_db_user}")"
    default_db_password="$(dotenv_get "${REFERENCE_ENV}" "DB_PASSWORD" || true)"
    default_cache_host="$(dotenv_get "${REFERENCE_ENV}" "CACHE_HOST" || true)"
    default_cache_port="$(dotenv_get "${REFERENCE_ENV}" "CACHE_PORT" || printf '%s' "${default_cache_port}")"
    default_cache_password="$(dotenv_get "${REFERENCE_ENV}" "CACHE_PASSWORD" || true)"
    default_upload_secret="$(dotenv_get "${REFERENCE_ENV}" "UPLOAD_TOKEN_SECRET" || true)"
    default_replication_secret="$(dotenv_get "${REFERENCE_ENV}" "REPLICATION_SECRET" || true)"
  fi
fi

DB_HOST="$(ask_optional "DB Host (IP oder DNS)" "${default_db_host}")"
if [[ -z "${DB_HOST}" ]]; then
  DB_HOST="$(ask_required "DB Host (IP oder DNS)")"
fi
DB_PORT="$(ask_optional "DB Port" "${default_db_port}")"
DB_NAME="$(ask_optional "DB Name" "${default_db_name}")"
DB_USER="$(ask_optional "DB User" "${default_db_user}")"

if [[ -n "${default_db_password}" ]]; then
  if [[ "$(ask_yes_no "DB Passwort aus Referenz-env uebernehmen?" "y")" == "y" ]]; then
    DB_PASSWORD="${default_db_password}"
  else
    DB_PASSWORD="$(ask_secret_required "DB Passwort")"
  fi
else
  DB_PASSWORD="$(ask_secret_required "DB Passwort")"
fi

CACHE_HOST="$(ask_optional "Cache Host (IP oder DNS)" "${default_cache_host}")"
if [[ -z "${CACHE_HOST}" ]]; then
  CACHE_HOST="$(ask_required "Cache Host (IP oder DNS)")"
fi
CACHE_PORT="$(ask_optional "Cache Port" "${default_cache_port}")"

if [[ -n "${default_cache_password}" ]]; then
  if [[ "$(ask_yes_no "Cache Passwort aus Referenz-env uebernehmen?" "y")" == "y" ]]; then
    CACHE_PASSWORD="${default_cache_password}"
  else
    CACHE_PASSWORD="$(ask_secret_optional "Cache Passwort")"
  fi
else
  CACHE_PASSWORD="$(ask_secret_optional "Cache Passwort")"
fi

if [[ -n "${default_upload_secret}" ]]; then
  if [[ "$(ask_yes_no "UPLOAD_TOKEN_SECRET aus Referenz-env uebernehmen?" "y")" == "y" ]]; then
    UPLOAD_TOKEN_SECRET="${default_upload_secret}"
  else
    UPLOAD_TOKEN_SECRET="$(ask_secret_required "UPLOAD_TOKEN_SECRET")"
  fi
else
  UPLOAD_TOKEN_SECRET="$(ask_secret_required "UPLOAD_TOKEN_SECRET")"
fi

if [[ -n "${default_replication_secret}" ]]; then
  if [[ "$(ask_yes_no "REPLICATION_SECRET aus Referenz-env uebernehmen?" "y")" == "y" ]]; then
    REPLICATION_SECRET="${default_replication_secret}"
  else
    REPLICATION_SECRET="$(ask_secret_required "REPLICATION_SECRET")"
  fi
else
  REPLICATION_SECRET="$(ask_secret_required "REPLICATION_SECRET")"
fi

DISABLE_JOB_WORKERS="1"
if [[ "$(ask_yes_no "Job-Worker auf Storage-Node aktivieren?" "n")" == "y" ]]; then
  DISABLE_JOB_WORKERS="0"
fi

echo
echo "Zusammenfassung:"
echo "  Stack Dir:              ${STACK_DIR}"
echo "  Uploads/Tmp:            ${UPLOADS_DIR} / ${TMP_DIR}"
echo "  Node ID:                ${NODE_ID}"
echo "  Public Domain:          ${PUBLIC_DOMAIN}"
echo "  App Image:              ${APP_IMAGE}"
echo "  DB:                     ${DB_HOST}:${DB_PORT} (${DB_NAME}/${DB_USER})"
echo "  Cache:                  ${CACHE_HOST}:${CACHE_PORT}"
echo "  Cache Passwort:         $(mask_secret "${CACHE_PASSWORD}")"
echo "  UPLOAD_TOKEN_SECRET:    $(mask_secret "${UPLOAD_TOKEN_SECRET}")"
echo "  REPLICATION_SECRET:     $(mask_secret "${REPLICATION_SECRET}")"
echo "  DISABLE_JOB_WORKERS:    ${DISABLE_JOB_WORKERS}"
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
NODE_ID=${NODE_ID}
DISABLE_JOB_WORKERS=${DISABLE_JOB_WORKERS}"

echo "Deploye Storage-Node..."
docker compose -f "${COMPOSE_FILE}" --env-file "${ENV_FILE}" pull || true
docker compose -f "${COMPOSE_FILE}" --env-file "${ENV_FILE}" up -d
docker compose -f "${COMPOSE_FILE}" --env-file "${ENV_FILE}" ps

echo
echo "Fertig."
echo "Storage Stack: ${STACK_DIR}"
echo ".env:          ${ENV_FILE}"
echo
echo "Wichtig im Admin:"
echo "  storage_pool.node_id=${NODE_ID}"
echo "  storage_pool.public_base_url=${PUBLIC_DOMAIN}"
echo "  storage_pool.upload_api_url=${PUBLIC_DOMAIN}/api/internal/upload"
