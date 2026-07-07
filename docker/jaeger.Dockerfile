# Jaeger v2 — multi-stage build
# Build stage: ubuntu:24.04 (download binary)
# Runtime stage: distroless

FROM ubuntu:24.04 AS build

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        ca-certificates \
        curl && \
    rm -rf /var/lib/apt/lists/*

RUN curl -fsSL https://github.com/jaegertracing/jaeger/releases/download/v2.19.0/jaeger-2.19.0-linux-amd64.tar.gz \
        -o /tmp/jaeger.tar.gz && \
    tar xzf /tmp/jaeger.tar.gz -C /tmp && \
    cp /tmp/jaeger-2.19.0-linux-amd64/jaeger /usr/local/bin/jaeger && \
    chmod +x /usr/local/bin/jaeger && \
    rm -rf /tmp/jaeger*

# Runtime stage
FROM gcr.io/distroless/cc-debian13:nonroot

COPY --from=build /usr/local/bin/jaeger /usr/local/bin/jaeger
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

EXPOSE 4317 4318 16686

USER 65534:65534

ENTRYPOINT ["/usr/local/bin/jaeger"]
