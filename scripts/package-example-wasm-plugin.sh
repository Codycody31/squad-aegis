#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUTPUT_DIR="${OUTPUT_DIR:-$ROOT_DIR/dist/wasm-plugin-hello}"

mkdir -p "${OUTPUT_DIR}/bin/wasm"

echo "Building example WASM module (requires: rustc + wasm32-unknown-unknown)..."
(
  cd "${ROOT_DIR}"
  rustup target add wasm32-unknown-unknown 2>/dev/null || true
  rustc --edition 2021 --crate-type cdylib "${ROOT_DIR}/examples/wasm-plugin-hello/plugin.rs" \
    --target wasm32-unknown-unknown -C opt-level=s -o "${OUTPUT_DIR}/bin/wasm/plugin.wasm"
)

echo "Refreshing manifest checksum..."
SHA256="$(sha256sum "${OUTPUT_DIR}/bin/wasm/plugin.wasm" | awk '{print $1}')"
MANIFEST_OUT="${OUTPUT_DIR}/manifest.json"
if command -v jq >/dev/null 2>&1; then
  jq --arg sha "${SHA256}" '(.targets[0].sha256) = $sha' \
    "${ROOT_DIR}/examples/wasm-plugin-hello/manifest.json" > "${MANIFEST_OUT}"
else
  python3 -c "import json,sys; m=json.load(open(sys.argv[1])); m['targets'][0]['sha256']=sys.argv[2]; json.dump(m, open(sys.argv[3],'w'), indent=2); open(sys.argv[3],'a').write(chr(10))" \
    "${ROOT_DIR}/examples/wasm-plugin-hello/manifest.json" "${SHA256}" "${MANIFEST_OUT}"
fi

TARGET_LABEL="wasm-wasm"
ZIP_PATH="${OUTPUT_DIR}/wasm-hello-${TARGET_LABEL}.zip"
rm -f "${ZIP_PATH}"
(
  cd "${OUTPUT_DIR}"
  zip -q -r "${ZIP_PATH}" manifest.json bin
)

echo "Created unsigned WASM plugin bundle:"
echo "  ${ZIP_PATH}"
echo
echo "Upload via the same plugin package endpoint as native .zip bundles."
echo "Set plugins.wasm_enabled: false in config to reject wasm installs."
