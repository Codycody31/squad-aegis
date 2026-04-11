#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CONNECTOR_ID="${CONNECTOR_ID:-com.squad-aegis.connectors.examples.hello}"
CONNECTOR_NAME="${CONNECTOR_NAME:-Hello connector example}"
CONNECTOR_DESCRIPTION="${CONNECTOR_DESCRIPTION:-Responds to JSON invoke action ping.}"
CONNECTOR_VERSION="${CONNECTOR_VERSION:-0.1.0}"
CONNECTOR_AUTHOR="${CONNECTOR_AUTHOR:-Squad Aegis}"
MIN_HOST_API_VERSION="${MIN_HOST_API_VERSION:-1}"
# Native connectors may use an empty capability set per target if none are needed.
REQUIRED_CAPABILITIES="${REQUIRED_CAPABILITIES:-}"
TARGETS="${TARGETS:-linux/$(go env GOARCH)}"
OUTPUT_DIR="${OUTPUT_DIR:-$ROOT_DIR/dist/native-connector-hello}"
LIBRARY_NAME="${LIBRARY_NAME:-hello-connector}"

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
    echo "Native connectors currently must target linux. TARGETS entry ${TARGET_SPEC} is not supported." >&2
    exit 1
  fi

  LIBRARY_RELATIVE_PATH="bin/${TARGET_OS}-${TARGET_ARCH}/${LIBRARY_NAME}"
  LIBRARY_PATH="${OUTPUT_DIR}/${LIBRARY_RELATIVE_PATH}"
  mkdir -p "$(dirname "${LIBRARY_PATH}")"

  echo "Building subprocess-isolated native connector binary for ${TARGET_OS}/${TARGET_ARCH}..."
  (
    cd "${ROOT_DIR}"
    env GOCACHE=/tmp/go-build-cache GOOS="${TARGET_OS}" GOARCH="${TARGET_ARCH}" \
      go build -o "${LIBRARY_PATH}" ./examples/native-connector-hello
  )
  chmod 0755 "${LIBRARY_PATH}"

  if command -v sha256sum >/dev/null 2>&1; then
    SHA256="$(sha256sum "${LIBRARY_PATH}" | awk '{print $1}')"
  elif command -v shasum >/dev/null 2>&1; then
    SHA256="$(shasum -a 256 "${LIBRARY_PATH}" | awk '{print $1}')"
  else
    echo "Could not find sha256sum or shasum to compute the connector checksum." >&2
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
  ZIP_PATH="${OUTPUT_DIR}/hello-connector-${TARGET_LABELS[0]}.zip"
else
  ZIP_PATH="${OUTPUT_DIR}/hello-connector-multi-target.zip"
fi


cat > "${MANIFEST_PATH}" <<EOF
{
  "connector_id": "${CONNECTOR_ID}",
  "name": "${CONNECTOR_NAME}",
  "description": "${CONNECTOR_DESCRIPTION}",
  "version": "${CONNECTOR_VERSION}",
  "official": false,
  "license": "",
  "entry_symbol": "GetAegisConnector",
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

echo "Created unsigned native connector bundle:"
echo "  ${ZIP_PATH}"
echo
echo "For local testing, enable unsafe sideloads before uploading this bundle:"
echo "  connectors.allow_unsafe_sideload / plugins.allow_unsafe_sideload: true"
echo "or the equivalent environment/config override in your local setup."
