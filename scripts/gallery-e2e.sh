#!/usr/bin/env bash
set -euo pipefail

API="${API_URL:-http://localhost:8080}"
SUFFIX="${GALLERY_E2E_SUFFIX:-$(date +%s)-$$}"
ADMIN_TOKEN="${ADMIN_TOKEN:-}"

pass() { printf 'PASS: %s\n' "$*"; }
fail() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }

require_json() {
  local payload="$1"
  local expression="$2"
  local label="$3"
  printf '%s' "$payload" | jq -e "$expression" >/dev/null || fail "$label"
}

admin_request() {
  local method="$1"
  local path="$2"
  local body="${3:-}"
  local args=(-sSf --max-time 30 -X "$method" "$API/admin/api/$path" -H "Authorization: Bearer $ADMIN_TOKEN")
  if [[ -n "$body" ]]; then
    args+=(-H "Content-Type: application/json" -d "$body")
  fi
  curl "${args[@]}"
}

if [[ -z "$ADMIN_TOKEN" ]]; then
  login="$(curl -sSf --max-time 15 -X POST "$API/admin/api/login" \
    -H "Content-Type: application/json" \
    -d '{"email":"admin@starai.local","password":"admin123"}')" || fail "Admin login request"
  ADMIN_TOKEN="$(printf '%s' "$login" | jq -r '.data.token // empty')"
  [[ -n "$ADMIN_TOKEN" ]] || fail "Admin login returned no token"
fi
pass "Gallery E2E admin login"

gap_public_id="g_gap_e2e_${SUFFIX}"
spec_public_id="g_spec_e2e_${SUFFIX}"
title="Gallery E2E ${SUFFIX}"
prompt="Create a clean geometric editorial composition using coral, teal, black, and white blocks. Test run ${SUFFIX}. No text."

import_payload="$(jq -n \
  --arg gap "$gap_public_id" \
  --arg spec "$spec_public_id" \
  --arg title "$title" \
  --arg prompt "$prompt" \
  --arg snapshot "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  '{
    tasks: [{
      public_id: $gap,
      source_report_hash: ("gallery-e2e-" + $gap),
      source_snapshot_at: $snapshot,
      dimension_type: "category_style",
      category: "设计",
      style: "几何编辑",
      scene: "海报",
      prompt_language: "en",
      target_count: 1,
      current_count: 0,
      gap_count: 1,
      priority: "high",
      title: ($title + " gap"),
      rationale: "Isolated pre-production generation and publication regression.",
      risk_flags: [],
      source_context: {test: "gallery-e2e"}
    }],
    specs: [{
      public_id: $spec,
      gap_task_public_id: $gap,
      title: $title,
      purpose: "Verify the complete controlled Gallery generation and publication workflow.",
      prompt: $prompt,
      negative_prompt: "blurry, malformed geometry, unreadable details",
      prompt_language: "en",
      image_text_languages: [],
      expected_text: [],
      category: "设计",
      styles: ["几何编辑"],
      scenes: ["海报"],
      aspect_ratio: "1:1",
      output_media_type: "image",
      candidate_models: [{code: "image_fast_v1", role: "primary", rationale: "Isolated mock image route"}],
      acceptance_criteria: ["image is decodable", "metadata matches the specification", "manual review is required before publication"],
      selection_reason: "Deterministic isolated regression fixture.",
      risk_flags: [],
      risk_notes: "",
      risk_review_status: "not_required",
      status: "ready",
      generator_type: "manual",
      generation_input: {size: "1024x1024", n: 1},
      generation_output: {}
    }]
  }')"

imported="$(admin_request POST gallery/case-specs/import "$import_payload")" || fail "Import Gallery E2E case specification"
spec_id="$(printf '%s' "$imported" | jq -r --arg id "$spec_public_id" '.data.items[] | select(.public_id == $id) | .id' | head -1)"
[[ "$spec_id" =~ ^[0-9]+$ ]] || fail "Imported Gallery E2E specification was not returned"
pass "Gallery gap and ready specification import"

batch_payload="$(jq -n --argjson spec "$spec_id" '{spec_ids: [$spec], max_concurrency: 1, max_retries: 1, budget_limit: 1}')"
batch="$(admin_request POST gallery/generation-batches "$batch_payload")" || fail "Create Gallery generation batch"
batch_id="$(printf '%s' "$batch" | jq -r '.data.id // empty')"
[[ "$batch_id" =~ ^[0-9]+$ ]] || fail "Generation batch returned no id"
require_json "$batch" '.data.status == "draft" and .data.task_count == 1' "Generation batch draft payload"
admin_request POST "gallery/generation-batches/$batch_id/start" >/dev/null || fail "Start Gallery generation batch"
pass "Gallery generation batch start"

