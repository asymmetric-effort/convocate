# MinIO — multi-stage build
# Build stage: ubuntu:24.04
# Runtime stage: distroless

ARG UBUNTU_BASE_TAG=latest
FROM ghcr.io/asymmetric-effort/convocate/ubuntu-base:${UBUNTU_BASE_TAG} AS build

ARG DEPS_URL

RUN curl -fsSL "${DEPS_URL}/minio-linux-amd64" \
        -o /usr/local/bin/minio && \
    chmod +x /usr/local/bin/minio

# Runtime stage
FROM gcr.io/distroless/cc-debian13:nonroot

COPY --from=build /usr/local/bin/minio /usr/local/bin/minio

EXPOSE 9000 9001

USER 65534:65534

ENTRYPOINT ["/usr/local/bin/minio"]
CMD ["server", "/data", "--console-address", ":9001"]
