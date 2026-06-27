# Convocate UI — multi-stage build
# Build stage: ubuntu:24.04 with Go (static file server)
# Runtime stage: distroless

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
COPY ui/cmd/serve/ .
RUN go build -o /convocate-ui .

# Runtime stage
FROM gcr.io/distroless/cc-debian13:nonroot

COPY --from=build /convocate-ui /usr/local/bin/convocate-ui
COPY ui/public/ /app/public/

WORKDIR /app

EXPOSE 8080

USER 65534:65534

ENV PORT=8080

ENTRYPOINT ["/usr/local/bin/convocate-ui"]
