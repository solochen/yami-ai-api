#!/usr/bin/env bash
set -euo pipefail

API="${API_URL:-http://localhost:8080}"
PREPARE_TEST_AUTH="${PREPARE_TEST_AUTH:-0}"
ADMIN_TOKEN=""
ORIGINAL_CAPTCHA=""
CAPTCHA_CHANGED=0

pass() { printf 'PASS: %s\n' "$*"; }
fail() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }

require_json() {
  local payload="$1"
  local expression="$2"
  local label="$3"
  printf '%s' "$payload" | jq -e "$expression" >/dev/null || fail "$label"
}

restore_config() {
  if [[ "$CAPTCHA_CHANGED" == "1" && -n "$ADMIN_TOKEN" && -n "$ORIGINAL_CAPTCHA" ]]; then
    curl -sS -o /dev/null -X PATCH "$API/admin/api/system-configs" \
      -H "Authorization: Bearer $ADMIN_TOKEN" \
      -H "Content-Type: application/json" \
      -d "{\"image_captcha_enabled\":$ORIGINAL_CAPTCHA}" || true
  fi
}
trap restore_config EXIT

printf '=== StarAI integration smoke: %s ===\n' "$API"

health="$(curl -sSf --max-time 15 "$API/health")" || fail "API health request"
require_json "$health" '.data.status == "ok"' "API health payload"
pass "API health"

admin_login="$(curl -sSf --max-time 15 -X POST "$API/admin/api/login" \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@starai.local","password":"admin123"}')" || fail "Admin login request"
ADMIN_TOKEN="$(printf '%s' "$admin_login" | jq -r '.data.token // empty')"
[[ -n "$ADMIN_TOKEN" ]] || fail "Admin login returned no token"
pass "Admin login"

if [[ "$PREPARE_TEST_AUTH" == "1" || "$PREPARE_TEST_AUTH" == "true" ]]; then
  config="$(curl -sSf --max-time 15 "$API/admin/api/system-configs" -H "Authorization: Bearer $ADMIN_TOKEN")" || fail "Read system configs"
  ORIGINAL_CAPTCHA="$(printf '%s' "$config" | jq -c '.data.image_captcha_enabled // true')"
  curl -sSf --max-time 15 -X PATCH "$API/admin/api/system-configs" \
    -H "Authorization: Bearer $ADMIN_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"image_captcha_enabled":false}' >/dev/null || fail "Disable captcha for isolated smoke"
  CAPTCHA_CHANGED=1
  pass "Temporary isolated login configuration"
fi

user_login="$(curl -sSf --max-time 15 -X POST "$API/api/auth/login/password" \
  -H "Content-Type: application/json" \
  -d '{"email":"demo@starai.local","password":"demo123"}')" || fail "Demo user login; use PREPARE_TEST_AUTH=1 in an isolated environment"
TOKEN="$(printf '%s' "$user_login" | jq -r '.data.token // empty')"
[[ -n "$TOKEN" ]] || fail "Demo user login returned no token"
AUTH=(-H "Authorization: Bearer $TOKEN")
ADMIN_AUTH=(-H "Authorization: Bearer $ADMIN_TOKEN")
pass "Demo user login"

models="$(curl -sSf --max-time 15 "$API/api/models" "${AUTH[@]}")" || fail "Models request"
require_json "$models" '.data | type == "array" and length > 0' "Models payload"
pass "Models list"

payment_status="$(curl -sS -o /dev/null -w '%{http_code}' --max-time 15 -X POST "$API/api/payment/orders" \
  "${AUTH[@]}" -H "Content-Type: application/json" -d '{}')"
[[ "$payment_status" == "403" ]] || fail "Payment-disabled guard expected HTTP 403, got $payment_status"
pass "Payment-disabled guard"

dashboard="$(curl -sSf --max-time 15 "$API/admin/api/dashboard" "${ADMIN_AUTH[@]}")" || fail "Admin dashboard request"
require_json "$dashboard" '.data | type == "object"' "Admin dashboard payload"
pass "Admin dashboard"

tags="$(curl -sSf --max-time 15 "$API/api/gallery/tags")" || fail "Gallery tags request"
require_json "$tags" '.data.items | type == "array"' "Gallery tags payload"
gallery="$(curl -sSf --max-time 15 "$API/api/gallery")" || fail "Gallery list request"
require_json "$gallery" '.data.items | type == "array"' "Gallery list payload"
references="$(curl -sSf --max-time 15 "$API/api/gallery/references")" || fail "Gallery references request"
require_json "$references" '.data.cases | type == "array"' "Gallery references payload"
pass "Gallery public contracts"

for path in \
  gallery/references \
  gallery/case-specs \
  gallery/generation-batches \
  gallery/quality-reviews; do
  payload="$(curl -sSf --max-time 15 "$API/admin/api/$path" "${ADMIN_AUTH[@]}")" || fail "Admin $path request"
  require_json "$payload" '.data.items | type == "array"' "Admin $path payload"
done
pass "Gallery Admin contracts and authorization"

chat="$(curl -sSf --max-time 30 -X POST "$API/api/chat/completions" \
  "${AUTH[@]}" -H "Content-Type: application/json" \
  -d '{"model_code":"chat_demo_v1","messages":[{"role":"user","content":"hello"}],"stream":false}')" || fail "Mock chat request"
require_json "$chat" '.content | type == "string" and length > 0' "Mock chat payload"
pass "Mock chat completion"

printf '=== Integration smoke passed ===\n'
