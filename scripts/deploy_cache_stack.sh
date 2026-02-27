#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# shellcheck source=./deploy_common.sh
source "${SCRIPT_DIR}/deploy_common.sh"

CACHE_TEMPLATE="${REPO_ROOT}/docker/prod/cache.compose.yml"
if [[ ! -f "${CACHE_TEMPLATE}" ]]; then
  echo "Fehler: ${CACHE_TEMPLATE} nicht gefunden." >&2
  exit 1
fi

ensure_docker_compose_ready

echo "Interaktiver Deploy: Dragonfly (Cache-Server)"
echo

STACK_DIR="$(ask_default "Cache Stack Verzeichnis" "/srv/dragonfly")"
DATA_DIR="$(ask_default "Cache Daten-Verzeichnis" "/srv/dragonfly/data")"

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

APPLY_UFW="n"
UFW_ALLOWED_IPS_CSV=""
UFW_ENABLE="n"
if [[ "$(ask_yes_no "UFW-Regeln fuer Cache (6379) setzen?" "n")" == "y" ]]; then
  APPLY_UFW="y"
  UFW_ALLOWED_IPS_CSV="$(ask_required "Erlaubte Source-IP(s) (CSV, z.B. 1.2.3.4,5.6.7.8)")"
  UFW_ENABLE="$(ask_yes_no "UFW aktivieren, falls noch deaktiviert?" "n")"
fi

echo
echo "Zusammenfassung:"
echo "  Stack Dir:          ${STACK_DIR}"
echo "  Data Dir:           ${DATA_DIR}"
if [[ "${CACHE_AUTH_ENABLED}" == "y" ]]; then
  echo "  Cache Auth:         aktiv"
  echo "  Cache Passwort:     $(mask_secret "${CACHE_PASSWORD}")"
else
  echo "  Cache Auth:         deaktiviert"
fi
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
if [[ "${CACHE_AUTH_ENABLED}" == "y" ]]; then
  cat >"${COMPOSE_FILE}" <<'EOF'
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
  cp "${CACHE_TEMPLATE}" "${COMPOSE_FILE}"
fi

write_file_secure "${ENV_FILE}" "CACHE_DATA_DIR=${DATA_DIR}
CACHE_PASSWORD=${CACHE_PASSWORD}"

echo "Deploye Cache..."
docker compose -f "${COMPOSE_FILE}" --env-file "${ENV_FILE}" pull || true
docker compose -f "${COMPOSE_FILE}" --env-file "${ENV_FILE}" up -d
docker compose -f "${COMPOSE_FILE}" --env-file "${ENV_FILE}" ps

echo "Cache Healthcheck..."
if [[ "${CACHE_AUTH_ENABLED}" == "y" ]]; then
  if docker compose -f "${COMPOSE_FILE}" --env-file "${ENV_FILE}" exec -T cache \
    redis-cli -a "${CACHE_PASSWORD}" ping 2>/dev/null | grep -q "^PONG$"; then
    echo "  Cache erreichbar."
  else
    echo "  Warnung: Cache noch nicht erreichbar (Start kann noch laufen)."
  fi
else
  if docker compose -f "${COMPOSE_FILE}" --env-file "${ENV_FILE}" exec -T cache \
    redis-cli ping 2>/dev/null | grep -q "^PONG$"; then
    echo "  Cache erreichbar."
  else
    echo "  Warnung: Cache noch nicht erreichbar (Start kann noch laufen)."
  fi
fi

if [[ "${APPLY_UFW}" == "y" ]]; then
  if ! command -v ufw >/dev/null 2>&1; then
    echo "Warnung: ufw nicht gefunden, Regeln wurden uebersprungen."
  else
    echo "Setze UFW-Regeln..."
    run_privileged ufw allow OpenSSH
    run_privileged ufw deny 6379/tcp || true
    IFS=',' read -r -a allowed_ips <<<"${UFW_ALLOWED_IPS_CSV}"
    for raw_ip in "${allowed_ips[@]}"; do
      ip="$(trim "${raw_ip}")"
      if [[ -n "${ip}" ]]; then
        run_privileged ufw allow from "${ip}" to any port 6379 proto tcp
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
echo "Cache Stack: ${STACK_DIR}"
echo "Cache .env:  ${ENV_FILE}"
echo
echo "Fuer App/Storage:"
echo "  CACHE_HOST=<IP dieses Cache-VPS>"
echo "  CACHE_PORT=6379"
if [[ "${CACHE_AUTH_ENABLED}" == "y" ]]; then
  echo "  CACHE_PASSWORD=${CACHE_PASSWORD}"
else
  echo "  CACHE_PASSWORD=<leer>"
fi
