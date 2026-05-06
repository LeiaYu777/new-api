#!/usr/bin/env bash
set -euo pipefail

# Collect Seedance 2.0 billing acceptance evidence into a timestamped directory.
# It does not submit a task by itself. Run scripts/seedance-billing-smoke.sh first,
# then pass the returned TASK_ID here to gather task result, billing summary, CSV,
# alerts, statements, and Prometheus metrics.
#
# Required:
#   BASE_URL=https://your-domain
#   ADMIN_ACCESS_TOKEN=sk-admin-access-token
#   ADMIN_USER_ID=1
#
# Optional:
#   API_KEY=sk-user-token                 Fetch /v1/video/generations/{TASK_ID}.
#   TASK_ID=task_xxx                      Filter billing evidence to one task.
#   MODEL_FILTER=doubao-seedance-2-0%     Billing model filter.
#   READINESS_MODEL=doubao-seedance-2-0   Model used by /api/billing/readiness.
#   START_TIMESTAMP=...                   Defaults to now - 24h.
#   END_TIMESTAMP=...                     Defaults to now + 1h.
#   USER_ID=...                           Optional user filter.
#   USERNAME=...                          Optional username filter.
#   SELF_ACCESS_TOKEN=...                 Optional normal user access token for /api/billing/self/* evidence.
#   SELF_USER_ID=...                      Optional normal user id for New-Api-User; defaults to USER_ID when set.
#   COLLECT_SELF_BILLING=auto             auto|true|false. auto collects when SELF_ACCESS_TOKEN and SELF_USER_ID exist.
#   BILLING_SOURCE=wallet|subscription    Optional source filter.
#   CHANNEL=...                           Optional channel id filter.
#   GROUP=...                             Optional group filter.
#   LIMIT=1000                            Billing export/query limit.
#   COLLECT_TOPUP=true                    Also export recharge/topup rows.
#   OUTPUT_DIR=compliance/evidence/...    Evidence output directory.

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

BASE_URL="${BASE_URL:-http://localhost:3000}"
MODEL_FILTER="${MODEL_FILTER:-doubao-seedance-2-0%}"
READINESS_MODEL="${READINESS_MODEL:-$(printf '%s' "${MODEL_FILTER}" | sed 's/%$//')}"
TASK_ID="${TASK_ID:-}"
USER_ID="${USER_ID:-}"
USERNAME="${USERNAME:-}"
SELF_ACCESS_TOKEN="${SELF_ACCESS_TOKEN:-${USER_ACCESS_TOKEN:-}}"
SELF_USER_ID="${SELF_USER_ID:-${USER_ID:-}}"
COLLECT_SELF_BILLING="${COLLECT_SELF_BILLING:-auto}"
BILLING_SOURCE="${BILLING_SOURCE:-}"
CHANNEL="${CHANNEL:-}"
GROUP="${GROUP:-}"
LIMIT="${LIMIT:-1000}"
COLLECT_TOPUP="${COLLECT_TOPUP:-true}"
HTTP_TIMEOUT_SECONDS="${HTTP_TIMEOUT_SECONDS:-30}"
now_ts="$(date +%s)"
START_TIMESTAMP="${START_TIMESTAMP:-$((now_ts - 86400))}"
END_TIMESTAMP="${END_TIMESTAMP:-$((now_ts + 3600))}"
RUN_ID="${RUN_ID:-$(date -u +%Y%m%dT%H%M%SZ)}"
OUTPUT_DIR="${OUTPUT_DIR:-${ROOT_DIR}/compliance/evidence/seedance-${RUN_ID}}"

if [[ -z "${ADMIN_ACCESS_TOKEN:-}" || -z "${ADMIN_USER_ID:-}" ]]; then
  echo "ADMIN_ACCESS_TOKEN and ADMIN_USER_ID are required." >&2
  exit 2
fi
if ! command -v curl >/dev/null 2>&1; then
  echo "curl is required." >&2
  exit 2
fi
if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required." >&2
  exit 2
fi

mkdir -p "${OUTPUT_DIR}"
FAILS=0

admin_authorization_header() {
  case "${ADMIN_ACCESS_TOKEN}" in
    Bearer\ *) printf '%s' "${ADMIN_ACCESS_TOKEN}" ;;
    *) printf 'Bearer %s' "${ADMIN_ACCESS_TOKEN}" ;;
  esac
}

api_authorization_header() {
  case "${API_KEY:-}" in
    Bearer\ *) printf '%s' "${API_KEY}" ;;
    *) printf 'Bearer %s' "${API_KEY:-}" ;;
  esac
}

