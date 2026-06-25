# OpenBao — multi-stage build
# Build stage: ubuntu:24.04
# Runtime stage: distroless

FROM ubuntu:24.04 AS build

ARG OPENBAO_VERSION=2.5.0
ARG TARGETARCH=amd64

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        ca-certificates \
        curl \
        unzip && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /build

RUN curl -fsSL \
        "https://github.com/openbao/openbao/releases/download/v${OPENBAO_VERSION}/bao_${OPENBAO_VERSION}_linux_${TARGETARCH}.tar.gz" \
        -o openbao.tar.gz && \
    tar -xzf openbao.tar.gz && \
    chmod +x bao && \
    mv bao /usr/local/bin/bao

# Create default config for filesystem-backed storage
RUN mkdir -p /opt/openbao/config && \
    cat > /opt/openbao/config/config.hcl <<'EOF'
storage "file" {
  path = "/openbao/data"
}

listener "tcp" {
  address     = "0.0.0.0:8200"
  tls_disable = true
}

api_addr     = "http://0.0.0.0:8200"
disable_mlock = true
ui            = false
EOF

# Runtime stage
FROM gcr.io/distroless/cc-debian12:nonroot

COPY --from=build /usr/local/bin/bao /usr/local/bin/bao
COPY --from=build /opt/openbao/config/ /openbao/config/

EXPOSE 8200

USER nonroot:nonroot

ENTRYPOINT ["/usr/local/bin/bao"]
CMD ["server", "-config=/openbao/config/config.hcl"]