task=""
for _ in $(seq 1 60); do
  tasks="$(admin_request GET "gallery/generation-batches/$batch_id/tasks?page_size=10")" || fail "Read Gallery generation tasks"
  task="$(printf '%s' "$tasks" | jq -c '.data.items[0] // empty')"
  status="$(printf '%s' "$task" | jq -r '.status // empty')"
  if [[ "$status" == "pending_review" ]]; then
    break
  fi
  if [[ "$status" == "failed" || "$status" == "cancelled" || "$status" == "expired" || "$status" == "unknown" ]]; then
    printf '%s\n' "$task" >&2
    fail "Gallery generation task entered terminal failure state: $status"
  fi
  sleep 1
done
require_json "$task" '.status == "pending_review" and (.output_asset_url | startswith("http"))' "Gallery task did not reach pending_review"
task_id="$(printf '%s' "$task" | jq -r '.id')"
asset_url="$(printf '%s' "$task" | jq -r '.output_asset_url')"
curl -sSf --max-time 15 "$asset_url" >/dev/null || fail "Persisted Gallery asset is not publicly readable"
pass "Worker generation, persistence, and pending review gate"

review=""
for _ in $(seq 1 20); do
  reviews="$(admin_request GET "gallery/quality-reviews?page_size=100")" || fail "Read Gallery quality reviews"
  review="$(printf '%s' "$reviews" | jq -c --argjson task "$task_id" '.data.items[] | select(.task_id == $task)' | head -1)"
  [[ -n "$review" ]] && break
  sleep 1
done
review_id="$(printf '%s' "$review" | jq -r '.id // empty')"
[[ "$review_id" =~ ^[0-9]+$ ]] || fail "Quality review record was not created"

prepublish_body="$(mktemp)"
prepublish_code="$(curl -sS -o "$prepublish_body" -w '%{http_code}' --max-time 15 -X POST \
  "$API/admin/api/gallery/quality-reviews/$review_id/publish" \
  -H "Authorization: Bearer $ADMIN_TOKEN")"
rm -f "$prepublish_body"
[[ "$prepublish_code" == "400" ]] || fail "Unapproved publish expected HTTP 400, got $prepublish_code"
pass "Unreviewed publication guard"

checked="$(admin_request POST "gallery/quality-reviews/$review_id/auto-check")" || fail "Run Gallery automatic quality check"
require_json "$checked" '.data.status == "awaiting_manual_review" and .data.check_summary.overall == "pass" and .data.asset_check.status == "pass"' "Automatic quality check result"
printf '%s' "$checked" | jq -e --arg url "$asset_url" '.data.asset_check.url == $url' >/dev/null || fail "Automatic check leaked an internal asset URL"
pass "Automatic asset, metadata, risk, and duplicate checks"

empty_reject_body="$(mktemp)"
empty_reject_code="$(curl -sS -o "$empty_reject_body" -w '%{http_code}' --max-time 15 -X POST \
  "$API/admin/api/gallery/quality-reviews/$review_id/reject" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{}')"
rm -f "$empty_reject_body"
[[ "$empty_reject_code" == "400" ]] || fail "Empty rejection reason expected HTTP 400, got $empty_reject_code"
pass "Empty rejection reason guard"

approved="$(admin_request POST "gallery/quality-reviews/$review_id/approve" '{"review_notes":"Isolated pre-production visual review passed."}')" || fail "Approve Gallery quality review"
require_json "$approved" '.data.status == "approved"' "Gallery manual approval result"
pass "Manual Gallery approval"

published_once="$(admin_request POST "gallery/quality-reviews/$review_id/publish")" || fail "Publish approved Gallery item"
require_json "$published_once" '.data.status == "published" and (.data.gallery_item_id | type == "number")' "First Gallery publication result"
gallery_item_id="$(printf '%s' "$published_once" | jq -r '.data.gallery_item_id')"
published_at="$(printf '%s' "$published_once" | jq -r '.data.published_at')"

published_twice="$(admin_request POST "gallery/quality-reviews/$review_id/publish")" || fail "Repeat Gallery publication"
printf '%s' "$published_twice" | jq -e --argjson id "$gallery_item_id" --arg at "$published_at" '.data.status == "published" and .data.gallery_item_id == $id and .data.published_at == $at' >/dev/null || fail "Gallery publication was not idempotent"
pass "Explicit publication and idempotent retry"

public_gallery="$(curl -sSf --max-time 15 "$API/api/gallery?page=1&page_size=100")" || fail "Read public Gallery after publication"
printf '%s' "$public_gallery" | jq -e --arg title "$title" --arg asset "$asset_url" '[.data.items[] | select(.title == $title and .cover_url == $asset and .status == "approved")] | length == 1' >/dev/null || fail "Published Gallery item is missing or duplicated"
pass "Published item is queryable in the public Gallery"

printf '=== Gallery generation E2E passed: %s ===\n' "$title"
