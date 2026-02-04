#!/usr/bin/env python3
"""
Create a Vertex AI Agent Engine (Reasoning Engine) for session management.

This script creates a minimal Reasoning Engine that serves as a session store
backend for the ADK-based Go backend.

Usage:
    python create_agent_engine.py         # Create new Agent Engine
    python create_agent_engine.py --list  # List existing engines

Environment variables:
    GOOGLE_CLOUD_PROJECT: GCP project ID
    GOOGLE_CLOUD_LOCATION: Region (default: us-central1)
    STAGING_BUCKET: GCS bucket for staging (required for creation)
"""

import os
import sys


def create_agent_engine():
    """Create a Reasoning Engine for session management."""
    import vertexai
    from vertexai.preview import reasoning_engines

    project_id = os.environ.get("GOOGLE_CLOUD_PROJECT", "ai-hackathon-e04d2")
    location = os.environ.get("GOOGLE_CLOUD_LOCATION", "us-central1")
    staging_bucket = os.environ.get("STAGING_BUCKET", f"gs://{project_id}-agent-staging")

    print(f"Project: {project_id}")
    print(f"Location: {location}")
    print(f"Staging Bucket: {staging_bucket}")

    # Initialize Vertex AI
    vertexai.init(
        project=project_id,
        location=location,
        staging_bucket=staging_bucket
    )

    # Define a minimal agent class for the Reasoning Engine
    # The actual agent logic runs in our Go backend; this is just for session storage
    class PhotoCoachSessionAgent:
        """Minimal agent for session management backend."""

        def __init__(self):
            self.name = "photo_coach_session_agent"

        def query(self, input_text: str = "") -> str:
            """Placeholder query method - actual logic is in Go backend."""
            return "This agent is used as a session backend. Use the Go API for interactions."

    # Create the Reasoning Engine
    print("\nCreating Reasoning Engine...")
    print("This may take 1-2 minutes...")

    try:
        reasoning_engine = reasoning_engines.ReasoningEngine.create(
            PhotoCoachSessionAgent(),
            display_name="photo-coach-agent-engine",
            description="Session management backend for Photo Coach application",
            requirements=["cloudpickle==3"],
            extra_packages=[],
        )

        print("\n" + "=" * 60)
        print("Agent Engine created successfully!")
        print("=" * 60)
        print(f"\nAgent Engine Resource Name: {reasoning_engine.resource_name}")

        # Extract just the ID part
        engine_id = reasoning_engine.resource_name.split("/")[-1]
        print(f"\nAgent Engine ID: {engine_id}")
        print(f"\nSet this environment variable in Cloud Run:")
        print(f"  AGENT_ENGINE_ID={engine_id}")

        return engine_id

    except Exception as e:
        print(f"\nError creating Reasoning Engine: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)


def list_agent_engines():
    """List existing Reasoning Engines."""
    from google.cloud.aiplatform_v1beta1 import ReasoningEngineServiceClient

    project_id = os.environ.get("GOOGLE_CLOUD_PROJECT", "ai-hackathon-e04d2")
    location = os.environ.get("GOOGLE_CLOUD_LOCATION", "us-central1")

    client = ReasoningEngineServiceClient(
        client_options={"api_endpoint": f"{location}-aiplatform.googleapis.com"}
    )

    parent = f"projects/{project_id}/locations/{location}"

    print(f"\nListing Reasoning Engines in {parent}...")
    engines = client.list_reasoning_engines(parent=parent)

    count = 0
    for engine in engines:
        count += 1
        engine_id = engine.name.split("/")[-1]
        print(f"\n  Display Name: {engine.display_name}")
        print(f"  Engine ID: {engine_id}")
        print(f"  Full Name: {engine.name}")
        print(f"  Create Time: {engine.create_time}")
        print("  ---")

    if count == 0:
        print("  (No engines found)")
    else:
        print(f"\nTotal: {count} engine(s)")

    return count


if __name__ == "__main__":
    if len(sys.argv) > 1 and sys.argv[1] == "--list":
        list_agent_engines()
    else:
        create_agent_engine()
