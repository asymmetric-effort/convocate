# Fluent Bit — single stage using ubuntu runtime
# Fluent Bit has many shared library dependencies that make
# distroless impractical — use ubuntu:24.04 as the runtime.

FROM 192.168.3.90:5000/convocate/ubuntu-base:latest

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        gnupg && \
    curl -fsSL https://packages.fluentbit.io/fluentbit.key | gpg --dearmor -o /usr/share/keyrings/fluentbit-keyring.gpg && \
    echo "deb [signed-by=/usr/share/keyrings/fluentbit-keyring.gpg] https://packages.fluentbit.io/ubuntu/noble noble main" \
        > /etc/apt/sources.list.d/fluent-bit.list && \
    apt-get update && \
    apt-get install -y --no-install-recommends fluent-bit && \
    apt-get purge -y gnupg && \
    apt-get autoremove -y && \
    rm -rf /var/lib/apt/lists/*

USER nobody:nogroup

ENTRYPOINT ["/opt/fluent-bit/bin/fluent-bit"]
CMD ["-c", "/fluent-bit/etc/fluent-bit.conf"]
