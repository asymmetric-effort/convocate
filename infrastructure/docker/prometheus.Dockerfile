# Prometheus — multi-stage build
# Build stage: ubuntu:26.04 (download binary)
# Runtime stage: distroless

ARG UBUNTU_BASE_TAG=latest
FROM ghcr.io/asymmetric-effort/convocate/ubuntu-base:${UBUNTU_BASE_TAG} AS build

ARG DEPS_URL

RUN curl -fsSL "${DEPS_URL}/prometheus-3.4.1-linux-amd64.tar.gz" \
        -o /tmp/prometheus.tar.gz && \
    tar xzf /tmp/prometheus.tar.gz -C /tmp && \
    cp /tmp/prometheus-3.4.1.linux-amd64/prometheus /usr/local/bin/prometheus && \
    cp /tmp/prometheus-3.4.1.linux-amd64/promtool /usr/local/bin/promtool && \
    chmod +x /usr/local/bin/prometheus /usr/local/bin/promtool && \
    rm -rf /tmp/prometheus*

# Runtime stage
FROM gcr.io/distroless/cc-debian13:nonroot

COPY --from=build /usr/local/bin/prometheus /usr/local/bin/prometheus
COPY --from=build /usr/local/bin/promtool /usr/local/bin/promtool
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

EXPOSE 9090

USER 65534:65534

ENTRYPOINT ["/usr/local/bin/prometheus"]
