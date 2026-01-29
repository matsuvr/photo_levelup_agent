#!/bin/bash
set -euo pipefail

PROJECT_ID="${PROJECT_ID:-ai-agent-hackathon-vol}"
REGION="${REGION:-asia-northeast1}"
REPO_NAMES=("firebaseapphosting-images" "images")

echo "Cleaning up old Artifact Registry images..."

for REPO_NAME in "${REPO_NAMES[@]}"; do
  if ! gcloud artifacts repositories describe "${REPO_NAME}" \
    --project "${PROJECT_ID}" \
    --location "${REGION}" >/dev/null 2>&1; then
    echo "Repository not found: ${REPO_NAME}"
    continue
  fi

  PACKAGE_NAMES=$(gcloud artifacts packages list \
    --project "${PROJECT_ID}" \
    --location "${REGION}" \
    --repository "${REPO_NAME}" \
    --format="value(name)")

  if [ -z "${PACKAGE_NAMES}" ]; then
    echo "No packages found in ${REPO_NAME}."
    continue
  fi

  while IFS= read -r PACKAGE; do
    if [ -z "${PACKAGE}" ]; then
      continue
    fi

    DIGESTS_TO_DELETE=$(gcloud artifacts docker images list \
      "${REGION}-docker.pkg.dev/${PROJECT_ID}/${REPO_NAME}/${PACKAGE}" \
      --sort-by=CREATE_TIME \
      --format="value(digest)" | head -n -1)

    if [ -z "${DIGESTS_TO_DELETE}" ]; then
      echo "No old images to delete for ${REPO_NAME}/${PACKAGE}."
      continue
    fi

    while IFS= read -r DIGEST; do
      if [ -z "${DIGEST}" ]; then
        continue
      fi
      echo "Deleting ${REPO_NAME}/${PACKAGE}@${DIGEST}"
      gcloud artifacts docker images delete \
        "${REGION}-docker.pkg.dev/${PROJECT_ID}/${REPO_NAME}/${PACKAGE}@${DIGEST}" \
        --delete-tags \
        --quiet
    done <<< "${DIGESTS_TO_DELETE}"
  done <<< "${PACKAGE_NAMES}"
done

echo "Artifact Registry cleanup complete."
