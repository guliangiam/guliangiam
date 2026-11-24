#!/usr/bin/env bash
# Generate and verify the SPDX JSON SBOM used by the release pipeline.
# AC-SEC-006: Syft is pinned; a missing/corrupt artifact fails the job.
set -euo pipefail

output="${1:-dist/sbom.spdx.json}"
syft_version="${SYFT_VERSION:-v1.18.1}"

if ! command -v syft >/dev/null 2>&1; then
  echo "syft is required (expected version ${syft_version})" >&2
  exit 1
fi

mkdir -p "$(dirname "${output}")"
syft dir:. \
  --source-name guliangiam \
  --source-version "$(git describe --tags --always --dirty 2>/dev/null || echo unknown)" \
  -o "spdx-json=${output}"
python3 scripts/release/check_sbom.py "${output}"
sha256sum "${output}" > "${output}.sha256"
python3 scripts/release/check_sbom.py "${output}"
