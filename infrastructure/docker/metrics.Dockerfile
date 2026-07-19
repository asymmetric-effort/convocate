# Convocate Node Metrics — lightweight DaemonSet agent
# Reads /proc and filesystem stats, pushes to the API every 3s.

ARG UBUNTU_BASE_TAG=latest
FROM ghcr.io/asymmetric-effort/convocate/ubuntu-base:${UBUNTU_BASE_TAG} AS build

ARG GO_VERSION=1.26.3
ARG DEPS_URL

RUN curl -fsSL "${DEPS_URL}/go-1.26.4-linux-amd64.tar.gz" \
        -o go.tar.gz && \
    tar -C /usr/local -xzf go.tar.gz && \
    rm go.tar.gz

ENV PATH="/usr/local/go/bin:${PATH}"
ENV CGO_ENABLED=0

WORKDIR /build
COPY o11y/metrics/ .
RUN go build -o /node-metrics .

# Runtime: static distroless (no libc needed for a pure-Go binary)
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /node-metrics /usr/local/bin/node-metrics

USER 65534:65534

ENTRYPOINT ["/usr/local/bin/node-metrics"]
