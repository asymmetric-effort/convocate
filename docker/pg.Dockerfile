# PostgreSQL — multi-stage build
# Build stage: ubuntu:24.04
# Runtime stage: distroless

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

# Collect binaries and required shared libraries
RUN mkdir -p /opt/pg/bin /opt/pg/lib /opt/pg/share && \
    cp /usr/lib/postgresql/${PG_VERSION}/bin/postgres \
       /usr/lib/postgresql/${PG_VERSION}/bin/pg_isready \
       /usr/lib/postgresql/${PG_VERSION}/bin/initdb \
       /usr/lib/postgresql/${PG_VERSION}/bin/pg_ctl \
       /usr/lib/postgresql/${PG_VERSION}/bin/psql \
       /opt/pg/bin/ && \
    cp -r /usr/lib/postgresql/${PG_VERSION}/lib/* /opt/pg/lib/ && \
    cp -r /usr/share/postgresql/${PG_VERSION}/* /opt/pg/share/

# Collect shared library dependencies
RUN mkdir -p /opt/pg/deps && \
    for bin in /opt/pg/bin/*; do \
        ldd "$bin" 2>/dev/null | grep "=>" | awk '{print $3}' | \
        while read lib; do \
            [ -f "$lib" ] && cp -n "$lib" /opt/pg/deps/ || true; \
        done; \
    done && \
    for lib in /opt/pg/lib/*.so*; do \
        ldd "$lib" 2>/dev/null | grep "=>" | awk '{print $3}' | \
        while read dep; do \
            [ -f "$dep" ] && cp -n "$dep" /opt/pg/deps/ || true; \
        done; \
    done

# Collect locale and timezone data needed by PostgreSQL
RUN mkdir -p /opt/pg/locale /opt/pg/zoneinfo && \
    cp -r /usr/lib/locale/* /opt/pg/locale/ 2>/dev/null || true && \
    cp -r /usr/share/zoneinfo/* /opt/pg/zoneinfo/ 2>/dev/null || true

# Create the init script
RUN echo '#!/bin/sh' > /opt/pg/bin/docker-entrypoint.sh && \
    echo 'set -e' >> /opt/pg/bin/docker-entrypoint.sh && \
    echo 'export LD_LIBRARY_PATH=/usr/local/lib/pg:/usr/local/lib/pg/deps' >> /opt/pg/bin/docker-entrypoint.sh && \
    echo 'PGDATA="${PGDATA:-/var/lib/postgresql/data}"' >> /opt/pg/bin/docker-entrypoint.sh && \
    echo 'if [ ! -s "$PGDATA/PG_VERSION" ]; then' >> /opt/pg/bin/docker-entrypoint.sh && \
    echo '  /usr/local/bin/pg/initdb -D "$PGDATA" --auth=trust --no-instructions' >> /opt/pg/bin/docker-entrypoint.sh && \
    echo '  echo "host all all 0.0.0.0/0 trust" >> "$PGDATA/pg_hba.conf"' >> /opt/pg/bin/docker-entrypoint.sh && \
    echo '  echo "listen_addresses = '"'"'*'"'"'" >> "$PGDATA/postgresql.conf"' >> /opt/pg/bin/docker-entrypoint.sh && \
    echo 'fi' >> /opt/pg/bin/docker-entrypoint.sh && \
    echo 'exec /usr/local/bin/pg/postgres -D "$PGDATA"' >> /opt/pg/bin/docker-entrypoint.sh && \
    chmod +x /opt/pg/bin/docker-entrypoint.sh

# Runtime stage
FROM gcr.io/distroless/cc-debian12:debug

COPY --from=build /opt/pg/bin/ /usr/local/bin/pg/
COPY --from=build /opt/pg/lib/ /usr/local/lib/pg/
COPY --from=build /opt/pg/share/ /usr/local/share/pg/
COPY --from=build /opt/pg/deps/ /usr/local/lib/pg/deps/
COPY --from=build /opt/pg/locale/ /usr/lib/locale/
COPY --from=build /opt/pg/zoneinfo/ /usr/share/zoneinfo/

ENV LD_LIBRARY_PATH=/usr/local/lib/pg:/usr/local/lib/pg/deps
ENV PGDATA=/var/lib/postgresql/data

EXPOSE 5432

ENTRYPOINT ["/busybox/sh", "/usr/local/bin/pg/docker-entrypoint.sh"]
