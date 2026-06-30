# Convocate Node Metrics — lightweight DaemonSet agent
# Reads /proc and filesystem stats, pushes to the API every 3s.

FROM ubuntu:24.04 AS build

ARG GO_VERSION=1.26.3

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        ca-certificates \
        curl && \
    rm -rf /var/lib/apt/lists/*

RUN ARCH=$(uname -m | sed 's/x86_64/amd64/' | sed 's/aarch64/arm64/') && \
    curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${ARCH}.tar.gz" \
        -o go.tar.gz && \
    tar -C /usr/local -xzf go.tar.gz && \
    rm go.tar.gz

ENV PATH="/usr/local/go/bin:${PATH}"
ENV CGO_ENABLED=0

WORKDIR /build
COPY metrics/ .
RUN go build -o /node-metrics .

# Runtime: static distroless (no libc needed for a pure-Go binary)
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /node-metrics /usr/local/bin/node-metrics

USER 65534:65534

ENTRYPOINT ["/usr/local/bin/node-metrics"]
