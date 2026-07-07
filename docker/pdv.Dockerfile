# Convocate Post-Deployment Verification
# Uses distroless debug (busybox) for lightweight API+UI testing

FROM gcr.io/distroless/cc-debian13:debug

COPY test/pdv/run-tests.sh /tests/run-tests.sh

USER 65534:65534

ENTRYPOINT ["/busybox/sh", "/tests/run-tests.sh"]
