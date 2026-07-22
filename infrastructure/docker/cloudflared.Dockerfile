# Cloudflared tunnel agent — mirror of upstream image
# We mirror to GHCR to avoid Docker Hub rate limits and maintain
# consistency with our container image policy.
#
# The cloudflared binary is a statically-linked Go binary, so we
# extract it from the upstream image into our ubuntu base.

ARG UBUNTU_BASE_TAG=latest
FROM cloudflare/cloudflared:latest AS upstream

FROM ghcr.io/asymmetric-effort/convocate/ubuntu-base:${UBUNTU_BASE_TAG}

COPY --from=upstream /usr/local/bin/cloudflared /usr/local/bin/cloudflared

USER 65534:65534

ENTRYPOINT ["/usr/local/bin/cloudflared"]
