# Traefik ingress controller — mirror of upstream binary
# We mirror to GHCR for container image policy compliance.

ARG UBUNTU_BASE_TAG=latest
FROM traefik:v3.4 AS upstream

FROM ghcr.io/asymmetric-effort/convocate/ubuntu-base:${UBUNTU_BASE_TAG}

COPY --from=upstream /usr/local/bin/traefik /usr/local/bin/traefik

USER 65534:65534

ENTRYPOINT ["/usr/local/bin/traefik"]
