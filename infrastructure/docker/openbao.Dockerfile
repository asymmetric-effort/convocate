# OpenBao — multi-stage build
# Build stage: ubuntu:24.04
# Runtime stage: distroless

FROM ghcr.io/asymmetric-effort/convocate/ubuntu-base:latest AS build

ARG OPENBAO_VERSION=2.5.5
ARG DEPS_URL

WORKDIR /build

RUN curl -fsSL "${DEPS_URL}/openbao-2.5.5-linux-amd64.tar.gz" \
        -o openbao.tar.gz && \
    tar -xzf openbao.tar.gz && \
    chmod +x bao && \
    mv bao /usr/local/bin/bao

# Create default config for filesystem-backed storage
RUN mkdir -p /opt/openbao/config /opt/openbao/data /opt/openbao/tls && \
    printf 'storage "file" {\n  path = "/openbao/data"\n}\n\nlistener "tcp" {\n  address     = "0.0.0.0:8200"\n  tls_disable = true\n}\n\naudit {\n  type = "file"\n  path = "file"\n  description = "Audit log"\n  options = {\n    file_path = "/openbao/audit/audit.log"\n  }\n}\n\ntelemetry {\n  prometheus_retention_time = "30s"\n  disable_hostname         = true\n  unauthenticated_metrics_access = true\n}\n\napi_addr      = "https://auth.asymmetric-effort.com"\ndisable_mlock = true\nui            = true\n' > /opt/openbao/config/config.hcl && \
    mkdir -p /opt/openbao/audit

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
