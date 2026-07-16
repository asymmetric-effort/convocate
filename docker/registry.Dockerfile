# Docker Distribution (CNCF Registry) — multi-stage build
# Build stage: ubuntu:24.04
# Runtime stage: distroless

FROM ubuntu:24.04 AS build

ARG REGISTRY_VERSION=3.1.1

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        ca-certificates \
        curl \
        openssl && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /build

RUN ARCH=$(uname -m | sed 's/x86_64/amd64/' | sed 's/aarch64/arm64/') && \
    curl -fsSL \
        "https://github.com/distribution/distribution/releases/download/v${REGISTRY_VERSION}/registry_${REGISTRY_VERSION}_linux_${ARCH}.tar.gz" \
        -o registry.tar.gz && \
    tar -xzf registry.tar.gz && \
    chmod +x registry

# Generate self-signed TLS certificate at build time
RUN openssl ecparam -genkey -name prime256v1 -noout -out /build/registry.key && \
    openssl req -new -x509 -key /build/registry.key \
        -out /build/registry.crt \
        -days 3650 \
        -subj "/CN=convocate-registry" \
        -addext "subjectAltName=IP:192.168.3.90,DNS:convocate-registry"

# Create TLS-enabled config
RUN printf 'version: 0.1\nlog:\n  fields:\n    service: registry\nstorage:\n  filesystem:\n    rootdirectory: /var/lib/registry\n  delete:\n    enabled: true\nhttp:\n  addr: 0.0.0.0:5000\n  tls:\n    certificate: /etc/distribution/tls/registry.crt\n    key: /etc/distribution/tls/registry.key\n  headers:\n    X-Content-Type-Options: [nosniff]\n' > /build/config.yml

# Runtime stage
FROM gcr.io/distroless/cc-debian13:nonroot

COPY --from=build /build/registry /usr/local/bin/registry
COPY --from=build /build/config.yml /etc/distribution/config.yml
COPY --from=build /build/registry.crt /etc/distribution/tls/registry.crt
COPY --from=build /build/registry.key /etc/distribution/tls/registry.key

EXPOSE 5000

USER nonroot:nonroot

ENTRYPOINT ["/usr/local/bin/registry"]
CMD ["serve", "/etc/distribution/config.yml"]
