#!/usr/bin/env bash
set -euo pipefail

# Simple deploy helper for the Caddy reverse proxy using Docker Compose.
# Assumes host networking and a Caddyfile on the host.
#
# Usage example:
#   bash scripts/deploy_proxy.sh \
#     -f /srv/caddy/docker-compose.yml \
#     -E CADDY_EMAIL=admin@your-domain

COMPOSE_FILE=""
ENV_INJECT=()

while getopts ":f:E:" opt; do
  case $opt in
    f) COMPOSE_FILE="$OPTARG" ;;
    E) ENV_INJECT+=("$OPTARG") ;;
    *) echo "Usage: $0 -f <compose_file> [-E KEY=VALUE]" >&2; exit 1 ;;
  esac
done

if [[ -z "${COMPOSE_FILE}" ]]; then
  echo "Usage: $0 -f <compose_file> [-E KEY=VALUE]" >&2
  exit 1
fi

if ! command -v docker >/dev/null 2>&1; then
  echo "docker not found in PATH" >&2
  exit 1
fi

if ! docker compose version >/dev/null 2>&1; then
  echo "docker compose plugin not available" >&2
  exit 1
fi

# Export injected env for this process
for kv in "${ENV_INJECT[@]:-}"; do
  export "$kv"
done

echo "Pulling caddy image (if registry is reachable)..."
docker compose -f "${COMPOSE_FILE}" pull || true

echo "Recreating proxy service..."
docker compose -f "${COMPOSE_FILE}" up -d

echo "Done. Current services:"
docker compose -f "${COMPOSE_FILE}" ps

