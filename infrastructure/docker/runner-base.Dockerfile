# Runner base image — parallel multi-stage tool downloads
# Rarely changes. Runner inherits this and adds runner binaries + Claude.

# ── Parallel download stages (BuildKit builds these concurrently) ────────────

FROM ghcr.io/asymmetric-effort/convocate/ubuntu-base:latest AS dl-go
ARG GO_VERSION=1.26.4
ARG DEPS_URL
RUN curl -fsSL "${DEPS_URL}/go-1.26.4-linux-amd64.tar.gz" | \
    tar -xz -C /usr/local

FROM ghcr.io/asymmetric-effort/convocate/ubuntu-base:latest AS dl-kubectl
ARG DEPS_URL
RUN curl -fsSL "${DEPS_URL}/kubectl-1.31.4-linux-amd64" \
        -o /usr/local/bin/kubectl && \
    chmod +x /usr/local/bin/kubectl

FROM ghcr.io/asymmetric-effort/convocate/ubuntu-base:latest AS dl-helm
ARG DEPS_URL
RUN curl -fsSL "${DEPS_URL}/helm-3.17.3-linux-amd64.tar.gz" | \
    tar -xz -C /tmp && \
    mv /tmp/linux-amd64/helm /usr/local/bin/helm && \
    chmod +x /usr/local/bin/helm

FROM ghcr.io/asymmetric-effort/convocate/ubuntu-base:latest AS dl-bun
RUN apt-get update && apt-get install -y --no-install-recommends unzip && rm -rf /var/lib/apt/lists/*
RUN curl -fsSL https://bun.sh/install | bash

FROM ghcr.io/asymmetric-effort/convocate/ubuntu-base:latest AS dl-leakdetector
RUN apt-get update && apt-get install -y --no-install-recommends git && rm -rf /var/lib/apt/lists/*
COPY --from=dl-go /usr/local/go /usr/local/go
ENV PATH="/usr/local/go/bin:${PATH}"
RUN git clone --depth 1 https://github.com/asymmetric-effort/leakdetector.git /tmp/ld && \
    cd /tmp/ld && \
    CGO_ENABLED=0 go build -o /usr/local/bin/leakdetector ./cmd/leakdetector && \
    rm -rf /tmp/ld

# ── Final assembly (layer order: most stable → least stable) ─────────────────

FROM ghcr.io/asymmetric-effort/convocate/ubuntu-base:latest

ENV DEBIAN_FRONTEND=noninteractive

# 1. System packages (very stable)
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        git \
        gnupg \
        jq \
        libicu74 \
        libssl3 \
        make \
        openssh-client \
        python3 \
        python3-pip \
        python3-venv \
        software-properties-common \
        sudo \
        unzip \
        wget && \
    rm -rf /var/lib/apt/lists/*

# 2. Docker CLI (stable — version pinned by apt repo)
RUN curl -fsSL https://download.docker.com/linux/ubuntu/gpg | \
    gpg --dearmor -o /usr/share/keyrings/docker.gpg && \
    echo "deb [signed-by=/usr/share/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu noble stable" \
        > /etc/apt/sources.list.d/docker.list && \
    apt-get update && \
    apt-get install -y --no-install-recommends docker-ce-cli && \
    rm -rf /var/lib/apt/lists/*

# 3. Node.js (stable — major version pinned)
RUN curl -fsSL https://deb.nodesource.com/setup_24.x | bash - && \
    apt-get install -y --no-install-recommends nodejs && \
    rm -rf /var/lib/apt/lists/*

# 4. Ansible (stable — pip install)
RUN python3 -m venv /opt/ansible && \
    /opt/ansible/bin/pip install --no-cache-dir ansible && \
    ln -s /opt/ansible/bin/ansible /usr/local/bin/ansible && \
    ln -s /opt/ansible/bin/ansible-playbook /usr/local/bin/ansible-playbook && \
    ln -s /opt/ansible/bin/ansible-galaxy /usr/local/bin/ansible-galaxy

# 5. Playwright + browser deps (large, stable)
RUN npx playwright install --with-deps chromium

# 6. Copy pre-downloaded binaries (from parallel stages)
COPY --from=dl-go /usr/local/go /usr/local/go
COPY --from=dl-kubectl /usr/local/bin/kubectl /usr/local/bin/kubectl
COPY --from=dl-helm /usr/local/bin/helm /usr/local/bin/helm
COPY --from=dl-bun /root/.bun /root/.bun
COPY --from=dl-leakdetector /usr/local/bin/leakdetector /usr/local/bin/leakdetector

ENV PATH="/usr/local/go/bin:/root/.bun/bin:${PATH}"
