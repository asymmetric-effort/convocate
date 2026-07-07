# GitHub Actions Self-Hosted Runner
# Build stage: ubuntu:24.04
# Runtime stage: ubuntu:24.04 (needs full OS for Ansible, Docker, Playwright)

FROM ubuntu:24.04

# Avoid interactive prompts
ENV DEBIAN_FRONTEND=noninteractive

# ── System packages ──────────────────────────────────────────────────────────
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        ca-certificates \
        curl \
        git \
        gnupg \
        jq \
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

# ── Go ───────────────────────────────────────────────────────────────────────
ARG GO_VERSION=1.26.4
RUN curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" | \
    tar -xz -C /usr/local
ENV PATH="/usr/local/go/bin:${PATH}"

# ── Bun ──────────────────────────────────────────────────────────────────────
RUN curl -fsSL https://bun.sh/install | bash
ENV PATH="/root/.bun/bin:${PATH}"

# ── Node.js (for Playwright) ────────────────────────────────────────────────
RUN curl -fsSL https://deb.nodesource.com/setup_22.x | bash - && \
    apt-get install -y --no-install-recommends nodejs && \
    rm -rf /var/lib/apt/lists/*

# ── Playwright + browser deps ───────────────────────────────────────────────
RUN npx playwright install --with-deps chromium

# ── Docker CLI ───────────────────────────────────────────────────────────────
RUN curl -fsSL https://download.docker.com/linux/ubuntu/gpg | \
    gpg --dearmor -o /usr/share/keyrings/docker.gpg && \
    echo "deb [signed-by=/usr/share/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu noble stable" \
        > /etc/apt/sources.list.d/docker.list && \
    apt-get update && \
    apt-get install -y --no-install-recommends docker-ce-cli && \
    rm -rf /var/lib/apt/lists/*

# ── kubectl ──────────────────────────────────────────────────────────────────
RUN curl -fsSL "https://dl.k8s.io/release/$(curl -fsSL https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" \
        -o /usr/local/bin/kubectl && \
    chmod +x /usr/local/bin/kubectl

# ── Helm ─────────────────────────────────────────────────────────────────────
RUN curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash

# ── Ansible ──────────────────────────────────────────────────────────────────
RUN python3 -m venv /opt/ansible && \
    /opt/ansible/bin/pip install --no-cache-dir ansible && \
    ln -s /opt/ansible/bin/ansible /usr/local/bin/ansible && \
    ln -s /opt/ansible/bin/ansible-playbook /usr/local/bin/ansible-playbook && \
    ln -s /opt/ansible/bin/ansible-galaxy /usr/local/bin/ansible-galaxy

# ── leakdetector ─────────────────────────────────────────────────────────────
RUN git clone https://github.com/asymmetric-effort/leakdetector.git /tmp/leakdetector && \
    cd /tmp/leakdetector && \
    make build && \
    cp build/linux/amd64/leakdetector /usr/local/bin/leakdetector && \
    chmod +x /usr/local/bin/leakdetector && \
    rm -rf /tmp/leakdetector

# ── GitHub Actions Runner ───────────────────────────────────────────────────
ARG RUNNER_VERSION=2.322.0
RUN mkdir -p /opt/runner && \
    curl -fsSL "https://github.com/actions/runner/releases/download/v${RUNNER_VERSION}/actions-runner-linux-x64-${RUNNER_VERSION}.tar.gz" | \
    tar -xz -C /opt/runner && \
    /opt/runner/bin/installdependencies.sh

# ── Runner user ──────────────────────────────────────────────────────────────
RUN useradd -m -s /bin/bash runner && \
    usermod -aG docker runner && \
    chown -R runner:runner /opt/runner

# ── Verify installations ────────────────────────────────────────────────────
RUN go version && \
    bun --version && \
    node --version && \
    docker --version && \
    kubectl version --client && \
    helm version --short && \
    ansible --version | head -1 && \
    leakdetector --version && \
    /opt/runner/run.sh --version || true

WORKDIR /opt/runner
USER runner

ENTRYPOINT ["/opt/runner/run.sh"]
