# MinIO — multi-stage build
# Build stage: ubuntu:24.04
# Runtime stage: distroless

FROM 192.168.3.90:5000/convocate/ubuntu-base:latest AS build

RUN curl -fsSL https://dl.min.io/server/minio/release/linux-amd64/minio \
        -o /usr/local/bin/minio && \
    chmod +x /usr/local/bin/minio

# Runtime stage
FROM gcr.io/distroless/cc-debian13:nonroot

COPY --from=build /usr/local/bin/minio /usr/local/bin/minio

EXPOSE 9000 9001

USER 65534:65534

ENTRYPOINT ["/usr/local/bin/minio"]
CMD ["server", "/data", "--console-address", ":9001"]
