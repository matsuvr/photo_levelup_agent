#!/bin/bash
set -euo pipefail

# =============================================================================
# Photo Level Up Agent - Artifact & Resource Cleanup Script
# =============================================================================
# Deletes old container images, Cloud Run revisions, and build source archives
# to reduce storage costs. Keeps only the latest 1 resource of each type.
#
# Usage:
#   ./scripts/cleanup_artifact_images.sh          # Run with defaults
#   DRY_RUN=1 ./scripts/cleanup_artifact_images.sh  # Preview without deleting
# =============================================================================

PROJECT_ID="${PROJECT_ID:-ai-hackathon-e04d2}"
REGION="${REGION:-us-central1}"
DRY_RUN="${DRY_RUN:-0}"

REPO_NAMES=("cloud-run-source-deploy" "firebaseapphosting-images")
EMPTY_REPO_CANDIDATES=("mcp-cloud-run-deployments")
CLOUD_RUN_SERVICE="photo-coach-api"
SOURCE_BUCKET_PREFIXES=("run-sources-" "ai-hackathon-e04d2-source-bucket")

info()    { echo "[INFO]  $1"; }
warn()    { echo "[WARN]  $1"; }
error()   { echo "[ERROR] $1"; }
dry_run() { [[ "$DRY_RUN" == "1" ]] && echo "[DRY-RUN] $*" && return 0 || return 1; }

# ---- 1. Artifact Registry: delete old images (keep latest 1) ----------------
info "=== Artifact Registry image cleanup ==="

for REPO_NAME in "${REPO_NAMES[@]}"; do
  if ! gcloud artifacts repositories describe "${REPO_NAME}" \
    --project "${PROJECT_ID}" \
    --location "${REGION}" >/dev/null 2>&1; then
    warn "Repository not found: ${REPO_NAME} — skipping"
    continue
  fi

  PACKAGE_NAMES=$(gcloud artifacts packages list \
    --project "${PROJECT_ID}" \
    --location "${REGION}" \
    --repository "${REPO_NAME}" \
    --format="value(name)" 2>/dev/null || true)

  if [ -z "${PACKAGE_NAMES}" ]; then
    info "No packages in ${REPO_NAME}"
    continue
  fi

  while IFS= read -r PACKAGE; do
    [ -z "${PACKAGE}" ] && continue

    IMAGE_PATH="${REGION}-docker.pkg.dev/${PROJECT_ID}/${REPO_NAME}/${PACKAGE}"

    DIGESTS_TO_DELETE=$(gcloud artifacts docker images list \
      "${IMAGE_PATH}" \
      --sort-by=CREATE_TIME \
      --format="value(digest)" | head -n -1)

    if [ -z "${DIGESTS_TO_DELETE}" ]; then
      info "No old images for ${REPO_NAME}/${PACKAGE}"
      continue
    fi

    while IFS= read -r DIGEST; do
      [ -z "${DIGEST}" ] && continue
      if dry_run "Would delete ${IMAGE_PATH}@${DIGEST}"; then
        continue
      fi
      info "Deleting image ${REPO_NAME}/${PACKAGE}@${DIGEST}"
      gcloud artifacts docker images delete \
        "${IMAGE_PATH}@${DIGEST}" \
        --delete-tags --quiet || warn "Failed to delete ${IMAGE_PATH}@${DIGEST}"
    done <<< "${DIGESTS_TO_DELETE}"
  done <<< "${PACKAGE_NAMES}"
done

# ---- 2. Empty Artifact Registry repositories --------------------------------
info "=== Empty repository cleanup ==="

for REPO_NAME in "${EMPTY_REPO_CANDIDATES[@]}"; do
  if ! gcloud artifacts repositories describe "${REPO_NAME}" \
    --project "${PROJECT_ID}" \
    --location "${REGION}" >/dev/null 2>&1; then
    info "Repository ${REPO_NAME} does not exist — skipping"
    continue
  fi

  PKG_COUNT=$(gcloud artifacts packages list \
    --project "${PROJECT_ID}" \
    --location "${REGION}" \
    --repository "${REPO_NAME}" \
    --format="value(name)" 2>/dev/null | wc -l)

  if [ "${PKG_COUNT}" -eq 0 ]; then
    if dry_run "Would delete empty repository ${REPO_NAME}"; then
      continue
    fi
    info "Deleting empty repository: ${REPO_NAME}"
    gcloud artifacts repositories delete "${REPO_NAME}" \
      --project "${PROJECT_ID}" \
      --location "${REGION}" \
      --quiet || warn "Failed to delete repository ${REPO_NAME}"
  else
    info "Repository ${REPO_NAME} has ${PKG_COUNT} package(s) — keeping"
  fi
done

# ---- 3. Cloud Run: delete inactive revisions --------------------------------
info "=== Cloud Run revision cleanup ==="

SERVICES=$(gcloud run services list \
  --project "${PROJECT_ID}" \
  --region "${REGION}" \
  --format="value(SERVICE)" 2>/dev/null || true)

for SERVICE in ${SERVICES}; do
  [ -z "${SERVICE}" ] && continue

  # Get the currently serving revision(s)
  ACTIVE_REVISIONS=$(gcloud run services describe "${SERVICE}" \
    --project "${PROJECT_ID}" \
    --region "${REGION}" \
    --format="value(status.traffic.revisionName)" 2>/dev/null | tr ';' '\n' | sort -u)

  ALL_REVISIONS=$(gcloud run revisions list \
    --service "${SERVICE}" \
    --project "${PROJECT_ID}" \
    --region "${REGION}" \
    --format="value(REVISION)" 2>/dev/null || true)

  for REV in ${ALL_REVISIONS}; do
    [ -z "${REV}" ] && continue
    if echo "${ACTIVE_REVISIONS}" | grep -qx "${REV}"; then
      info "Keeping active revision: ${REV}"
      continue
    fi
    if dry_run "Would delete revision ${REV}"; then
      continue
    fi
    info "Deleting inactive revision: ${REV}"
    gcloud run revisions delete "${REV}" \
      --project "${PROJECT_ID}" \
      --region "${REGION}" \
      --quiet || warn "Failed to delete revision ${REV}"
  done
done

# ---- 4. Cloud Build source buckets ------------------------------------------
info "=== Cloud Build source bucket cleanup ==="

ALL_BUCKETS=$(gsutil ls -p "${PROJECT_ID}" 2>/dev/null || true)

for PREFIX in "${SOURCE_BUCKET_PREFIXES[@]}"; do
  MATCHING_BUCKETS=$(echo "${ALL_BUCKETS}" | grep "${PREFIX}" || true)

  for BUCKET in ${MATCHING_BUCKETS}; do
    [ -z "${BUCKET}" ] && continue

    # List objects sorted by time, keep the newest one
    OBJECTS=$(gsutil ls -l "${BUCKET}" 2>/dev/null | grep -v "TOTAL:" | sed '/^$/d' | sort -k2 | head -n -1 | awk '{print $NF}')

    if [ -z "${OBJECTS}" ]; then
      info "No old objects in ${BUCKET}"
      continue
    fi

    while IFS= read -r OBJ; do
      [ -z "${OBJ}" ] && continue
      if dry_run "Would delete ${OBJ}"; then
        continue
      fi
      info "Deleting source archive: ${OBJ}"
      gsutil rm "${OBJ}" || warn "Failed to delete ${OBJ}"
    done <<< "${OBJECTS}"
  done
done

info "=== Cleanup complete ==="