self_authorization_header() {
  case "${SELF_ACCESS_TOKEN:-}" in
    Bearer\ *) printf '%s' "${SELF_ACCESS_TOKEN}" ;;
    *) printf 'Bearer %s' "${SELF_ACCESS_TOKEN:-}" ;;
  esac
}

is_true() {
  case "$(printf '%s' "${1:-}" | tr '[:upper:]' '[:lower:]')" in
    true|1|yes|y|on) return 0 ;;
    *) return 1 ;;
  esac
}

is_false() {
  case "$(printf '%s' "${1:-}" | tr '[:upper:]' '[:lower:]')" in
    false|0|no|n|off) return 0 ;;
    *) return 1 ;;
  esac
}

should_collect_self_billing() {
  local mode
  mode="$(printf '%s' "${COLLECT_SELF_BILLING}" | tr '[:upper:]' '[:lower:]')"
  if is_true "${mode}"; then
    return 0
  fi
  if is_false "${mode}"; then
    return 1
  fi
  [[ -n "${SELF_ACCESS_TOKEN}" && -n "${SELF_USER_ID}" ]]
}

build_base_query_args() {
  COMMON_QUERY=(
    --data-urlencode "start_timestamp=${START_TIMESTAMP}"
    --data-urlencode "end_timestamp=${END_TIMESTAMP}"
    --data-urlencode "model_name=${MODEL_FILTER}"
    --data-urlencode "limit=${LIMIT}"
  )
  if [[ -n "${TASK_ID}" ]]; then
    COMMON_QUERY+=(--data-urlencode "task_id=${TASK_ID}")
  fi
  if [[ -n "${USER_ID}" ]]; then
    COMMON_QUERY+=(--data-urlencode "user_id=${USER_ID}")
  fi
  if [[ -n "${USERNAME}" ]]; then
    COMMON_QUERY+=(--data-urlencode "username=${USERNAME}")
  fi
  if [[ -n "${BILLING_SOURCE}" ]]; then
    COMMON_QUERY+=(--data-urlencode "billing_source=${BILLING_SOURCE}")
  fi
  if [[ -n "${CHANNEL}" ]]; then
    COMMON_QUERY+=(--data-urlencode "channel=${CHANNEL}")
  fi
  if [[ -n "${GROUP}" ]]; then
    COMMON_QUERY+=(--data-urlencode "group=${GROUP}")
  fi
}

build_self_query_args() {
  SELF_QUERY=(
    --data-urlencode "start_timestamp=${START_TIMESTAMP}"
    --data-urlencode "end_timestamp=${END_TIMESTAMP}"
    --data-urlencode "model_name=${MODEL_FILTER}"
    --data-urlencode "limit=${LIMIT}"
  )
  if [[ -n "${TASK_ID}" ]]; then
    SELF_QUERY+=(--data-urlencode "task_id=${TASK_ID}")
  fi
  if [[ -n "${BILLING_SOURCE}" ]]; then
    SELF_QUERY+=(--data-urlencode "billing_source=${BILLING_SOURCE}")
  fi
  if [[ -n "${CHANNEL}" ]]; then
    SELF_QUERY+=(--data-urlencode "channel=${CHANNEL}")
  fi
  if [[ -n "${GROUP}" ]]; then
    SELF_QUERY+=(--data-urlencode "group=${GROUP}")
  fi
}

build_topup_query_args() {
  TOPUP_QUERY=(
    --data-urlencode "type=1"
    --data-urlencode "start_timestamp=${START_TIMESTAMP}"
    --data-urlencode "end_timestamp=${END_TIMESTAMP}"
    --data-urlencode "limit=${LIMIT}"
  )
  if [[ -n "${USER_ID}" ]]; then
    TOPUP_QUERY+=(--data-urlencode "user_id=${USER_ID}")
  fi
  if [[ -n "${USERNAME}" ]]; then
    TOPUP_QUERY+=(--data-urlencode "username=${USERNAME}")
  fi
}

build_self_topup_query_args() {
  SELF_TOPUP_QUERY=(
    --data-urlencode "type=1"
    --data-urlencode "start_timestamp=${START_TIMESTAMP}"
    --data-urlencode "end_timestamp=${END_TIMESTAMP}"
    --data-urlencode "limit=${LIMIT}"
  )
}

pretty_json_file() {
  local file="$1"
  local tmp="${file}.tmp"
  if jq . "${file}" >"${tmp}" 2>/dev/null; then
    mv "${tmp}" "${file}"
  else
    rm -f "${tmp}"
  fi
}

