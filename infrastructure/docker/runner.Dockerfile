# GitHub Actions Self-Hosted Runner
# Inherits from runner-base (all tools pre-installed)
# Only adds: runner binary, Claude, user setup
# Layer order: most stable → least stable for fast rebuilds

ARG RUNNER_BASE_TAG=latest
FROM ghcr.io/asymmetric-effort/convocate/runner-base:${RUNNER_BASE_TAG}

# ── Runner user + docker group (stable) ─────────────────────────────────────
RUN groupadd -f docker && \
    useradd -m -s /bin/bash runner && \
    usermod -aG docker runner

# ── GitHub Actions Runner (changes on version bump) ────────────────────────
ARG RUNNER_VERSION=2.335.1
ARG DEPS_URL
RUN mkdir -p /opt/runner && \
    curl -fsSL "${DEPS_URL}/actions-runner-2.335.1-linux-x64.tar.gz" | \
    tar -xz -C /opt/runner && \
    /opt/runner/bin/installdependencies.sh || true && \
    chown -R runner:runner /opt/runner

# ── Claude Code (changes most frequently) ──────────────────────────────────
ARG CLAUDE_VERSION=2.1.197
RUN npm install -g @anthropic-ai/claude-code@${CLAUDE_VERSION}

# ── Verify installations ────────────────────────────────────────────────────
RUN go version && \
    bun --version && \
    node --version && \
    docker --version && \
    kubectl version --client && \
    helm version --short && \
    ansible --version | head -1 && \
    leakdetector --version && \
    claude --version

WORKDIR /opt/runner
USER runner

ENTRYPOINT ["/opt/runner/run.sh"]
