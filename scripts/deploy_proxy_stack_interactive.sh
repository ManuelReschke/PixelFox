#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# shellcheck source=./deploy_common.sh
source "${SCRIPT_DIR}/deploy_common.sh"

PROXY_TEMPLATE="${REPO_ROOT}/docker/prod/proxy.compose.yml"
if [[ ! -f "${PROXY_TEMPLATE}" ]]; then
  echo "Fehler: ${PROXY_TEMPLATE} nicht gefunden." >&2
  exit 1
fi

ensure_docker_compose_ready

echo "Interaktiver Deploy: Caddy Reverse Proxy"
echo

STACK_DIR="$(ask_default "Proxy Stack Verzeichnis" "/srv/caddy")"
CADDYFILE_PATH="$(ask_default "Caddyfile Pfad" "/srv/caddy/Caddyfile")"
CADDY_DATA_DIR="$(ask_default "Caddy Data Verzeichnis" "/srv/caddy/data")"
CADDY_CONFIG_DIR="$(ask_default "Caddy Config Verzeichnis" "/srv/caddy/config")"

DOMAIN="$(ask_required "Domain (z.B. pixelfox.cc)")"
BACKEND="$(ask_default "Backend Ziel (host:port)" "127.0.0.1:4000")"
CADDY_EMAIL="$(ask_optional "ACME Email (optional)")"

REDIRECT_WWW="n"
WWW_DOMAIN=""
if [[ "$(ask_yes_no "www auf Root-Domain umleiten?" "y")" == "y" ]]; then
  REDIRECT_WWW="y"
  WWW_DOMAIN="www.${DOMAIN}"
fi

echo
echo "Zusammenfassung:"
echo "  Stack Dir:        ${STACK_DIR}"
echo "  Caddyfile:        ${CADDYFILE_PATH}"
echo "  Caddy Data:       ${CADDY_DATA_DIR}"
echo "  Caddy Config:     ${CADDY_CONFIG_DIR}"
echo "  Domain:           ${DOMAIN}"
echo "  Backend:          ${BACKEND}"
echo "  ACME Email:       ${CADDY_EMAIL:-<leer>}"
if [[ "${REDIRECT_WWW}" == "y" ]]; then
  echo "  www Redirect:     ${WWW_DOMAIN} -> ${DOMAIN}"
else
  echo "  www Redirect:     nein"
fi
echo

if [[ "$(ask_yes_no "Deployment jetzt ausfuehren?" "y")" != "y" ]]; then
  echo "Abgebrochen."
  exit 0
fi

echo
echo "Erstelle Verzeichnisse..."
CADDYFILE_DIR="$(dirname "${CADDYFILE_PATH}")"
run_privileged mkdir -p "${STACK_DIR}" "${CADDYFILE_DIR}" "${CADDY_DATA_DIR}" "${CADDY_CONFIG_DIR}"
if [[ "${EUID}" -ne 0 ]]; then
  run_privileged chown -R "$(id -un):$(id -gn)" "${STACK_DIR}" || true
fi

COMPOSE_FILE="${STACK_DIR}/docker-compose.yml"
ENV_FILE="${STACK_DIR}/.env"

echo "Schreibe Compose und .env..."
cp "${PROXY_TEMPLATE}" "${COMPOSE_FILE}"
write_file_secure "${ENV_FILE}" "CADDY_EMAIL=${CADDY_EMAIL}
CADDYFILE_PATH=${CADDYFILE_PATH}
CADDY_DATA_DIR=${CADDY_DATA_DIR}
CADDY_CONFIG_DIR=${CADDY_CONFIG_DIR}"

echo "Schreibe Caddyfile..."
: >"${CADDYFILE_PATH}"
if [[ -n "${CADDY_EMAIL}" ]]; then
  cat >>"${CADDYFILE_PATH}" <<EOF
{
  email ${CADDY_EMAIL}
}

EOF
fi

cat >>"${CADDYFILE_PATH}" <<EOF
${DOMAIN} {
  encode zstd gzip
  header {
    Strict-Transport-Security "max-age=31536000; includeSubDomains; preload"
    X-Frame-Options "DENY"
    X-Content-Type-Options "nosniff"
    Referrer-Policy "strict-origin-when-cross-origin"
  }
  reverse_proxy ${BACKEND}
}
EOF

if [[ "${REDIRECT_WWW}" == "y" ]]; then
  cat >>"${CADDYFILE_PATH}" <<EOF
${WWW_DOMAIN} {
  redir https://${DOMAIN}{uri} permanent
}
EOF
fi

echo "Deploye Proxy..."
docker compose -f "${COMPOSE_FILE}" --env-file "${ENV_FILE}" pull || true
docker compose -f "${COMPOSE_FILE}" --env-file "${ENV_FILE}" up -d
docker compose -f "${COMPOSE_FILE}" --env-file "${ENV_FILE}" ps

echo
echo "Fertig."
echo "Proxy Stack: ${STACK_DIR}"
echo "Caddyfile:   ${CADDYFILE_PATH}"
echo
echo "Pruefen:"
echo "  curl -I https://${DOMAIN}"
