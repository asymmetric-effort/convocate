# SAML/SCIM Agent — multi-stage build
# Build stage: ubuntu:26.04 with Go
# Runtime stage: distroless

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
COPY src/saml-scim-agent/ .
RUN go build -o /saml-scim-agent .

# Runtime stage
FROM gcr.io/distroless/cc-debian13:debug

COPY --from=build /saml-scim-agent /usr/local/bin/saml-scim-agent

EXPOSE 8443

USER 65534:65534

ENTRYPOINT ["/usr/local/bin/saml-scim-agent"]
