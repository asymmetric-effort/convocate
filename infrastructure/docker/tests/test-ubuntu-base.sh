#!/usr/bin/env bash
set -euo pipefail

IMAGE="${1:?Usage: $0 <image>}"
CONTAINER_NAME="test-ubuntu-base-$$"

cleanup() { docker rm -f "$CONTAINER_NAME" 2>/dev/null || true; }
trap cleanup EXIT

echo "Testing ubuntu-base image: $IMAGE"

# Verify ca-certificates exist
echo "  Checking ca-certificates..."
docker run --rm --name "${CONTAINER_NAME}-ca" "$IMAGE" \
    ls /etc/ssl/certs/ca-certificates.crt >/dev/null
echo "    ca-certificates: OK"

# Verify curl is available
echo "  Checking curl..."
docker run --rm --name "${CONTAINER_NAME}-curl" "$IMAGE" \
    curl --version >/dev/null
echo "    curl: OK"

# Verify apt-transport-https is installed
echo "  Checking apt-transport-https..."
docker run --rm --name "${CONTAINER_NAME}-apt" "$IMAGE" \
    dpkg -s apt-transport-https >/dev/null 2>&1
echo "    apt-transport-https: OK"

echo "PASS: $(basename "$0")"
