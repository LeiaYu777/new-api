#!/usr/bin/env bash
set -euo pipefail

# Preflight checks for Seedance 2.0 billing production readiness.
# It validates local environment/configuration only. It does not submit tasks.
#
# Usage:
#   scripts/seedance-billing-preflight.sh
#   CONFIG_FILE=.env.production scripts/seedance-billing-preflight.sh
#   CHECK_HTTP=true BASE_URL=https://your-domain scripts/seedance-billing-preflight.sh
#
# Optional controls:
#   LOAD_ENV_FILE=true              Load CONFIG_FILE if present.
#   CONFIG_FILE=.env                Env file to load.
#   MODEL=doubao-seedance-2-0       Model name used for warnings.
#   REQUIRE_LOCAL_WORKER=false      Treat local worker config problems as failures.
#   USAGE_CONFIRMED=false           Mark upstream usage.total_tokens as verified.
#   SEEDANCE_REQUIRE_PRICE_CONFIRMATION=false  Runtime gate requiring explicit price signoff.
#   SEEDANCE_PRICE_CONFIRMED=false             Set true after customer price is configured.
#   FAIL_ON_WARNINGS=false          Exit non-zero when warnings exist.
#   CHECK_HTTP=false                Check BASE_URL /api/status reachability.
#   CHECK_BILLING_METRICS=false     Check BASE_URL /api/billing/metrics with admin credentials.
#   ADMIN_ACCESS_TOKEN=...          Admin access token for CHECK_BILLING_METRICS.
#   ADMIN_USER_ID=1                 Admin user id for CHECK_BILLING_METRICS New-Api-User header.

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

CONFIG_FILE="${CONFIG_FILE:-${ROOT_DIR}/.env}"
LOAD_ENV_FILE="${LOAD_ENV_FILE:-true}"

if [[ "${LOAD_ENV_FILE}" == "true" && -f "${CONFIG_FILE}" ]]; then
  set -a
  # shellcheck disable=SC1090
  source "${CONFIG_FILE}"
  set +a
fi

FAILS=0
WARNS=0
CHECKS=0

