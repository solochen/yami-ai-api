#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

COMPOSE_FILE="${COMPOSE_FILE:-infra/docker/docker-compose.prod.yml}"
ENV_FILE="${ENV_FILE:-.env.preprod}"
TEMPLATE="${TEMPLATE:-.env.preprod.example}"
PROJECT="${COMPOSE_PROJECT_NAME:-starai-preprod}"
KEEP_RUNNING=0
SKIP_BUILD=0
PRESERVE_DATA=0

usage() {
  cat <<'EOF'
Usage: bash scripts/preprod-verify.sh [options]

Options:
  --keep-running   Keep the isolated stack running after verification.
  --skip-build     Reuse existing isolated images.
  --preserve-data  Keep the isolated PostgreSQL/Redis volumes before starting.
  -h, --help       Show this help.

This script only uses the isolated Compose project starai-preprod. It never
targets the default development Compose project unless COMPOSE_PROJECT_NAME is
explicitly overridden.
EOF
}

while (($# > 0)); do
  case "$1" in
    --) shift; continue ;;
    --keep-running) KEEP_RUNNING=1 ;;
    --skip-build) SKIP_BUILD=1 ;;
    --preserve-data) PRESERVE_DATA=1 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown option: $1" >&2; usage >&2; exit 2 ;;
  esac
  shift
done

if [[ ! -f "$ENV_FILE" ]]; then
  [[ -f "$TEMPLATE" ]] || { echo "Missing $TEMPLATE" >&2; exit 1; }
  cp "$TEMPLATE" "$ENV_FILE"
  echo "[preprod] created $ENV_FILE from $TEMPLATE"
fi

COMPOSE=(docker compose --project-name "$PROJECT" --env-file "$ENV_FILE" -f "$COMPOSE_FILE")
step() { printf '[preprod] %s\n' "$*"; }
compose() { "${COMPOSE[@]}" "$@"; }

wait_healthy() {
  local service="$1"
  local id status
  for _ in $(seq 1 60); do
    id="$(compose ps -q "$service" 2>/dev/null | tr -d '\r' || true)"
    if [[ -n "$id" ]]; then
      status="$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "$id" 2>/dev/null || true)"
      if [[ "$status" == "healthy" || "$status" == "running" ]]; then
        return 0
      fi
      if [[ "$status" == "unhealthy" || "$status" == "exited" || "$status" == "dead" ]]; then
        break
      fi
    fi
    sleep 2
  done
  compose logs --tail=100 "$service" || true
  echo "[preprod] $service did not become healthy" >&2
  return 1
}

http_expect() {
  local url="$1"
  local expected="$2"
  local label="${3:-$url}"
  local code body
  body="$(mktemp)"
  for _ in 1 2 3; do
    code="$(curl -sS -o "$body" -w '%{http_code}' --max-time 15 "$url" || true)"
    if [[ "$code" == "$expected" ]]; then
      rm -f "$body"
      step "$label: HTTP $code"
      return 0
    fi
    sleep 2
  done
  echo "[preprod] $label expected HTTP $expected, got $code" >&2
  sed -n '1,80p' "$body" >&2 || true
  rm -f "$body"
  return 1
}

cleanup() {
  if [[ "$KEEP_RUNNING" == "1" ]]; then
    step "keeping isolated stack: web=http://localhost:13000 admin=http://localhost:13001 api=http://localhost:18080"
  else
    step "stopping and removing isolated stack"
    compose --profile preprod --profile tools down -v --remove-orphans || true
    rm -rf data/preprod-uploads
  fi
}
trap cleanup EXIT

step "checking Docker daemon"
docker info >/dev/null
step "validating Compose configuration"
compose config --quiet

if [[ "$PRESERVE_DATA" != "1" ]]; then
  step "resetting isolated PostgreSQL, Redis, and uploads"
  compose --profile preprod --profile tools down -v --remove-orphans
  rm -rf data/preprod-uploads
fi

step "starting isolated dependencies"
compose --profile preprod up -d postgres redis mock-new-api
wait_healthy postgres
wait_healthy redis
wait_healthy mock-new-api

if [[ "$SKIP_BUILD" != "1" ]]; then
  for service in api worker web admin mock-new-api; do
    step "building $service"
    compose build "$service"
  done
fi

step "applying migrations twice for idempotence"
compose --profile tools run --rm migrate /migrate up
compose --profile tools run --rm migrate /migrate up

step "rolling back and restoring isolated migrations"
compose --profile tools run --rm migrate /migrate down
compose --profile tools run --rm migrate /migrate up

step "starting isolated application services"
compose --profile preprod up -d --force-recreate api worker web admin mock-new-api
for service in api worker web admin mock-new-api; do
  wait_healthy "$service"
done

API_URL="${PREPROD_API_URL:-http://localhost:18080}"
WEB_URL="${PREPROD_WEB_URL:-http://localhost:13000}"
ADMIN_URL="${PREPROD_ADMIN_URL:-http://localhost:13001}"

step "checking API health, metrics, and Gallery public contracts"
health="$(curl -sSf --max-time 15 "$API_URL/health")"
printf '%s' "$health" | jq -e '.data.status == "ok"' >/dev/null
http_expect "$API_URL/metrics" 200 "API metrics"
for endpoint in gallery/tags gallery; do
  payload="$(curl -sSf --max-time 15 "$API_URL/api/$endpoint")"
  printf '%s' "$payload" | jq -e '.data.items | type == "array"' >/dev/null
  step "API /api/$endpoint: valid items payload"
done
references="$(curl -sSf --max-time 15 "$API_URL/api/gallery/references")"
printf '%s' "$references" | jq -e '.data.cases | type == "array"' >/dev/null
step "API /api/gallery/references: valid cases payload"

step "checking unauthenticated Admin Gallery protection"
unauth_code="$(curl -sS -o /dev/null -w '%{http_code}' --max-time 15 "$API_URL/admin/api/gallery/quality-reviews")"
[[ "$unauth_code" == "401" || "$unauth_code" == "403" ]]
step "Admin Gallery API without token: HTTP $unauth_code"

step "checking Web and Admin application routes"
http_expect "$WEB_URL/app/gallery" 200 "Web Gallery"
http_expect "$ADMIN_URL/admin/login" 200 "Admin login"
http_expect "$ADMIN_URL/admin/gallery" 200 "Admin Gallery"

step "running the portable integration smoke suite"
API_URL="$API_URL" PREPARE_TEST_AUTH=1 bash scripts/integration-test.sh

step "running the isolated Gallery generation and publication E2E"
API_URL="$API_URL" bash scripts/gallery-e2e.sh

step "isolated pre-production verification passed"
