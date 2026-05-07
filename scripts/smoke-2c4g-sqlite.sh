#!/usr/bin/env bash
set -euo pipefail

COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.sqlite.yml}"
ENV_FILE="${ENV_FILE:-.env.sqlite}"
SERVICE_NAME="${SERVICE_NAME:-new-api}"
HEALTH_URL="${HEALTH_URL:-http://localhost:3000/api/status}"
MAX_WAIT_SECONDS="${MAX_WAIT_SECONDS:-120}"

log() {
  printf '[smoke-2c4g-sqlite] %s\n' "$*"
}

fail() {
  printf '[smoke-2c4g-sqlite] ERROR: %s\n' "$*" >&2
  exit 1
}

command -v docker >/dev/null 2>&1 || fail "docker is not installed or not in PATH"
docker compose version >/dev/null 2>&1 || fail "docker compose plugin is not available"
command -v curl >/dev/null 2>&1 || fail "curl is required for the smoke health check"

[ -f "$COMPOSE_FILE" ] || fail "$COMPOSE_FILE not found"
if [ ! -f "$ENV_FILE" ]; then
  if [ -f ".env.sqlite.example" ]; then
    log "$ENV_FILE not found; using .env.sqlite.example for smoke validation"
    ENV_FILE=".env.sqlite.example"
  else
    fail "$ENV_FILE not found and .env.sqlite.example is unavailable"
  fi
fi

mkdir -p data logs

log "starting service with $COMPOSE_FILE and $ENV_FILE"
docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" up -d

log "waiting for $HEALTH_URL"
start_ts=$(date +%s)
while true; do
  if curl -fsS "$HEALTH_URL" | grep '"success"[[:space:]]*:[[:space:]]*true' >/dev/null; then
    log "health check passed"
    break
  fi

  now_ts=$(date +%s)
  if [ $((now_ts - start_ts)) -ge "$MAX_WAIT_SECONDS" ]; then
    docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" logs --tail=120 "$SERVICE_NAME" || true
    fail "service did not become healthy within ${MAX_WAIT_SECONDS}s"
  fi
  sleep 3
done

log "container status"
docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" ps

log "one-shot docker stats"
docker stats --no-stream "$SERVICE_NAME"

log "smoke validation completed without reading API keys, customer data, or calling real models"
