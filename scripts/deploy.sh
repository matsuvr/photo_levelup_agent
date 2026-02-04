#!/bin/bash

# =============================================================================
# Photo Level Up Agent - Deployment Script
# =============================================================================
# This script handles deployment of both frontend and backend components.
#
# Components:
#   - Frontend (Next.js): Firebase App Hosting (deploys via Git push)
#   - Backend (Go API): Cloud Run (direct deployment)
#
# Usage:
#   ./scripts/deploy.sh              # Deploy both frontend and backend
#   ./scripts/deploy.sh backend      # Deploy backend only
#   ./scripts/deploy.sh frontend     # Trigger frontend deployment (git push)
#   ./scripts/deploy.sh cleanup      # Remove orphaned resources
#   ./scripts/deploy.sh setup        # Initial setup (create App Hosting backend)
#   ./scripts/deploy.sh status       # Show current deployment status
# =============================================================================

set -e

# --- Configuration ---
PROJECT_ID="ai-agent-hackathon-vol"
BACKEND_REGION="asia-northeast1"      # Go API runs here (closer to Japan users)
FRONTEND_REGION="us-central1"          # App Hosting only supports us-central1 currently
BACKEND_API_SERVICE="photo-coach-api"
FRONTEND_BACKEND_ID="photo-coach-frontend"
GITHUB_REPO="matsuvr/photo_levelup_agent"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Helper functions
info() { echo -e "${BLUE}ℹ️  $1${NC}"; }
success() { echo -e "${GREEN}✅ $1${NC}"; }
warn() { echo -e "${YELLOW}⚠️  $1${NC}"; }
error() { echo -e "${RED}❌ $1${NC}"; }

# Ensure we are in the project root
cd "$(dirname "$0")/.."
PROJECT_ROOT=$(pwd)

# Load environment variables from .env if it exists
load_env() {
    if [ -f .env ]; then
        info "Loading .env file..."
        set -a
        source .env
        set +a
    fi
}

# Check required environment variables
check_env() {
    if [ -z "$GOOGLE_API_KEY" ]; then
        error "GOOGLE_API_KEY is not set."
        echo "Please set it in your environment or in a .env file at the project root."
        exit 1
    fi
}

# Show current deployment status
show_status() {
    echo ""
    echo "=============================================="
    echo "📊 Current Deployment Status"
    echo "=============================================="
    echo ""

    info "Cloud Run Services:"
    gcloud run services list --project "$PROJECT_ID" --format="table(SERVICE,REGION,URL)" 2>/dev/null || echo "  (unable to fetch)"

    echo ""
    info "Firebase App Hosting Backends:"
    firebase apphosting:backends:list --project "$PROJECT_ID" 2>/dev/null || echo "  (unable to fetch)"

    echo ""
}

# Deploy backend (Go API) to Cloud Run
deploy_backend() {
    echo ""
    echo "=============================================="
    echo "🚀 Deploying Backend (Go API) to Cloud Run"
    echo "=============================================="
    echo ""

    load_env
    check_env

    info "Deploying $BACKEND_API_SERVICE to $BACKEND_REGION..."
    cd "$PROJECT_ROOT/backend"

    gcloud run deploy "$BACKEND_API_SERVICE" \
        --source . \
        --region "$BACKEND_REGION" \
        --project "$PROJECT_ID" \
        --allow-unauthenticated \
        --set-env-vars "GOOGLE_API_KEY=$GOOGLE_API_KEY" \
        --quiet

    BACKEND_URL=$(gcloud run services describe "$BACKEND_API_SERVICE" \
        --region "$BACKEND_REGION" \
        --project "$PROJECT_ID" \
        --format='value(status.url)')

    cd "$PROJECT_ROOT"

    success "Backend deployed: $BACKEND_URL"

    # Update apphosting.yaml with the new backend URL
    info "Updating frontend/apphosting.yaml with new backend URL..."
    if [ -f "frontend/apphosting.yaml" ]; then
        # Use sed to update BACKEND_BASE_URL
        sed -i "s|value: https://.*\.run\.app|value: $BACKEND_URL|g" frontend/apphosting.yaml
        success "Updated BACKEND_BASE_URL in apphosting.yaml"
    fi

    echo ""
    echo "Backend URL: $BACKEND_URL"
}

# Trigger frontend deployment via Git push
deploy_frontend() {
    echo ""
    echo "=============================================="
    echo "🚀 Triggering Frontend Deployment (Git Push)"
    echo "=============================================="
    echo ""

    # Check for uncommitted changes
    if [ -n "$(git status --porcelain)" ]; then
        warn "Found uncommitted changes."
        echo "Please commit your changes before deploying:"
        echo "  git add . && git commit -m 'Your message'"
        exit 1
    fi

    CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
    info "Pushing $CURRENT_BRANCH to GitHub to trigger Firebase App Hosting..."

    git push origin "$CURRENT_BRANCH"

    success "Git push completed!"
    echo ""
    echo "Monitor deployment at:"
    echo "  https://console.firebase.google.com/project/$PROJECT_ID/apphosting"
}

