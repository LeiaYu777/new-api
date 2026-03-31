#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUTPUT_PATH="${1:-${ROOT_DIR}/compliance/sbom.spdx.json}"

mkdir -p "$(dirname "${OUTPUT_PATH}")"

docker run --rm -v "${ROOT_DIR}:/src" anchore/syft:latest /src -o spdx-json > "${OUTPUT_PATH}"
echo "SBOM generated at ${OUTPUT_PATH}"
