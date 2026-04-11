#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PLUGIN_ID="${PLUGIN_ID:-com.squad-aegis.plugins.examples.hello}"
PLUGIN_NAME="${PLUGIN_NAME:-Hello Example}"
PLUGIN_DESCRIPTION="${PLUGIN_DESCRIPTION:-Replies to players who type !hello in chat.}"
PLUGIN_VERSION="${PLUGIN_VERSION:-0.1.0}"
PLUGIN_AUTHOR="${PLUGIN_AUTHOR:-Squad Aegis}"
MIN_HOST_API_VERSION="${MIN_HOST_API_VERSION:-1}"
REQUIRED_CAPABILITIES="${REQUIRED_CAPABILITIES:-api.rcon,api.connector,events.rcon}"
TARGETS="${TARGETS:-linux/$(go env GOARCH)}"
OUTPUT_DIR="${OUTPUT_DIR:-$ROOT_DIR/dist/native-plugin-hello}"
LIBRARY_NAME="${LIBRARY_NAME:-hello-example}"

rm -rf "${OUTPUT_DIR}/bin"
mkdir -p "${OUTPUT_DIR}/bin"

MANIFEST_PATH="${OUTPUT_DIR}/manifest.json"
TARGET_JSON=""
TARGET_LABELS=()
REQUIRED_CAPABILITIES_JSON=""

IFS=',' read -r -a REQUIRED_CAPABILITY_VALUES <<< "${REQUIRED_CAPABILITIES}"
for RAW_CAPABILITY in "${REQUIRED_CAPABILITY_VALUES[@]}"; do
  CAPABILITY="${RAW_CAPABILITY//[[:space:]]/}"
  if [[ -z "${CAPABILITY}" ]]; then
    continue
  fi
  if [[ -n "${REQUIRED_CAPABILITIES_JSON}" ]]; then
    REQUIRED_CAPABILITIES_JSON+=", "
  fi
  REQUIRED_CAPABILITIES_JSON+="\"${CAPABILITY}\""
done

IFS=',' read -r -a TARGET_SPECS <<< "${TARGETS}"
for RAW_TARGET_SPEC in "${TARGET_SPECS[@]}"; do
  TARGET_SPEC="${RAW_TARGET_SPEC//[[:space:]]/}"
  TARGET_OS="${TARGET_SPEC%%/*}"
  TARGET_ARCH="${TARGET_SPEC##*/}"

  if [[ -z "${TARGET_OS}" || -z "${TARGET_ARCH}" || "${TARGET_SPEC}" != */* ]]; then
    echo "Invalid TARGETS entry: ${RAW_TARGET_SPEC}. Expected os/arch, for example linux/amd64." >&2
    exit 1
  fi
  if [[ "${TARGET_OS}" != "linux" ]]; then
    echo "Native plugins currently must target linux. TARGETS entry ${TARGET_SPEC} is not supported." >&2
    exit 1
  fi

  LIBRARY_RELATIVE_PATH="bin/${TARGET_OS}-${TARGET_ARCH}/${LIBRARY_NAME}"
  LIBRARY_PATH="${OUTPUT_DIR}/${LIBRARY_RELATIVE_PATH}"
  mkdir -p "$(dirname "${LIBRARY_PATH}")"

  echo "Building subprocess-isolated native plugin binary for ${TARGET_OS}/${TARGET_ARCH}..."
  (
    cd "${ROOT_DIR}"
    env GOCACHE=/tmp/go-build-cache GOOS="${TARGET_OS}" GOARCH="${TARGET_ARCH}" \
      go build -o "${LIBRARY_PATH}" ./examples/native-plugin-hello
  )
  chmod 0755 "${LIBRARY_PATH}"

  if command -v sha256sum >/dev/null 2>&1; then
    SHA256="$(sha256sum "${LIBRARY_PATH}" | awk '{print $1}')"
  elif command -v shasum >/dev/null 2>&1; then
    SHA256="$(shasum -a 256 "${LIBRARY_PATH}" | awk '{print $1}')"
  else
    echo "Could not find sha256sum or shasum to compute the plugin checksum." >&2
    exit 1
  fi

  if [[ -n "${TARGET_JSON}" ]]; then
    TARGET_JSON+=$',\n'
  fi
  TARGET_JSON+=$(cat <<EOF
    {
      "min_host_api_version": ${MIN_HOST_API_VERSION},
      "required_capabilities": [${REQUIRED_CAPABILITIES_JSON}],
      "target_os": "${TARGET_OS}",
      "target_arch": "${TARGET_ARCH}",
      "sha256": "${SHA256}",
      "library_path": "${LIBRARY_RELATIVE_PATH}"
    }
EOF
)
  TARGET_LABELS+=("${TARGET_OS}-${TARGET_ARCH}")
done

if [[ ${#TARGET_LABELS[@]} -eq 1 ]]; then
  ZIP_PATH="${OUTPUT_DIR}/hello-example-${TARGET_LABELS[0]}.zip"
else
  ZIP_PATH="${OUTPUT_DIR}/hello-example-multi-target.zip"
fi

cat > "${MANIFEST_PATH}" <<EOF
{
  "plugin_id": "${PLUGIN_ID}",
  "name": "${PLUGIN_NAME}",
  "description": "${PLUGIN_DESCRIPTION}",
  "version": "${PLUGIN_VERSION}",
  "official": false,
  "license": "",
  "entry_symbol": "GetAegisPlugin",
  "targets": [
$(printf '%b\n' "${TARGET_JSON}")
  ]
}
EOF

rm -f "${ZIP_PATH}"
(
  cd "${OUTPUT_DIR}"
  zip -q -r "${ZIP_PATH}" manifest.json bin
)

echo "Created unsigned native plugin bundle:"
echo "  ${ZIP_PATH}"
echo
echo "For local testing, enable unsafe sideloads before uploading this bundle:"
echo "  plugins.allow_unsafe_sideload: true"
echo "or the equivalent environment/config override in your local setup."