fetch_admin() {
  local path="$1"
  local output="$2"
  shift 2
  local http_code
  http_code="$(curl -sS -G -m "${HTTP_TIMEOUT_SECONDS}" \
    -H "Authorization: $(admin_authorization_header)" \
    -H "New-Api-User: ${ADMIN_USER_ID}" \
    -o "${output}" \
    -w '%{http_code}' \
    "${BASE_URL%/}${path}" \
    "$@" || true)"
  printf '%s\n' "${http_code}" >"${output}.http_status"
  if [[ "${http_code}" =~ ^2[0-9][0-9]$ ]]; then
    echo "[OK] ${path} -> ${output}"
    return 0
  fi
  echo "[FAIL] ${path} returned HTTP ${http_code:-curl_error}; see ${output}" >&2
  return 1
}

collect_admin_json() {
  local path="$1"
  local output="$2"
  shift 2
  if fetch_admin "${path}" "${output}" "$@"; then
    pretty_json_file "${output}"
  else
    FAILS=$((FAILS + 1))
  fi
}

collect_admin_file() {
  local path="$1"
  local output="$2"
  shift 2
  if ! fetch_admin "${path}" "${output}" "$@"; then
    FAILS=$((FAILS + 1))
  fi
}

fetch_self() {
  local path="$1"
  local output="$2"
  shift 2
  local http_code
  http_code="$(curl -sS -G -m "${HTTP_TIMEOUT_SECONDS}" \
    -H "Authorization: $(self_authorization_header)" \
    -H "New-Api-User: ${SELF_USER_ID}" \
    -o "${output}" \
    -w '%{http_code}' \
    "${BASE_URL%/}${path}" \
    "$@" || true)"
  printf '%s\n' "${http_code}" >"${output}.http_status"
  if [[ "${http_code}" =~ ^2[0-9][0-9]$ ]]; then
    echo "[OK] ${path} -> ${output}"
    return 0
  fi
  echo "[FAIL] ${path} returned HTTP ${http_code:-curl_error}; see ${output}" >&2
  return 1
}

collect_self_json() {
  local path="$1"
  local output="$2"
  shift 2
  if fetch_self "${path}" "${output}" "$@"; then
    pretty_json_file "${output}"
  else
    FAILS=$((FAILS + 1))
  fi
}

collect_self_file() {
  local path="$1"
  local output="$2"
  shift 2
  if ! fetch_self "${path}" "${output}" "$@"; then
    FAILS=$((FAILS + 1))
  fi
}

COMMON_QUERY=()
TOPUP_QUERY=()
SELF_QUERY=()
SELF_TOPUP_QUERY=()
build_base_query_args
build_topup_query_args
build_self_query_args
build_self_topup_query_args

COLLECT_SELF=false
if should_collect_self_billing; then
  COLLECT_SELF=true
fi
if [[ "${COLLECT_SELF}" == "true" && ( -z "${SELF_ACCESS_TOKEN}" || -z "${SELF_USER_ID}" ) ]]; then
  echo "SELF_ACCESS_TOKEN and SELF_USER_ID are required when COLLECT_SELF_BILLING=true." >&2
  exit 2
fi

jq -n \
  --arg generated_at "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  --arg base_url "${BASE_URL}" \
  --arg model_filter "${MODEL_FILTER}" \
  --arg readiness_model "${READINESS_MODEL}" \
  --arg task_id "${TASK_ID}" \
  --arg start_timestamp "${START_TIMESTAMP}" \
  --arg end_timestamp "${END_TIMESTAMP}" \
  --arg user_id "${USER_ID}" \
  --arg username "${USERNAME}" \
  --arg self_user_id "${SELF_USER_ID}" \
  --arg collect_self_billing "${COLLECT_SELF}" \
  --arg billing_source "${BILLING_SOURCE}" \
  --arg channel "${CHANNEL}" \
  --arg group "${GROUP}" \
  --arg limit "${LIMIT}" \
  '{
    generated_at: $generated_at,
    base_url: $base_url,
    model_filter: $model_filter,
    readiness_model: $readiness_model,
    task_id: $task_id,
    start_timestamp: ($start_timestamp | tonumber),
    end_timestamp: ($end_timestamp | tonumber),
    user_id: $user_id,
    username: $username,
    self_user_id: $self_user_id,
    collect_self_billing: ($collect_self_billing == "true"),
    billing_source: $billing_source,
    channel: $channel,
    group: $group,
    limit: ($limit | tonumber)
  }' >"${OUTPUT_DIR}/manifest.json"

cat >"${OUTPUT_DIR}/README.md" <<EOF_README
# Seedance 2.0 Billing Evidence

Generated at: $(date -u +%Y-%m-%dT%H:%M:%SZ)

## Scope

- Base URL: ${BASE_URL}
- Model filter: ${MODEL_FILTER}
- Task ID: ${TASK_ID:-not set}
- Start timestamp: ${START_TIMESTAMP}
- End timestamp: ${END_TIMESTAMP}
- User ID: ${USER_ID:-not set}
- Username: ${USERNAME:-not set}
- Self billing user ID: ${SELF_USER_ID:-not set}
- Collect self billing: ${COLLECT_SELF}
- Billing source: ${BILLING_SOURCE:-not set}

