# Grafana — multi-stage build
# Build stage: ubuntu:24.04 (download binary)
# Runtime stage: ubuntu:24.04 (Grafana needs many shared libs)

FROM ubuntu:24.04 AS build

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        ca-certificates \
        curl && \
    rm -rf /var/lib/apt/lists/*

RUN curl -fsSL https://dl.grafana.com/oss/release/grafana-13.1.0.linux-amd64.tar.gz \
        -o /tmp/grafana.tar.gz && \
    tar xzf /tmp/grafana.tar.gz -C /opt && \
    mv /opt/grafana-13.1.0 /opt/grafana && \
    rm /tmp/grafana.tar.gz

# Runtime stage
FROM ubuntu:24.04

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        ca-certificates \
        libfontconfig1 && \
    rm -rf /var/lib/apt/lists/*

COPY --from=build /opt/grafana /opt/grafana

RUN mkdir -p /var/lib/grafana /var/log/grafana && \
    chown -R 65534:65534 /opt/grafana /var/lib/grafana /var/log/grafana

EXPOSE 3000

USER 65534:65534

WORKDIR /opt/grafana

ENTRYPOINT ["/opt/grafana/bin/grafana", "server", \
    "--homepath=/opt/grafana", \
    "--config=/etc/grafana/grafana.ini"]
