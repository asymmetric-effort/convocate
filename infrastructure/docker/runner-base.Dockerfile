# Runner base image — parallel multi-stage tool downloads
# Rarely changes. Runner inherits this and adds runner binaries + Claude.

# ── Parallel download stages (BuildKit builds these concurrently) ────────────

ARG UBUNTU_BASE_TAG=latest
FROM ghcr.io/asymmetric-effort/convocate/ubuntu-base:${UBUNTU_BASE_TAG} AS dl-go
ARG GO_VERSION=1.26.4
ARG DEPS_URL
RUN curl -fsSL "${DEPS_URL}/go-1.26.4-linux-amd64.tar.gz" | \
    tar -xz -C /usr/local

ARG UBUNTU_BASE_TAG=latest
FROM ghcr.io/asymmetric-effort/convocate/ubuntu-base:${UBUNTU_BASE_TAG} AS dl-kubectl
ARG DEPS_URL
RUN curl -fsSL "${DEPS_URL}/kubectl-1.31.4-linux-amd64" \
        -o /usr/local/bin/kubectl && \
    chmod +x /usr/local/bin/kubectl

ARG UBUNTU_BASE_TAG=latest
FROM ghcr.io/asymmetric-effort/convocate/ubuntu-base:${UBUNTU_BASE_TAG} AS dl-helm
ARG DEPS_URL
RUN curl -fsSL "${DEPS_URL}/helm-3.17.3-linux-amd64.tar.gz" | \
    tar -xz -C /tmp && \
    mv /tmp/linux-amd64/helm /usr/local/bin/helm && \
    chmod +x /usr/local/bin/helm

ARG UBUNTU_BASE_TAG=latest
FROM ghcr.io/asymmetric-effort/convocate/ubuntu-base:${UBUNTU_BASE_TAG} AS dl-bun
ARG DEPS_URL
RUN apt-get update && apt-get install -y --no-install-recommends unzip && rm -rf /var/lib/apt/lists/*
RUN curl -fsSL "${DEPS_URL}/bun-1.2.19-linux-x64.zip" -o /tmp/bun.zip && \
    unzip -q /tmp/bun.zip -d /tmp/bun-extracted && \
    mkdir -p /root/.bun/bin && \
    mv /tmp/bun-extracted/bun-linux-x64/bun /root/.bun/bin/bun && \
    chmod +x /root/.bun/bin/bun && \
    rm -rf /tmp/bun.zip /tmp/bun-extracted

ARG UBUNTU_BASE_TAG=latest
FROM ghcr.io/asymmetric-effort/convocate/ubuntu-base:${UBUNTU_BASE_TAG} AS dl-leakdetector
RUN apt-get update && apt-get install -y --no-install-recommends git && rm -rf /var/lib/apt/lists/*
COPY --from=dl-go /usr/local/go /usr/local/go
ENV PATH="/usr/local/go/bin:${PATH}"
RUN git clone --depth 1 https://github.com/asymmetric-effort/leakdetector.git /tmp/ld && \
    cd /tmp/ld && \
    CGO_ENABLED=0 go build -o /usr/local/bin/leakdetector ./cmd/leakdetector && \
    rm -rf /tmp/ld

# ── Final assembly (layer order: most stable → least stable) ─────────────────

ARG UBUNTU_BASE_TAG=latest
FROM ghcr.io/asymmetric-effort/convocate/ubuntu-base:${UBUNTU_BASE_TAG}

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

# 7. GitHub Actions Runner (stable — version pinned)
ARG RUNNER_VERSION=2.335.1
RUN mkdir -p /opt/runner && \
    curl -fsSL "${DEPS_URL}/actions-runner-2.335.1-linux-x64.tar.gz" | \
    tar -xz -C /opt/runner && \
    /opt/runner/bin/installdependencies.sh || true

# 8. Runner user + docker group
RUN groupadd -f docker && \
    useradd -m -s /bin/bash runner && \
    usermod -aG docker runner && \
    chown -R runner:runner /opt/runner
