#!/bin/bash

# Exit on error
set -e

# --- Configuration ---
PROJECT_ID="ai-agent-hackathon-vol"
REGION="asia-northeast1"
BACKEND_SERVICE_NAME="photo-coach-backend"

echo "----------------------------------------------------------"
echo "🚀 Hybrid Deployment Script (Backend: Direct / Frontend: Git Push)"
echo "Project: $PROJECT_ID"
echo "----------------------------------------------------------"

# Ensure we are in the project root
cd "$(dirname "$0")/.."

# 1. Deploy Backend (Cloud Run)
echo ""
echo "Step 1: Deploying Backend (Go) directly to Cloud Run..."
cd backend
gcloud run deploy "$BACKEND_SERVICE_NAME" \
    --source . \
    --region "$REGION" \
    --project "$PROJECT_ID" \
    --allow-unauthenticated \
    --quiet

# Get and show the backend URL
BACKEND_URL=$(gcloud run services describe "$BACKEND_SERVICE_NAME" --region "$REGION" --project "$PROJECT_ID" --format='value(status.url)')
echo "✅ Backend deployed at: $BACKEND_URL"
cd ..

# 2. Trigger Frontend Deployment (Git Push -> App Hosting)
echo ""
echo "Step 2: Triggering Frontend deployment via Git Push..."

# Check for uncommitted changes
if [ -n "$(git status --porcelain)" ]; then
    echo "⚠️  Found uncommitted changes. Please commit your changes before deploying."
    echo "Example: git add . && git commit -m 'Your message' && ./scripts/deploy.sh"
    exit 1
fi

echo "📤 Pushing to GitHub to trigger Firebase App Hosting..."
# Push the current branch
CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
git push origin "$CURRENT_BRANCH"

echo ""
echo "----------------------------------------------------------"
echo "🎉 Backend Deployment Complete!"
echo "Backend URL: $BACKEND_URL"
echo ""
echo "📡 Frontend: Deployment triggered on GitHub."
echo "Monitor App Hosting at: https://console.firebase.google.com/project/$PROJECT_ID/apphosting"
echo "----------------------------------------------------------"
