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

# Create default config for filesystem-backed storage
RUN mkdir -p /opt/openbao/config /opt/openbao/data && \
    printf 'storage "file" {\n  path = "/openbao/data"\n}\n\nlistener "tcp" {\n  address     = "0.0.0.0:8200"\n  tls_disable = true\n}\n\napi_addr      = "http://0.0.0.0:8200"\ndisable_mlock = true\nui            = false\n' > /opt/openbao/config/config.hcl

# Runtime stage
FROM gcr.io/distroless/cc-debian13:debug

COPY --from=build /usr/local/bin/bao /usr/local/bin/bao
COPY --from=build /opt/openbao/config/ /openbao/config/
COPY --from=build /opt/openbao/data/ /openbao/data/

EXPOSE 8200

ENTRYPOINT ["/usr/local/bin/bao"]
CMD ["server", "-config=/openbao/config/config.hcl"]
