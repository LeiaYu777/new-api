#!/usr/bin/env bash
set -euo pipefail

# Verify a Seedance 2.0 billing evidence directory collected by:
#   scripts/seedance-billing-collect-evidence.sh
#
# Usage:
#   scripts/seedance-billing-verify-evidence.sh compliance/evidence/seedance-20260506T120000Z
#
# Optional controls:
#   EVIDENCE_DIR=...                  Directory to verify when no positional arg is given.
#   REQUIRE_TASK_RESULT=auto          auto|true|false. auto requires task-result.json when manifest.task_id is set.
#   REQUIRE_TASK_ID_MATCH=auto        auto|true|false. auto requires billing-ledger.csv to contain manifest.task_id when set.

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

EVIDENCE_DIR="${1:-${EVIDENCE_DIR:-}}"
REQUIRE_TASK_RESULT="${REQUIRE_TASK_RESULT:-auto}"
REQUIRE_TASK_ID_MATCH="${REQUIRE_TASK_ID_MATCH:-auto}"
FAILS=0
WARNS=0
CHECKS=0

lower() {
  printf '%s' "${1:-}" | tr '[:upper:]' '[:lower:]'
}

is_true() {
  case "$(lower "${1:-}")" in
    true|1|yes|y|on) return 0 ;;
    *) return 1 ;;
  esac
}

is_false() {
  case "$(lower "${1:-}")" in
    false|0|no|n|off) return 0 ;;
    *) return 1 ;;
  esac
}

pass() {
  CHECKS=$((CHECKS + 1))
  printf '[PASS] %s\n' "$1"
}

warn() {
  CHECKS=$((CHECKS + 1))
  WARNS=$((WARNS + 1))
  printf '[WARN] %s\n' "$1"
}

fail() {
  CHECKS=$((CHECKS + 1))
  FAILS=$((FAILS + 1))
  printf '[FAIL] %s\n' "$1"
}

latest_evidence_dir() {
  local latest=""
  if [[ -d "${ROOT_DIR}/compliance/evidence" ]]; then
    latest="$(find "${ROOT_DIR}/compliance/evidence" -maxdepth 1 -type d -name 'seedance-*' | sort | tail -n 1)"
  fi
  printf '%s' "${latest}"
}

if [[ -z "${EVIDENCE_DIR}" ]]; then
  EVIDENCE_DIR="$(latest_evidence_dir)"
fi
if [[ -z "${EVIDENCE_DIR}" ]]; then
  echo "Evidence directory is required, or create one under compliance/evidence/seedance-* first." >&2
  exit 2
fi
if [[ ! -d "${EVIDENCE_DIR}" ]]; then
  echo "Evidence directory does not exist: ${EVIDENCE_DIR}" >&2
  exit 2
fi
if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required." >&2
  exit 2
fi

printf 'Seedance billing evidence verification\n'
printf 'Evidence: %s\n' "${EVIDENCE_DIR}"

require_file() {
  local file="$1"
  local path="${EVIDENCE_DIR}/${file}"
  if [[ -s "${path}" ]]; then
    pass "${file} exists and is not empty."
    return 0
  fi
  fail "${file} is missing or empty."
  return 1
}

optional_file() {
  local file="$1"
  local path="${EVIDENCE_DIR}/${file}"
  if [[ -s "${path}" ]]; then
    pass "${file} exists."
    return 0
  fi
  warn "${file} is not present."
  return 1
}

check_http_status() {
  local file="$1"
  local status_file="${EVIDENCE_DIR}/${file}.http_status"
  local status
  if [[ ! -s "${status_file}" ]]; then
    fail "${file}.http_status is missing."
    return
  fi
  status="$(head -n 1 "${status_file}" | tr -d '[:space:]')"
  if [[ "${status}" =~ ^2[0-9][0-9]$ ]]; then
    pass "${file} HTTP status is ${status}."
  else
    fail "${file} HTTP status is ${status:-empty}."
  fi
}

check_json_file() {
  local file="$1"
  local path="${EVIDENCE_DIR}/${file}"
  if jq . "${path}" >/dev/null 2>&1; then
    pass "${file} is valid JSON."
  else
    fail "${file} is not valid JSON."
  fi
}

check_api_success() {
  local file="$1"
  local path="${EVIDENCE_DIR}/${file}"
  local success
  success="$(jq -r '.success // empty' "${path}" 2>/dev/null || true)"
  if [[ "${success}" == "true" ]]; then
    pass "${file} reports success=true."
  else
    fail "${file} does not report success=true."
  fi
}

csv_header_contains() {
  local file="$1"
  local field="$2"
  local header
  header="$(head -n 1 "${EVIDENCE_DIR}/${file}" 2>/dev/null || true)"
  case ",${header}," in
    *",${field},"*) return 0 ;;
    *) return 1 ;;
  esac
}

check_csv_headers() {
  local file="$1"
  shift
  local missing=0
  local field
  for field in "$@"; do
    if ! csv_header_contains "${file}" "${field}"; then
      fail "${file} is missing CSV column '${field}'."
      missing=1
    fi
  done
  if (( missing == 0 )); then
    pass "${file} includes required CSV columns."
  fi
}