lower() {
  printf '%s' "$1" | tr '[:upper:]' '[:lower:]'
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

info() {
  CHECKS=$((CHECKS + 1))
  printf '[INFO] %s\n' "$1"
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

is_int() {
  [[ "${1:-}" =~ ^-?[0-9]+$ ]]
}

check_int_range() {
  local name="$1"
  local value="$2"
  local default_value="$3"
  local min_value="$4"
  local max_value="$5"

  if [[ -z "${value}" ]]; then
    value="${default_value}"
    info "${name} not set, using default ${default_value}."
  fi
  if ! is_int "${value}"; then
    fail "${name} must be an integer, got '${value}'."
    return
  fi
  if (( value < min_value || value > max_value )); then
    fail "${name}=${value} is outside expected range ${min_value}-${max_value}."
    return
  fi
  pass "${name}=${value} is within expected range ${min_value}-${max_value}."
}

csv_contains_seedance() {
  local raw="$1"
  local normalized
  normalized="$(lower ",${raw},")"
  [[ "${normalized}" == *seedance* ]]
}

validate_domain_allowlist() {
  local name="$1"
  local value="$2"
  local required_hint="$3"

  if [[ -z "${value}" ]]; then
    warn "${name} is empty. ${required_hint}"
    return
  fi

  local invalid=0
  local item trimmed
  IFS=',' read -r -a items <<< "${value}"
  for item in "${items[@]}"; do
    trimmed="$(printf '%s' "${item}" | xargs)"
    if [[ -z "${trimmed}" ]]; then
      continue
    fi
    if [[ "${trimmed}" == "*." || "${trimmed}" == "*" ]]; then
      printf '[WARN] %s contains too-broad wildcard item: %s\n' "${name}" "${trimmed}"
      invalid=1
      continue
    fi
    if [[ "${trimmed}" == *" "* ]]; then
      printf '[WARN] %s contains whitespace inside item: %s\n' "${name}" "${trimmed}"
      invalid=1
      continue
    fi
  done

  if (( invalid == 1 )); then
    WARNS=$((WARNS + 1))
    CHECKS=$((CHECKS + 1))
  else
    pass "${name} is configured: ${value}."
  fi
}

print_section() {
  printf '\n== %s ==\n' "$1"
}

MODEL="${MODEL:-doubao-seedance-2-0}"
BASE_URL="${BASE_URL:-http://localhost:3000}"
REQUIRE_LOCAL_WORKER="${REQUIRE_LOCAL_WORKER:-false}"
USAGE_CONFIRMED="${USAGE_CONFIRMED:-false}"
FAIL_ON_WARNINGS="${FAIL_ON_WARNINGS:-false}"
CHECK_HTTP="${CHECK_HTTP:-false}"
CHECK_BILLING_METRICS="${CHECK_BILLING_METRICS:-false}"
HTTP_TIMEOUT_SECONDS="${HTTP_TIMEOUT_SECONDS:-5}"
METRICS_MODEL="${METRICS_MODEL:-${MODEL}%}"

printf 'Seedance 2.0 billing preflight\n'
printf 'Root: %s\n' "${ROOT_DIR}"
printf 'Config: %s%s\n' "${CONFIG_FILE}" "$([[ -f "${CONFIG_FILE}" ]] && printf ' (loaded)' || printf ' (not found)')"
printf 'Model: %s\n' "${MODEL}"

print_section "Worker and task lifecycle"
if is_false "${UPDATE_TASK:-true}"; then
  if is_true "${REQUIRE_LOCAL_WORKER}"; then
    fail "UPDATE_TASK=false. This node will not poll async Seedance tasks."
  else
    warn "UPDATE_TASK=false on this node. Ensure another master/worker has UPDATE_TASK=true."
  fi
else
  pass "UPDATE_TASK is enabled or defaults to true."
fi

if [[ "$(lower "${NODE_TYPE:-master}")" == "slave" ]]; then
  if is_true "${REQUIRE_LOCAL_WORKER}"; then
    fail "NODE_TYPE=slave. This node is not a master task worker."
  else
    warn "NODE_TYPE=slave. Ensure at least one master node runs task polling."
  fi
else
  pass "NODE_TYPE is master-compatible or unset."
fi

check_int_range "TASK_TIMEOUT_MINUTES" "${TASK_TIMEOUT_MINUTES:-}" "1440" 1 10080

print_section "Billing strategy"
if is_true "${SEEDANCE_BILLING_STRICT_USAGE:-false}"; then
  if is_true "${USAGE_CONFIRMED}"; then
    pass "SEEDANCE_BILLING_STRICT_USAGE=true and USAGE_CONFIRMED=true."
  else
    warn "SEEDANCE_BILLING_STRICT_USAGE=true but USAGE_CONFIRMED is not true. Successful tasks may be refunded if upstream usage is missing."
  fi
else
  pass "SEEDANCE_BILLING_STRICT_USAGE is disabled or unset."
fi

if is_false "${SEEDANCE_BILLING_BY_USAGE:-true}"; then
  info "SEEDANCE_BILLING_BY_USAGE=false. Fixed-price billing will be used without usage delta settlement."
else
  pass "SEEDANCE_BILLING_BY_USAGE is enabled or defaults to true."
fi

if csv_contains_seedance "${TASK_PRICE_PATCH:-}"; then
  pass "TASK_PRICE_PATCH contains a Seedance model pattern for per-call billing."
else
  warn "TASK_PRICE_PATCH does not contain Seedance. Ensure model_price or model_ratio is configured in the admin pricing settings for ${MODEL}."
fi

if is_true "${SEEDANCE_REQUIRE_PRICE_CONFIRMATION:-false}"; then
  if is_true "${SEEDANCE_PRICE_CONFIRMED:-false}"; then
    pass "SEEDANCE_REQUIRE_PRICE_CONFIRMATION=true and SEEDANCE_PRICE_CONFIRMED=true."
  else
    fail "SEEDANCE_REQUIRE_PRICE_CONFIRMATION=true but SEEDANCE_PRICE_CONFIRMED is not true. Seedance tasks will be blocked until the customer price is confirmed."
  fi
else
  warn "SEEDANCE_REQUIRE_PRICE_CONFIRMATION=false. Production should enable it and set SEEDANCE_PRICE_CONFIRMED=true after configuring customer pricing."
fi

print_section "Seedance request limits"
check_int_range "SEEDANCE_DEFAULT_DURATION" "${SEEDANCE_DEFAULT_DURATION:-}" "5" 1 600
check_int_range "SEEDANCE_MAX_DURATION" "${SEEDANCE_MAX_DURATION:-}" "60" 1 600
check_int_range "SEEDANCE_MAX_IMAGES" "${SEEDANCE_MAX_IMAGES:-}" "8" 1 32
check_int_range "SEEDANCE_MAX_REFERENCE_VIDEOS" "${SEEDANCE_MAX_REFERENCE_VIDEOS:-}" "3" 0 10

print_section "Remote URL controls"
validate_domain_allowlist "SEEDANCE_REMOTE_URL_ALLOWLIST" "${SEEDANCE_REMOTE_URL_ALLOWLIST:-}" "Production should restrict image/video assets to customer OSS/CDN domains."
if [[ -n "${CALLBACK_URL:-}" ]]; then
  validate_domain_allowlist "SEEDANCE_CALLBACK_URL_ALLOWLIST" "${SEEDANCE_CALLBACK_URL_ALLOWLIST:-}" "CALLBACK_URL is set, so callback domains should be restricted."
else
  if [[ -n "${SEEDANCE_CALLBACK_URL_ALLOWLIST:-}" ]]; then
    pass "SEEDANCE_CALLBACK_URL_ALLOWLIST is configured."
  else
    info "SEEDANCE_CALLBACK_URL_ALLOWLIST is empty. This is acceptable if callback_url is disabled for customers."
  fi
fi

print_section "Billing operations"
if is_true "${BILLING_STATEMENT_AUTO_ENABLED:-false}"; then
  pass "BILLING_STATEMENT_AUTO_ENABLED=true."
  check_int_range "BILLING_STATEMENT_AUTO_DAY" "${BILLING_STATEMENT_AUTO_DAY:-}" "1" 1 28
  check_int_range "BILLING_STATEMENT_AUTO_HOUR" "${BILLING_STATEMENT_AUTO_HOUR:-}" "2" 0 23
  check_int_range "BILLING_STATEMENT_AUTO_CHECK_INTERVAL_MINUTES" "${BILLING_STATEMENT_AUTO_CHECK_INTERVAL_MINUTES:-}" "60" 1 1440
else
  info "BILLING_STATEMENT_AUTO_ENABLED=false or unset. Use manual statement generation until the billing cycle is confirmed."
fi

check_int_range "BILLING_ALERT_TASK_TIMEOUT_SECONDS" "${BILLING_ALERT_TASK_TIMEOUT_SECONDS:-}" "3600" 60 86400
check_int_range "BILLING_ALERT_PENDING_TASK_COUNT" "${BILLING_ALERT_PENDING_TASK_COUNT:-}" "20" 1 100000

print_section "Smoke test prerequisites"
if [[ -n "${API_KEY:-}" ]]; then
  pass "API_KEY is set for scripts/seedance-billing-smoke.sh."
else
  warn "API_KEY is not set. Real Seedance smoke test cannot run yet."
fi

if ! command -v jq >/dev/null 2>&1; then
  warn "jq is not installed. scripts/seedance-billing-smoke.sh requires jq."
else
  pass "jq is installed."
fi
if ! command -v curl >/dev/null 2>&1; then
  warn "curl is not installed. Smoke and optional HTTP checks require curl."
else
  pass "curl is installed."
fi

if is_true "${CHECK_HTTP}"; then
  print_section "HTTP reachability"
  if command -v curl >/dev/null 2>&1; then
    http_code="$(curl -sS -m "${HTTP_TIMEOUT_SECONDS}" -o /tmp/seedance-preflight-status.json -w '%{http_code}' "${BASE_URL%/}/api/status" || true)"
    if [[ "${http_code}" =~ ^2[0-9][0-9]$ ]]; then
      pass "${BASE_URL%/}/api/status returned HTTP ${http_code}."
    else
      warn "${BASE_URL%/}/api/status returned HTTP ${http_code:-curl_error}."
    fi
    rm -f /tmp/seedance-preflight-status.json
  else
    warn "Skipping CHECK_HTTP because curl is not installed."
  fi
fi

if is_true "${CHECK_BILLING_METRICS}"; then
  print_section "Billing metrics"
  if ! command -v curl >/dev/null 2>&1; then
    fail "Cannot check /api/billing/metrics because curl is not installed."
  elif [[ -z "${ADMIN_ACCESS_TOKEN:-}" || -z "${ADMIN_USER_ID:-}" ]]; then
    fail "CHECK_BILLING_METRICS=true requires ADMIN_ACCESS_TOKEN and ADMIN_USER_ID."
  else
    metrics_url="${BASE_URL%/}/api/billing/metrics"
    metrics_file="$(mktemp -t seedance-billing-metrics.XXXXXX)"
    http_code="$(curl -sS -G -m "${HTTP_TIMEOUT_SECONDS}" \
      -H "Authorization: ${ADMIN_ACCESS_TOKEN}" \
      -H "New-Api-User: ${ADMIN_USER_ID}" \
      --data-urlencode "model_name=${METRICS_MODEL}" \
      -o "${metrics_file}" \
      -w '%{http_code}' \
      "${metrics_url}" || true)"
    if [[ ! "${http_code}" =~ ^2[0-9][0-9]$ ]]; then
      fail "${metrics_url}?model_name=${METRICS_MODEL} returned HTTP ${http_code:-curl_error}."
    elif grep -q "newapi_billing_net_quota" "${metrics_file}" &&
      grep -q "newapi_billing_task_failure_rate" "${metrics_file}" &&
      grep -q "newapi_billing_worker_lag_seconds" "${metrics_file}"; then
      pass "/api/billing/metrics returned required Seedance billing metrics."
    else
      fail "/api/billing/metrics did not include required billing metric names."
    fi
    rm -f "${metrics_file}"
  fi
else
  info "CHECK_BILLING_METRICS=false. Set it true in pre-production to validate Prometheus scraping."
fi

print_section "Summary"
printf 'Checks: %d, warnings: %d, failures: %d\n' "${CHECKS}" "${WARNS}" "${FAILS}"

if (( FAILS > 0 )); then
  printf 'Preflight failed. Fix failures before production rollout.\n' >&2
  exit 1
fi
if (( WARNS > 0 )) && is_true "${FAIL_ON_WARNINGS}"; then
  printf 'Preflight has warnings and FAIL_ON_WARNINGS=true.\n' >&2
  exit 1
fi
if (( WARNS > 0 )); then
  printf 'Preflight completed with warnings. Review them before customer delivery.\n'
else
  printf 'Preflight passed.\n'
fi
