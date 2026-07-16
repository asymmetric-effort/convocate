# OpenBao — multi-stage build
# Build stage: ubuntu:24.04
# Runtime stage: distroless

FROM ubuntu:24.04 AS build

ARG OPENBAO_VERSION=2.5.5

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        ca-certificates \
        curl && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /build

RUN ARCH=$(uname -m) && \
    curl -fsSL \
        "https://github.com/openbao/openbao/releases/download/v${OPENBAO_VERSION}/bao_${OPENBAO_VERSION}_Linux_${ARCH}.tar.gz" \
        -o openbao.tar.gz && \
    tar -xzf openbao.tar.gz && \
    chmod +x bao && \
    mv bao /usr/local/bin/bao

# Create default config, data, audit, and TLS directories
RUN mkdir -p /opt/openbao/config /opt/openbao/data /opt/openbao/audit /opt/openbao/tls

# Runtime stage
FROM gcr.io/distroless/cc-debian13:debug

COPY --from=build /usr/local/bin/bao /usr/local/bin/bao
COPY --from=build --chown=65534:65534 /opt/openbao/config/ /openbao/config/
COPY --from=build --chown=65534:65534 /opt/openbao/data/ /openbao/data/
COPY --from=build --chown=65534:65534 /opt/openbao/audit/ /openbao/audit/
COPY --from=build --chown=65534:65534 /opt/openbao/tls/ /openbao/tls/

EXPOSE 8200

USER 65534:65534

ENTRYPOINT ["/usr/local/bin/bao"]
CMD ["server", "-config=/openbao/config/config.hcl"]
