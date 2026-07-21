# Ubuntu base image for all Convocate build stages
# Pre-installs common packages, applies CIS hardening

FROM ubuntu:26.04

# Update packages and install base dependencies
RUN apt-get update && \
    apt-get upgrade -y && \
    apt-get install -y --no-install-recommends \
        apt-transport-https \
        ca-certificates \
        curl && \
    rm -rf /var/lib/apt/lists/*

# CIS Docker Benchmark hardening
# 4.1 — Create non-root user
RUN groupadd -r appuser && useradd -r -g appuser -s /usr/sbin/nologin appuser

# 4.6 — Remove setuid/setgid bits from executables
RUN find / -perm /6000 -type f -exec chmod a-s {} + 2>/dev/null || true

# 4.8 — Remove unnecessary packages and shells
RUN rm -f /usr/bin/passwd /usr/sbin/adduser 2>/dev/null || true

# 4.9 — Remove package manager cache
RUN rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

# CIS 5.x — filesystem hardening
RUN chmod 700 /root && \
    chmod 1777 /tmp
