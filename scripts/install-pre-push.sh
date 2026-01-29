#!/bin/bash
set -euo pipefail

ROOT_DIR=$(git rev-parse --show-toplevel)
HOOK_DIR="${ROOT_DIR}/.git/hooks"
HOOK_PATH="${HOOK_DIR}/pre-push"

mkdir -p "${HOOK_DIR}"
cp "${ROOT_DIR}/scripts/pre-push" "${HOOK_PATH}"
chmod +x "${HOOK_PATH}"

echo "Installed pre-push hook to ${HOOK_PATH}"
