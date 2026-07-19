# Convocate API — multi-stage build
# Build stage: ubuntu:24.04 with Go 1.26
# Runtime stage: distroless

FROM ghcr.io/asymmetric-effort/convocate/ubuntu-base:latest AS build

ARG GO_VERSION=1.26.3
ARG DEPS_URL

RUN curl -fsSL "${DEPS_URL}/go-1.26.4-linux-amd64.tar.gz" \
        -o go.tar.gz && \
    tar -C /usr/local -xzf go.tar.gz && \
    rm go.tar.gz

ENV PATH="/usr/local/go/bin:${PATH}"
ENV CGO_ENABLED=0

WORKDIR /build
COPY src/api/ .
RUN go build -o /convocate-api .

# Runtime stage — needs openssh-client and sshpass for node provisioning
FROM ubuntu:24.04 AS runtime

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        ca-certificates \
        openssh-client \
        sshpass && \
    rm -rf /var/lib/apt/lists/*

COPY --from=build /convocate-api /usr/local/bin/convocate-api

EXPOSE 8443

USER 65534:65534

ENTRYPOINT ["/usr/local/bin/convocate-api"]