## Files

- manifest.json: evidence scope and timestamps.
- billing-readiness.json: /api/billing/readiness server-side production readiness result.
- billing-summary.json: /api/billing/summary result.
- billing-alerts.json: /api/billing/alerts result.
- billing-statements.json: /api/billing/statements result.
- billing-ledger.csv: /api/billing/export result for consume/refund/all task billing rows.
- billing-metrics.prom: /api/billing/metrics Prometheus text output.
- billing-topups.csv: recharge/topup CSV when COLLECT_TOPUP=true.
- billing-self-summary.json: /api/billing/self/summary result when self billing collection is enabled.
- billing-self-statements.json: /api/billing/self/statements result when self billing collection is enabled.
- billing-self-ledger.csv: /api/billing/self/export result when self billing collection is enabled.
- billing-self-topups.csv: /api/billing/self/export type=topup result when self billing and topup collection are enabled.
- task-result.json: task query result when TASK_ID and API_KEY are provided.

Sensitive tokens are intentionally not written to this directory.
EOF_README

collect_admin_json "/api/billing/readiness" "${OUTPUT_DIR}/billing-readiness.json" --data-urlencode "model_name=${READINESS_MODEL}"
collect_admin_json "/api/billing/summary" "${OUTPUT_DIR}/billing-summary.json" "${COMMON_QUERY[@]}"
collect_admin_json "/api/billing/alerts" "${OUTPUT_DIR}/billing-alerts.json" "${COMMON_QUERY[@]}"
collect_admin_json "/api/billing/statements" "${OUTPUT_DIR}/billing-statements.json" "${COMMON_QUERY[@]}" --data-urlencode "p=1" --data-urlencode "page_size=${LIMIT}"
collect_admin_file "/api/billing/export" "${OUTPUT_DIR}/billing-ledger.csv" "${COMMON_QUERY[@]}"
collect_admin_file "/api/billing/metrics" "${OUTPUT_DIR}/billing-metrics.prom" "${COMMON_QUERY[@]}"

if is_true "${COLLECT_TOPUP}"; then
  collect_admin_file "/api/billing/export" "${OUTPUT_DIR}/billing-topups.csv" "${TOPUP_QUERY[@]}"
fi

if [[ "${COLLECT_SELF}" == "true" ]]; then
  collect_self_json "/api/billing/self/summary" "${OUTPUT_DIR}/billing-self-summary.json" "${SELF_QUERY[@]}"
  collect_self_json "/api/billing/self/statements" "${OUTPUT_DIR}/billing-self-statements.json" "${SELF_QUERY[@]}" --data-urlencode "p=1" --data-urlencode "page_size=${LIMIT}"
  collect_self_file "/api/billing/self/export" "${OUTPUT_DIR}/billing-self-ledger.csv" "${SELF_QUERY[@]}"
  if is_true "${COLLECT_TOPUP}"; then
    collect_self_file "/api/billing/self/export" "${OUTPUT_DIR}/billing-self-topups.csv" "${SELF_TOPUP_QUERY[@]}"
  fi
else
  echo "[INFO] Skipping /api/billing/self/* evidence because SELF_ACCESS_TOKEN or SELF_USER_ID is not set."
fi

if [[ -n "${TASK_ID}" && -n "${API_KEY:-}" ]]; then
  task_output="${OUTPUT_DIR}/task-result.json"
  task_http_code="$(curl -sS -m "${HTTP_TIMEOUT_SECONDS}" \
    -H "Authorization: $(api_authorization_header)" \
    -o "${task_output}" \
    -w '%{http_code}' \
    "${BASE_URL%/}/v1/video/generations/${TASK_ID}" || true)"
  printf '%s\n' "${task_http_code}" >"${task_output}.http_status"
  if [[ "${task_http_code}" =~ ^2[0-9][0-9]$ ]]; then
    pretty_json_file "${task_output}"
    echo "[OK] /v1/video/generations/${TASK_ID} -> ${task_output}"
  else
    echo "[FAIL] task fetch returned HTTP ${task_http_code:-curl_error}; see ${task_output}" >&2
    FAILS=$((FAILS + 1))
  fi
else
  echo "[INFO] Skipping task-result.json because TASK_ID or API_KEY is not set."
fi

if (( FAILS > 0 )); then
  echo "Evidence collection completed with ${FAILS} failed request(s). Output: ${OUTPUT_DIR}" >&2
  exit 1
fi

echo "Evidence collection completed: ${OUTPUT_DIR}"
