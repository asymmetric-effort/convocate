# Grafana — multi-stage build
# Build stage: ubuntu:24.04 (download binary + gather shared libs)
# Runtime stage: distroless

ARG UBUNTU_BASE_TAG=latest
FROM ghcr.io/asymmetric-effort/convocate/ubuntu-base:${UBUNTU_BASE_TAG} AS build

ARG DEPS_URL

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        libfontconfig1 && \
    rm -rf /var/lib/apt/lists/*

RUN curl -fsSL "${DEPS_URL}/grafana-13.1.0-linux-amd64.tar.gz" \
        -o /tmp/grafana.tar.gz && \
    tar xzf /tmp/grafana.tar.gz -C /opt && \
    mv /opt/grafana-13.1.0 /opt/grafana && \
    rm /tmp/grafana.tar.gz

# Collect fontconfig shared libs for distroless runtime
RUN mkdir -p /opt/libs && \
    cp /usr/lib/x86_64-linux-gnu/libfontconfig.so.1* /opt/libs/ && \
    cp /usr/lib/x86_64-linux-gnu/libfreetype.so.6* /opt/libs/ && \
    cp /usr/lib/x86_64-linux-gnu/libpng16.so.16* /opt/libs/ && \
    cp /usr/lib/x86_64-linux-gnu/libexpat.so.1* /opt/libs/ && \
    cp /usr/lib/x86_64-linux-gnu/libbrotlidec.so.1* /opt/libs/ && \
    cp /usr/lib/x86_64-linux-gnu/libbrotlicommon.so.1* /opt/libs/ && \
    cp /usr/lib/x86_64-linux-gnu/libz.so.1* /opt/libs/

# Copy fontconfig config
RUN mkdir -p /opt/fontconfig && \
    cp -r /etc/fonts /opt/fontconfig/

# Create data directories and default config
RUN mkdir -p /opt/grafana-data /opt/grafana-log /opt/grafana-config && \
    cp /opt/grafana/conf/defaults.ini /opt/grafana-config/grafana.ini

# Runtime stage
FROM gcr.io/distroless/cc-debian13:debug

COPY --from=build /opt/grafana /opt/grafana
COPY --from=build /opt/libs/ /usr/lib/x86_64-linux-gnu/
COPY --from=build /opt/fontconfig/fonts /etc/fonts
COPY --from=build --chown=65534:65534 /opt/grafana-data /var/lib/grafana
COPY --from=build --chown=65534:65534 /opt/grafana-log /var/log/grafana
COPY --from=build /opt/grafana-config/grafana.ini /etc/grafana/grafana.ini

EXPOSE 3000

USER 65534:65534

WORKDIR /opt/grafana

ENTRYPOINT ["/opt/grafana/bin/grafana", "server", \
    "--homepath=/opt/grafana", \
    "--config=/etc/grafana/grafana.ini"]
