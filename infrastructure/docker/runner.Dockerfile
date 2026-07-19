# GitHub Actions Self-Hosted Runner
# Inherits from runner-base (all tools + runner binary pre-installed)
# Only adds: Claude Code (changes most frequently)

ARG RUNNER_BASE_TAG=latest
FROM ghcr.io/asymmetric-effort/convocate/runner-base:${RUNNER_BASE_TAG}

# ── Claude Code (the only thing that changes frequently) ───────────────────
ARG CLAUDE_VERSION=2.1.197
RUN npm install -g @anthropic-ai/claude-code@${CLAUDE_VERSION}

# ── Verify ─────────────────────────────────────────────────────────────────
RUN claude --version

WORKDIR /opt/runner
USER runner

ENTRYPOINT ["/opt/runner/run.sh"]
