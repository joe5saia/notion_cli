#!/usr/bin/env bash

set -euo pipefail

# Resolve repository root relative to this script.
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN_NAME="notionctl"
BUILD_OUTPUT="${REPO_ROOT}/bin/${BIN_NAME}"

# Determine the destination for the compiled binary.
DEFAULT_GOBIN="${HOME}/go/bin"
GOBIN_PATH="$(go env GOBIN)"
if [[ -z "${GOBIN_PATH}" ]]; then
  GOBIN_PATH="${DEFAULT_GOBIN}"
fi

mkdir -p "$(dirname "${BUILD_OUTPUT}")"
mkdir -p "${GOBIN_PATH}"

cd "${REPO_ROOT}"
go build -o "${BUILD_OUTPUT}" .

cp "${BUILD_OUTPUT}" "${GOBIN_PATH}/${BIN_NAME}"

echo "Built ${BIN_NAME} → ${BUILD_OUTPUT}"
echo "Installed ${BIN_NAME} → ${GOBIN_PATH}/${BIN_NAME}"
