#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BUNDLE_DIR="${BUNDLE_DIR:-$ROOT_DIR/dist/native-connector-hello}"
PRIVATE_KEY_FILE="${PRIVATE_KEY_FILE:-$ROOT_DIR/dist/plugin-signing/private-key.b64}"
OUTPUT_ZIP="${OUTPUT_ZIP:-$BUNDLE_DIR/hello-connector-linux-amd64-signed.zip}"

cd "${ROOT_DIR}"
env GOCACHE=/tmp/go-build-cache go run ./scripts/sign_plugin_bundle \
  -bundle-dir "${BUNDLE_DIR}" \
  -private-key "${PRIVATE_KEY_FILE}" \
  -output "${OUTPUT_ZIP}"
