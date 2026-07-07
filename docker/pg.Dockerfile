# PostgreSQL — multi-stage build
# Build stage: ubuntu:24.04 (installs PG, creates entrypoint)
# Runtime stage: distroless (debian13 debug for busybox shell)
#
# Strategy: install PG from apt inside ubuntu, then copy the ENTIRE
# /usr tree needed at runtime. This avoids glibc mismatch and dynamic
# linker issues because we bring all libs from the same build.

FROM ubuntu:24.04 AS build

ARG PG_VERSION=17

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        ca-certificates \
        curl \
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
    mkdir -p /pg-root/var/lib/postgresql/data /pg-root/var/run/postgresql && \
    chown -R 65534:65534 /pg-root/var/lib/postgresql /pg-root/var/run/postgresql

# Create the entrypoint
RUN PG_BIN=/usr/lib/postgresql/${PG_VERSION}/bin && \
    printf '#!/bin/sh\nset -e\nPGDATA="${PGDATA:-/var/lib/postgresql/data}"\nif [ ! -s "$PGDATA/PG_VERSION" ]; then\n  %s/initdb -D "$PGDATA" --auth=trust --no-instructions --no-locale\n  echo "host all all 0.0.0.0/0 trust" >> "$PGDATA/pg_hba.conf"\n  echo "listen_addresses = '"'"'*'"'"'" >> "$PGDATA/postgresql.conf"\nfi\nexec %s/postgres -D "$PGDATA"\n' \
    "$PG_BIN" "$PG_BIN" > /pg-root/docker-entrypoint.sh && \
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
