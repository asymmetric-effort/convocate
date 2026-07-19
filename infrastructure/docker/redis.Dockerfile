# Redis — multi-stage build
# Build stage: ubuntu:24.04
# Runtime stage: distroless

FROM ghcr.io/asymmetric-effort/convocate/ubuntu-base:latest AS build

ARG REDIS_VERSION=7.2.7
ARG DEPS_URL

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        build-essential \
        pkg-config && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /build

RUN curl -fsSL "${DEPS_URL}/redis-7.2.7.tar.gz" \
        -o redis.tar.gz && \
    tar -xzf redis.tar.gz && \
    cd "redis-${REDIS_VERSION}" && \
    make -j"$(nproc)" MALLOC=libc BUILD_TLS=no && \
    make install PREFIX=/opt/redis

# Runtime stage
FROM gcr.io/distroless/cc-debian13:nonroot

COPY --from=build /opt/redis/bin/redis-server /usr/local/bin/redis-server
COPY --from=build /opt/redis/bin/redis-cli /usr/local/bin/redis-cli

EXPOSE 6379

USER nonroot:nonroot

ENTRYPOINT ["/usr/local/bin/redis-server"]
CMD ["--bind", "0.0.0.0", "--port", "6379", "--protected-mode", "no", "--save", "", "--appendonly", "no"]
