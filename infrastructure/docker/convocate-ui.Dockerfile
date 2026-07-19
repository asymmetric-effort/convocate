# Convocate UI — multi-stage build
# Stage 1: Bun bundles the SpecifyJS app
# Stage 2: Go compiles the static file server
# Runtime: distroless

FROM ghcr.io/asymmetric-effort/convocate/ubuntu-base:latest AS bundle

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        unzip && \
    rm -rf /var/lib/apt/lists/*

RUN curl -fsSL https://bun.sh/install | BUN_INSTALL=/usr/local bash

WORKDIR /build
COPY src/ui/package.json src/ui/bun.lock* ./
RUN bun install
COPY src/ui/src/ src/
COPY src/ui/public/ public/
RUN bun build src/app.ts --outdir public --minify --target=browser && \
    HASH=$(sha256sum public/app.js | cut -c1-8) && \
    mv public/app.js "public/app.${HASH}.js" && \
    sed -i "s|/app.js|/app.${HASH}.js|g" public/index.html

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
COPY src/ui/cmd/serve/ .
RUN go build -o /convocate-ui .

# Runtime stage
FROM gcr.io/distroless/cc-debian13:nonroot

COPY --from=build /convocate-ui /usr/local/bin/convocate-ui
COPY --from=bundle /build/public/ /app/public/
COPY src/ui/img/icons/ /app/public/img/icons/

WORKDIR /app

EXPOSE 8080

USER 65534:65534

ENV PORT=8080

ENTRYPOINT ["/usr/local/bin/convocate-ui"]
