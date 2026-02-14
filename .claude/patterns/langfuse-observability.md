# Langfuse Observability Pattern

**When to load:** Working on LLM tracing, observability, or runner telemetry

## Overview

Optional Langfuse integration for LLM observability, tracking usage metrics while protecting user privacy.

## Privacy-First Design

- **Default behavior**: User messages and assistant responses are **REDACTED** in traces
- **Preserved data**: Usage metrics (tokens, costs), metadata (model, turn count, timestamps)
- **Rationale**: Track costs and usage patterns without exposing potentially sensitive user data

## Configuration

**Enable Langfuse** (disabled by default):

```bash
# In ambient-admin-langfuse-secret
LANGFUSE_ENABLED=true
LANGFUSE_PUBLIC_KEY=<your-key>
LANGFUSE_SECRET_KEY=<your-secret>
LANGFUSE_HOST=http://langfuse-web.langfuse.svc.cluster.local:3000
```

**Privacy Controls** (optional - masking enabled by default):

```bash
# Masking is ENABLED BY DEFAULT (no environment variable needed)
# The runner defaults to LANGFUSE_MASK_MESSAGES=true if not set

# To explicitly set (optional):
LANGFUSE_MASK_MESSAGES=true

# To disable masking (dev/testing ONLY - exposes full message content):
LANGFUSE_MASK_MESSAGES=false
```

## Deployment

```bash
# Deploy with default privacy-preserving settings
./e2e/scripts/deploy-langfuse.sh

# For OpenShift
./e2e/scripts/deploy-langfuse.sh --openshift

# For Kubernetes
./e2e/scripts/deploy-langfuse.sh --kubernetes
```

## Implementation

- **Location**: `components/runners/claude-code-runner/observability.py`
- **Masking function**: `_privacy_masking_function()` - redacts content while preserving metrics
- **Test coverage**: `tests/test_privacy_masking.py` - validates masking behavior

## What Gets Logged

**With Masking Enabled (Default)**:

| Category | Logged? |
|----------|---------|
| Token counts (input, output, cache read, cache creation) | Yes |
| Cost calculations (USD per session) | Yes |
| Model names and versions | Yes |
| Turn counts and session durations | Yes |
| Tool usage (names, execution status) | Yes |
| Error states and completion status | Yes |
| User prompts | **Redacted** |
| Assistant responses | **Redacted** |
| Tool outputs with long content | **Redacted** |

**With Masking Disabled** (dev/testing only): All of the above plus full message content (potentially sensitive).

## OpenTelemetry Support

- **Current implementation**: Langfuse Python SDK (v3, OTel-based)
- **Alternative**: Pure OpenTelemetry SDK via Langfuse OTLP endpoint (`/api/public/otel`)
- **Migration**: Not recommended unless vendor neutrality is required
- **Benefit**: Current SDK already uses OTel underneath
