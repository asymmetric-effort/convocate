# Convocate Agent Container — Go wrapper + Claude CLI
# Build: ubuntu:24.04 with Go + Node.js + Claude CLI
# Runtime: minimal ubuntu:24.04 (Node.js required for Claude CLI)

# ---------------------------------------------------------------------------
# Stage 1: Build — compile Go binary and install Claude CLI
# ---------------------------------------------------------------------------
FROM ghcr.io/asymmetric-effort/convocate/ubuntu-base:latest AS build

ARG GO_VERSION=1.26.3
ARG CLAUDE_VERSION=2.1.197
ARG NODE_MAJOR=24
ARG DEPS_URL

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        gnupg && \
    rm -rf /var/lib/apt/lists/*

# Install Go
RUN curl -fsSL "${DEPS_URL}/go-1.26.4-linux-amd64.tar.gz" \
        -o go.tar.gz && \
    tar -C /usr/local -xzf go.tar.gz && \
    rm go.tar.gz

ENV PATH="/usr/local/go/bin:${PATH}"
ENV CGO_ENABLED=0

# Install Node.js (for Claude CLI runtime)
RUN curl -fsSL https://deb.nodesource.com/setup_${NODE_MAJOR}.x | bash - && \
    apt-get install -y --no-install-recommends nodejs && \
    rm -rf /var/lib/apt/lists/*

# Install Claude CLI (pinned version) and record install paths
RUN npm install -g @anthropic-ai/claude-code@${CLAUDE_VERSION} && \
    echo "Claude installed at: $(which claude)" && \
    echo "npm root: $(npm root -g)"

# Compile Go wrapper binary
WORKDIR /build
COPY src/agent/ .
RUN go build -ldflags "-X main.version=${CLAUDE_VERSION}-wrapper" \
    -o /convocate-agent-wrapper .

# ---------------------------------------------------------------------------
# Stage 2: Runtime — minimal ubuntu with Node.js + Claude CLI + Go binary
# ---------------------------------------------------------------------------
FROM ubuntu:24.04

ARG NODE_MAJOR=24

# Install only the Node.js runtime (no build tools)
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        ca-certificates \
        curl \
        gnupg && \
    curl -fsSL https://deb.nodesource.com/setup_${NODE_MAJOR}.x | bash - && \
    apt-get install -y --no-install-recommends nodejs && \
    apt-get purge -y --auto-remove gnupg && \
    rm -rf /var/lib/apt/lists/* \
        /usr/share/doc \
        /usr/share/man \
        /usr/share/info \
        /usr/share/lintian \
        /var/cache/apt \
        /tmp/*

# Copy Go wrapper binary
COPY --from=build /convocate-agent-wrapper /usr/bin/convocate-agent-wrapper

# Copy entire npm global directory (includes Claude CLI and all its deps)
COPY --from=build /usr/lib/node_modules /usr/lib/node_modules

# Copy npm global bin links
COPY --from=build /usr/bin/claude /usr/bin/claude

# Create claude user (UID 1337, GID 1337)
RUN groupadd -g 1337 claude && \
    useradd -u 1337 -g 1337 -m -s /bin/bash claude

# Create required directories
RUN mkdir -p /home/claude/workspace /tmp && \
    chown -R claude:claude /home/claude

# Runtime user
USER 1337:1337

WORKDIR /home/claude/workspace

EXPOSE 8443

ENTRYPOINT ["/usr/bin/convocate-agent-wrapper"]
