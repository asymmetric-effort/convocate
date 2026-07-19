# PostgreSQL — multi-stage build
# Build stage: ubuntu:24.04 (installs PG, creates entrypoint)
# Runtime stage: distroless (debian13 debug for busybox shell)
#
# Strategy: install PG from apt inside ubuntu, then copy the ENTIRE
# /usr tree needed at runtime. This avoids glibc mismatch and dynamic
# linker issues because we bring all libs from the same build.

FROM ghcr.io/asymmetric-effort/convocate/ubuntu-base:latest AS build

ARG PG_VERSION=17

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        gnupg && \
    echo "deb http://apt.postgresql.org/pub/repos/apt noble-pgdg main" \
        > /etc/apt/sources.list.d/pgdg.list && \
    curl -fsSL https://www.postgresql.org/media/keys/ACCC4CF8.asc \
        | gpg --dearmor -o /etc/apt/trusted.gpg.d/pgdg.gpg && \
    apt-get update && \
    apt-get install -y --no-install-recommends \
        "postgresql-${PG_VERSION}" && \
    rm -rf /var/lib/apt/lists/*

# Stage the PG installation and ALL its library dependencies
RUN mkdir -p /pg-root/usr/lib /pg-root/usr/share /pg-root/lib /pg-root/lib64 && \
    cp -a /usr/lib/postgresql /pg-root/usr/lib/ && \
    cp -a /usr/share/postgresql /pg-root/usr/share/ && \
    cp -a /usr/lib/x86_64-linux-gnu /pg-root/usr/lib/ && \
    cp -a /lib/x86_64-linux-gnu /pg-root/lib/ && \
    cp -a /lib64/ld-linux-x86-64.so.2 /pg-root/lib64/ && \
    cp -r /usr/share/zoneinfo /pg-root/usr/share/ && \
    cp -r /etc/ssl /pg-root/etc/ssl 2>/dev/null || true && \
    mkdir -p /pg-root/etc && \
    echo "postgres:x:65534:65534:PostgreSQL:/var/lib/postgresql:/bin/sh" > /pg-root/etc/passwd && \
    echo "postgres:x:65534:" > /pg-root/etc/group && \
    mkdir -p /pg-root/var/lib/postgresql/data /pg-root/var/run/postgresql /pg-root/tmp && \
    chown -R 65534:65534 /pg-root/var/lib/postgresql /pg-root/var/run/postgresql /pg-root/tmp

# Create the entrypoint
# When POSTGRES_PASSWORD is set, use scram-sha-256 authentication.
# Otherwise fall back to trust (for local development).
RUN PG_BIN=/usr/lib/postgresql/${PG_VERSION}/bin && \
    printf '#!/bin/sh\nPGDATA="${PGDATA:-/var/lib/postgresql/data}"\nPG_BIN=%s\nif [ ! -s "$PGDATA/PG_VERSION" ]; then\n  if [ -n "$POSTGRES_PASSWORD" ]; then\n    echo "$POSTGRES_PASSWORD" > /tmp/pwfile\n    $PG_BIN/initdb -D "$PGDATA" --auth=scram-sha-256 --pwfile=/tmp/pwfile --no-instructions --no-locale || true\n    echo "host all all 0.0.0.0/0 scram-sha-256" >> "$PGDATA/pg_hba.conf"\n  else\n    $PG_BIN/initdb -D "$PGDATA" --auth=trust --no-instructions --no-locale || true\n    echo "host all all 0.0.0.0/0 trust" >> "$PGDATA/pg_hba.conf"\n  fi\n  echo "password_encryption = '"'"'scram-sha-256'"'"'" >> "$PGDATA/postgresql.conf"\nfi\nexec $PG_BIN/postgres -D "$PGDATA" -c listen_addresses='"'"'*'"'"'\n' \
    "$PG_BIN" > /pg-root/docker-entrypoint.sh && \
    chmod +x /pg-root/docker-entrypoint.sh

# Runtime stage — use scratch+ubuntu libs instead of distroless to avoid glibc mismatch
FROM scratch

COPY --from=build /pg-root/ /
COPY --from=build /bin/sh /bin/sh
COPY --from=build /usr/bin/id /usr/bin/id

ENV PGDATA=/var/lib/postgresql/data

EXPOSE 5432

USER 65534:65534

ENTRYPOINT ["/bin/sh", "/docker-entrypoint.sh"]
