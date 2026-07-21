# VictoriaLogs — build from source
# Build stage: ubuntu-base + Go
# Runtime stage: distroless

ARG UBUNTU_BASE_TAG=latest
FROM ghcr.io/asymmetric-effort/convocate/ubuntu-base:${UBUNTU_BASE_TAG} AS build

ARG VICTORIALOGS_VERSION=v1.121.0

RUN apt-get update -qq && apt-get install -y -qq git golang-go make && \
    git clone --depth 1 --branch ${VICTORIALOGS_VERSION} \
      https://github.com/VictoriaMetrics/VictoriaMetrics.git /build && \
    cd /build && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
      go build -trimpath -ldflags="-s -w" \
      -o /usr/local/bin/victoria-logs-prod \
      ./app/victoria-logs && \
    chmod +x /usr/local/bin/victoria-logs-prod

# Runtime stage
FROM gcr.io/distroless/cc-debian13:nonroot

COPY --from=build /usr/local/bin/victoria-logs-prod /usr/local/bin/victoria-logs-prod
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

EXPOSE 9428

USER 65534:65534

ENTRYPOINT ["/usr/local/bin/victoria-logs-prod"]
