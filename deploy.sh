#!/bin/bash
set -euo pipefail

# Configuration
PROJECT_ID="${PROJECT_ID:-ai-agent-hackathon-vol}"
REGION="${REGION:-asia-northeast1}"
REPO_NAME="images"
IMAGE_NAME="photo-levelup-agent-backend"
SERVICE_NAME="photo-levelup-agent-backend"
SOURCE_IMAGE="ghcr.io/matsuvr/photo_levelup_agent-backend:latest"
TARGET_IMAGE="${REGION}-docker.pkg.dev/${PROJECT_ID}/${REPO_NAME}/${IMAGE_NAME}:latest"

echo "Starting deployment process..."

# 1. Pull image from GHCR
echo "Pulling image from ${SOURCE_IMAGE}..."
docker pull ${SOURCE_IMAGE}

# 2. Tag image for Artifact Registry
echo "Tagging image..."
docker tag ${SOURCE_IMAGE} ${TARGET_IMAGE}

# 3. Push to Artifact Registry
echo "Pushing image to Artifact Registry..."
docker push ${TARGET_IMAGE}

# 4. Deploy to Cloud Run
echo "Deploying to Cloud Run..."
gcloud run deploy ${SERVICE_NAME} \
  --image ${TARGET_IMAGE} \
  --project ${PROJECT_ID} \
  --region ${REGION} \
  --platform managed \
  --allow-unauthenticated \
  --set-env-vars=GOOGLE_CLOUD_PROJECT=${PROJECT_ID},GOOGLE_CLOUD_LOCATION=global,BUCKET_NAME=photo-coach,VERTEXAI_LLM=gemini-3-flash-preview,VERTEXAI_IMAGE_MODEL=gemini-3-pro-image-preview,GOOGLE_GENAI_USE_VERTEXAI=TRUE

echo "Running Artifact Registry cleanup..."
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
"${SCRIPT_DIR}/scripts/cleanup_artifact_images.sh"

echo "Deployment and cleanup complete!"
