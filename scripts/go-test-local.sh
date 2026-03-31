#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GOMODCACHE_DIR="${ROOT_DIR}/.cache/go-mod"
GOCACHE_DIR="${ROOT_DIR}/.cache/go-build"

mkdir -p "${GOMODCACHE_DIR}" "${GOCACHE_DIR}"

if [[ $# -gt 0 ]]; then
  PKGS="$*"
else
  PKGS="./..."
fi

if [[ "${SKIP_ROOT_EMBED:-1}" == "1" && "${PKGS}" == "./..." ]]; then
  PKGS="$(go list ./... | grep -v "^github.com/QuantumNous/new-api$")"
fi

GOMODCACHE="${GOMODCACHE_DIR}" GOCACHE="${GOCACHE_DIR}" go test ${PKGS}
