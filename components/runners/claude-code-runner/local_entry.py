#!/usr/bin/env python3
"""Local entry point for claude-code-runner without K8s dependencies."""

import os
import sys

os.environ.setdefault("WORKSPACE_PATH", os.getcwd())
os.environ.setdefault("SESSION_ID", "local-" + os.environ.get("AGUI_PORT", "0"))
os.environ.setdefault("INTERACTIVE", "true")

port = int(os.environ.get("AGUI_PORT", "8001"))

if __name__ == "__main__":
    import uvicorn

    sys.path.insert(0, os.path.dirname(__file__))
    uvicorn.run("main:app", host="127.0.0.1", port=port, log_level="info")
