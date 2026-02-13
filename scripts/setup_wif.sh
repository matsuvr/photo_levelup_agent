#!/bin/bash
set -euo pipefail

# =============================================================================
# Workload Identity Federation (WIF) Setup for GitHub Actions
# =============================================================================
# Run this script once to configure WIF so GitHub Actions can authenticate
# to GCP without long-lived service account keys.
#
# Prerequisites:
#   - gcloud CLI authenticated with Owner/Editor on the target project
#   - APIs enabled: iam.googleapis.com, iamcredentials.googleapis.com
#
# Usage:
#   ./scripts/setup_wif.sh
# =============================================================================

PROJECT_ID="${PROJECT_ID:-ai-hackathon-e04d2}"
GITHUB_REPO="${GITHUB_REPO:-matsuvr/photo_levelup_agent}"

POOL_ID="github-actions-pool"
PROVIDER_ID="github-provider"
SA_NAME="github-actions-cleanup"
SA_EMAIL="${SA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com"

info()    { echo -e "\033[0;34m[INFO]\033[0m  $1"; }
success() { echo -e "\033[0;32m[OK]\033[0m    $1"; }
error()   { echo -e "\033[0;31m[ERROR]\033[0m $1"; }

PROJECT_NUMBER=$(gcloud projects describe "${PROJECT_ID}" --format="value(projectNumber)")

# ---- Enable required APIs ----------------------------------------------------
info "Enabling required APIs..."
gcloud services enable iam.googleapis.com iamcredentials.googleapis.com \
  --project "${PROJECT_ID}" --quiet
success "APIs enabled"

# ---- Workload Identity Pool --------------------------------------------------
info "Creating Workload Identity Pool..."
if gcloud iam workload-identity-pools describe "${POOL_ID}" \
  --project "${PROJECT_ID}" --location global >/dev/null 2>&1; then
  info "Pool ${POOL_ID} already exists — skipping"
else
  gcloud iam workload-identity-pools create "${POOL_ID}" \
    --project "${PROJECT_ID}" \
    --location global \
    --display-name "GitHub Actions Pool"
  success "Pool created"
fi

# ---- Workload Identity Provider ----------------------------------------------
info "Creating Workload Identity Provider..."
if gcloud iam workload-identity-pools providers describe "${PROVIDER_ID}" \
  --project "${PROJECT_ID}" --location global \
  --workload-identity-pool "${POOL_ID}" >/dev/null 2>&1; then
  info "Provider ${PROVIDER_ID} already exists — skipping"
else
  gcloud iam workload-identity-pools providers create-oidc "${PROVIDER_ID}" \
    --project "${PROJECT_ID}" \
    --location global \
    --workload-identity-pool "${POOL_ID}" \
    --display-name "GitHub Provider" \
    --issuer-uri "https://token.actions.githubusercontent.com" \
    --attribute-mapping "google.subject=assertion.sub,attribute.repository=assertion.repository" \
    --attribute-condition "assertion.repository == '${GITHUB_REPO}'"
  success "Provider created"
fi

# ---- Service Account ---------------------------------------------------------
info "Creating service account..."
if gcloud iam service-accounts describe "${SA_EMAIL}" \
  --project "${PROJECT_ID}" >/dev/null 2>&1; then
  info "Service account ${SA_EMAIL} already exists — skipping"
else
  gcloud iam service-accounts create "${SA_NAME}" \
    --project "${PROJECT_ID}" \
    --display-name "GitHub Actions Cleanup SA"
  success "Service account created"
  info "Waiting for service account to propagate..."
  sleep 10
fi

# ---- IAM Roles --------------------------------------------------------------
info "Granting IAM roles to service account..."
ROLES=(
  "roles/artifactregistry.admin"
  "roles/run.admin"
  "roles/storage.admin"
)

for ROLE in "${ROLES[@]}"; do
  gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
    --member "serviceAccount:${SA_EMAIL}" \
    --role "${ROLE}" \
    --condition None \
    --quiet >/dev/null
  success "Granted ${ROLE}"
done

# ---- WIF Binding (allow GitHub repo to impersonate SA) -----------------------
info "Binding GitHub repo to service account via WIF..."
MEMBER="principalSet://iam.googleapis.com/projects/${PROJECT_NUMBER}/locations/global/workloadIdentityPools/${POOL_ID}/attribute.repository/${GITHUB_REPO}"

gcloud iam service-accounts add-iam-policy-binding "${SA_EMAIL}" \
  --project "${PROJECT_ID}" \
  --role "roles/iam.workloadIdentityUser" \
  --member "${MEMBER}" \
  --quiet >/dev/null
success "WIF binding created"

# ---- Output ------------------------------------------------------------------
WIF_PROVIDER="projects/${PROJECT_NUMBER}/locations/global/workloadIdentityPools/${POOL_ID}/providers/${PROVIDER_ID}"

echo ""
echo "=============================================="
echo "  WIF Setup Complete"
echo "=============================================="
echo ""
echo "Add these as GitHub repository secrets:"
echo ""
echo "  WIF_PROVIDER:"
echo "    ${WIF_PROVIDER}"
echo ""
echo "  WIF_SERVICE_ACCOUNT:"
echo "    ${SA_EMAIL}"
echo ""
echo "GitHub Settings > Secrets and variables > Actions > New repository secret"
echo ""
