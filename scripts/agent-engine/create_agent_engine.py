#!/usr/bin/env python3
"""
Create a Vertex AI Agent Engine (Reasoning Engine) for session management.

This script creates a minimal Reasoning Engine that serves as a session store
backend for the ADK-based Go backend.

Usage:
    python create_agent_engine.py

Environment variables:
    GOOGLE_CLOUD_PROJECT: GCP project ID
    GOOGLE_CLOUD_LOCATION: Region (default: us-central1)
"""

import os
import sys

from google.cloud import aiplatform


def create_agent_engine():
    """Create a Reasoning Engine for session management."""
    project_id = os.environ.get("GOOGLE_CLOUD_PROJECT", "ai-hackathon-e04d2")
    location = os.environ.get("GOOGLE_CLOUD_LOCATION", "us-central1")

    print(f"Project: {project_id}")
    print(f"Location: {location}")

    # Initialize Vertex AI
    aiplatform.init(project=project_id, location=location)

    # Define a minimal agent class for the Reasoning Engine
    # The actual agent logic runs in our Go backend; this is just for session storage
    class PhotoCoachSessionAgent:
        """Minimal agent for session management backend."""

        def __init__(self):
            self.name = "photo_coach_session_agent"

        def query(self, input_text: str) -> str:
            """Placeholder query method - actual logic is in Go backend."""
            return "This agent is used as a session backend. Use the Go API for interactions."

    # Create the Reasoning Engine
    print("\nCreating Reasoning Engine...")
    print("This may take a few minutes...")

    try:
        reasoning_engine = aiplatform.reasoning_engines.ReasoningEngine.create(
            PhotoCoachSessionAgent(),
            display_name="photo-coach-agent-engine",
            description="Session management backend for Photo Coach application",
            requirements=[],  # No additional requirements for session-only usage
        )

        print("\n" + "=" * 60)
        print("Agent Engine created successfully!")
        print("=" * 60)
        print(f"\nAgent Engine ID: {reasoning_engine.resource_name}")

        # Extract just the ID part
        engine_id = reasoning_engine.resource_name.split("/")[-1]
        print(f"\nSet this environment variable in Cloud Run:")
        print(f"  AGENT_ENGINE_ID={engine_id}")

        return engine_id

    except Exception as e:
        print(f"\nError creating Reasoning Engine: {e}")
        sys.exit(1)


if __name__ == "__main__":
    create_agent_engine()
