# InfluxDB v2.x — multi-stage build
# Build stage: ubuntu:24.04
# Runtime stage: distroless

ARG UBUNTU_BASE_TAG=latest
FROM ghcr.io/asymmetric-effort/convocate/ubuntu-base:${UBUNTU_BASE_TAG} AS build

ARG DEPS_URL

RUN curl -fsSL "${DEPS_URL}/influxdb-2.7.11-linux-amd64.tar.gz" \
        -o /tmp/influxdb.tar.gz && \
    tar xzf /tmp/influxdb.tar.gz -C /tmp && \
    cp /tmp/influxdb2-2.7.11/usr/bin/influxd /usr/local/bin/influxd && \
    chmod +x /usr/local/bin/influxd && \
    rm -rf /tmp/influxdb*

# Runtime stage
FROM gcr.io/distroless/cc-debian13:nonroot

COPY --from=build /usr/local/bin/influxd /usr/local/bin/influxd
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

EXPOSE 8086

USER 65534:65534

ENTRYPOINT ["/usr/local/bin/influxd"]
