# Gatekeeper — multi-stage build
# Build stage: ubuntu:24.04 with Go
# Runtime stage: distroless

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
COPY src/gatekeeper/ .
RUN go build -o /gatekeeper .

# Runtime stage
FROM gcr.io/distroless/cc-debian13:debug

COPY --from=build /gatekeeper /usr/local/bin/gatekeeper

EXPOSE 8443

USER 65534:65534

ENTRYPOINT ["/usr/local/bin/gatekeeper"]
