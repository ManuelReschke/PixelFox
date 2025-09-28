#!/usr/bin/env bash
set -euo pipefail

# Simple deploy helper for the App container using Docker Compose.
# - Pulls the image (if available) and recreates the app service.
# - Does not touch DB/Cache or Proxy.
#
# Usage examples:
#   bash scripts/deploy_app.sh \
#     -f /srv/pixelfox/docker-compose.yml \
#     -e /srv/pixelfox/.env \
#     -i registry.example.com/pixelfox/app:20250101-1200
#
#   # If compose file and .env are in repo paths:
#   bash scripts/deploy_app.sh -f docker/prod/app.compose.yml -e docker/prod/.env.app.example -i registry.example.com/pixelfox/app:latest

COMPOSE_FILE=""
ENV_FILE=""
APP_IMAGE=""

while getopts ":f:e:i:" opt; do
  case $opt in
    f) COMPOSE_FILE="$OPTARG" ;;
    e) ENV_FILE="$OPTARG" ;;
    i) APP_IMAGE="$OPTARG" ;;
    *) echo "Usage: $0 -f <compose_file> -e <env_file> [-i <image>]" >&2; exit 1 ;;
  esac
done

if [[ -z "${COMPOSE_FILE}" || -z "${ENV_FILE}" ]]; then
  echo "Usage: $0 -f <compose_file> -e <env_file> [-i <image>]" >&2
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

ENV_ARGS=("--env-file" "${ENV_FILE}")

if [[ -n "${APP_IMAGE}" ]]; then
  echo "Setting APP_IMAGE=${APP_IMAGE} for this deploy"
  export APP_IMAGE
fi

echo "Pulling images (if registry is reachable)..."
docker compose -f "${COMPOSE_FILE}" "${ENV_ARGS[@]}" pull || true

echo "Recreating app service..."
docker compose -f "${COMPOSE_FILE}" "${ENV_ARGS[@]}" up -d

echo "Done. Current services:"
docker compose -f "${COMPOSE_FILE}" "${ENV_ARGS[@]}" ps