check_metric_name() {
  local metric="$1"
  if grep -q "^${metric}" "${EVIDENCE_DIR}/billing-metrics.prom"; then
    pass "billing-metrics.prom contains ${metric}."
  else
    fail "billing-metrics.prom is missing ${metric}."
  fi
}

require_file "manifest.json" || true
require_file "README.md" || true
require_file "billing-readiness.json" || true
require_file "billing-summary.json" || true
require_file "billing-alerts.json" || true
require_file "billing-statements.json" || true
require_file "billing-ledger.csv" || true
require_file "billing-metrics.prom" || true
optional_file "billing-topups.csv" || true

for file in billing-readiness.json billing-summary.json billing-alerts.json billing-statements.json billing-ledger.csv billing-metrics.prom; do
  if [[ -e "${EVIDENCE_DIR}/${file}" || -e "${EVIDENCE_DIR}/${file}.http_status" ]]; then
    check_http_status "${file}"
  fi
done

for file in manifest.json billing-readiness.json billing-summary.json billing-alerts.json billing-statements.json; do
  if [[ -s "${EVIDENCE_DIR}/${file}" ]]; then
    check_json_file "${file}"
  fi
done

for file in billing-readiness.json billing-summary.json billing-alerts.json billing-statements.json; do
  if [[ -s "${EVIDENCE_DIR}/${file}" ]]; then
    check_api_success "${file}"
  fi
done

if [[ -s "${EVIDENCE_DIR}/billing-readiness.json" ]]; then
  readiness_status="$(jq -r '.data.status // empty' "${EVIDENCE_DIR}/billing-readiness.json" 2>/dev/null || true)"
  case "${readiness_status}" in
    ready|warning)
      pass "billing-readiness.json status is ${readiness_status}."
      ;;
    blocked)
      fail "billing-readiness.json status is blocked."
      ;;
    *)
      fail "billing-readiness.json status is ${readiness_status:-empty}."
      ;;
  esac
fi

if [[ -s "${EVIDENCE_DIR}/billing-ledger.csv" ]]; then
  check_csv_headers "billing-ledger.csv" \
    created_at user_id username model_name channel_id token_id log_type quota \
    group request_id task_id content billing_source subscription_id pre_consumed_quota actual_quota
fi

if [[ -s "${EVIDENCE_DIR}/billing-metrics.prom" ]]; then
  check_metric_name "newapi_billing_net_quota"
  check_metric_name "newapi_billing_task_failure_rate"
  check_metric_name "newapi_billing_worker_lag_seconds"
fi

manifest_task_id=""
if [[ -s "${EVIDENCE_DIR}/manifest.json" ]]; then
  manifest_task_id="$(jq -r '.task_id // empty' "${EVIDENCE_DIR}/manifest.json" 2>/dev/null || true)"
  if [[ -n "${manifest_task_id}" ]]; then
    pass "manifest.json contains task_id=${manifest_task_id}."
  else
    warn "manifest.json does not contain a task_id; task-level ledger matching is skipped unless REQUIRE_TASK_ID_MATCH=true."
  fi
fi

should_require_task_result=false
if is_true "${REQUIRE_TASK_RESULT}"; then
  should_require_task_result=true
elif [[ "$(lower "${REQUIRE_TASK_RESULT}")" == "auto" && -n "${manifest_task_id}" ]]; then
  should_require_task_result=true
fi

if [[ "${should_require_task_result}" == "true" ]]; then
  require_file "task-result.json" || true
  if [[ -s "${EVIDENCE_DIR}/task-result.json" ]]; then
    check_http_status "task-result.json"
    check_json_file "task-result.json"
  fi
else
  optional_file "task-result.json" || true
fi

should_require_task_match=false
if is_true "${REQUIRE_TASK_ID_MATCH}"; then
  should_require_task_match=true
elif [[ "$(lower "${REQUIRE_TASK_ID_MATCH}")" == "auto" && -n "${manifest_task_id}" ]]; then
  should_require_task_match=true
fi

if [[ "${should_require_task_match}" == "true" ]]; then
  if [[ -z "${manifest_task_id}" ]]; then
    fail "REQUIRE_TASK_ID_MATCH=true but manifest task_id is empty."
  elif grep -Fq "${manifest_task_id}" "${EVIDENCE_DIR}/billing-ledger.csv"; then
    pass "billing-ledger.csv contains task_id ${manifest_task_id}."
  else
    fail "billing-ledger.csv does not contain task_id ${manifest_task_id}."
  fi
fi

printf 'Checks: %d, warnings: %d, failures: %d\n' "${CHECKS}" "${WARNS}" "${FAILS}"
if (( FAILS > 0 )); then
  echo "Evidence verification failed." >&2
  exit 1
fi
if (( WARNS > 0 )); then
  echo "Evidence verification completed with warnings."
else
  echo "Evidence verification passed."
fi
