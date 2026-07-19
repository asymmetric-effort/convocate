# Convocate Node Metrics — lightweight DaemonSet agent
# Reads /proc and filesystem stats, pushes to the API every 3s.

FROM 192.168.3.90:5000/convocate/ubuntu-base:latest AS build

ARG GO_VERSION=1.26.3

RUN ARCH=$(uname -m | sed 's/x86_64/amd64/' | sed 's/aarch64/arm64/') && \
    curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${ARCH}.tar.gz" \
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