# Initial setup - create Firebase App Hosting backend
setup_apphosting() {
    echo ""
    echo "=============================================="
    echo "🔧 Setting up Firebase App Hosting"
    echo "=============================================="
    echo ""

    info "This will create a new Firebase App Hosting backend."
    echo ""
    echo "Configuration:"
    echo "  Backend ID: $FRONTEND_BACKEND_ID"
    echo "  Region: $FRONTEND_REGION"
    echo "  Root Directory: frontend"
    echo "  GitHub Repository: $GITHUB_REPO"
    echo ""

    read -p "Continue? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Aborted."
        exit 0
    fi

    info "Creating App Hosting backend..."
    firebase apphosting:backends:create \
        --project "$PROJECT_ID" \
        --primary-region "$FRONTEND_REGION" \
        --backend "$FRONTEND_BACKEND_ID" \
        --root-dir "frontend"

    echo ""
    warn "GitHub repository connection must be configured manually."
    echo "Please go to Firebase Console to connect the repository:"
    echo "  https://console.firebase.google.com/project/$PROJECT_ID/apphosting"

    success "Firebase App Hosting backend created!"
    echo ""
    echo "Next steps:"
    echo "  1. Verify the backend is connected to GitHub"
    echo "  2. Run: ./scripts/deploy.sh frontend"
}

# Cleanup orphaned resources
cleanup() {
    echo ""
    echo "=============================================="
    echo "🧹 Cleanup Orphaned Resources"
    echo "=============================================="
    echo ""

    show_status

    echo ""
    warn "This will remove resources that are no longer needed."
    echo ""
    echo "Resources to potentially remove:"
    echo ""

    # Check for old backend services
    OLD_SERVICES=$(gcloud run services list --project "$PROJECT_ID" \
        --format="value(SERVICE,REGION)" 2>/dev/null | grep -v "$BACKEND_API_SERVICE" || true)

    if [ -n "$OLD_SERVICES" ]; then
        echo "  Cloud Run services (not $BACKEND_API_SERVICE):"
        echo "$OLD_SERVICES" | while read svc region; do
            echo "    - $svc ($region)"
        done
    fi

    # Check for old App Hosting backends
    OLD_BACKENDS=$(firebase apphosting:backends:list --project "$PROJECT_ID" \
        --format="json" 2>/dev/null | grep -o '"backendId":"[^"]*"' | cut -d'"' -f4 | grep -v "$FRONTEND_BACKEND_ID" || true)

    if [ -n "$OLD_BACKENDS" ]; then
        echo "  App Hosting backends (not $FRONTEND_BACKEND_ID):"
        echo "$OLD_BACKENDS" | while read backend; do
            echo "    - $backend"
        done
    fi

    echo ""
    read -p "Remove these resources? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Aborted."
        exit 0
    fi

    # Delete old Cloud Run services
    if [ -n "$OLD_SERVICES" ]; then
        echo "$OLD_SERVICES" | while read svc region; do
            if [ -n "$svc" ] && [ -n "$region" ]; then
                info "Deleting Cloud Run service: $svc in $region..."
                gcloud run services delete "$svc" \
                    --region "$region" \
                    --project "$PROJECT_ID" \
                    --quiet || warn "Failed to delete $svc"
            fi
        done
    fi

    # Delete old App Hosting backends
    if [ -n "$OLD_BACKENDS" ]; then
        echo "$OLD_BACKENDS" | while read backend; do
            if [ -n "$backend" ]; then
                info "Deleting App Hosting backend: $backend..."
                firebase apphosting:backends:delete "$backend" \
                    --project "$PROJECT_ID" \
                    --force || warn "Failed to delete $backend"
            fi
        done
    fi

    success "Cleanup completed!"
}

# Main deployment (both frontend and backend)
deploy_all() {
    deploy_backend
    deploy_frontend
}

# Print usage
usage() {
    echo "Usage: $0 [command]"
    echo ""
    echo "Commands:"
    echo "  (none)    Deploy both backend and frontend"
    echo "  backend   Deploy backend (Go API) only"
    echo "  frontend  Trigger frontend deployment (git push)"
    echo "  setup     Initial setup (create App Hosting backend)"
    echo "  cleanup   Remove orphaned resources"
    echo "  status    Show current deployment status"
    echo ""
}

# Main
case "${1:-}" in
    backend)
        deploy_backend
        ;;
    frontend)
        deploy_frontend
        ;;
    setup)
        setup_apphosting
        ;;
    cleanup)
        cleanup
        ;;
    status)
        show_status
        ;;
    help|--help|-h)
        usage
        ;;
    "")
        deploy_all
        ;;
    *)
        error "Unknown command: $1"
        usage
        exit 1
        ;;
esac

echo ""
echo "=============================================="
echo "Done!"
echo "=============================================="
