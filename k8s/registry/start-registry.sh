#!/usr/bin/env bash
# Start the private container registry on this host (192.168.3.90).
# Persistence: Docker named volume (convocate-registry-data)
# Port: 5000 (TLS)
set -euo pipefail

REGISTRY_NAME="convocate-registry"
REGISTRY_PORT="5000"
REGISTRY_ADDR="192.168.3.90"
IMAGE="convocate-registry:local"
VOLUME="convocate-registry-data"

# Build the registry image
echo "Building registry image..."
docker build -f docker/registry.Dockerfile -t "$IMAGE" .

# Stop existing registry if running
docker rm -f "$REGISTRY_NAME" 2>/dev/null || true

# Create volume if needed
docker volume create "$VOLUME" 2>/dev/null || true

# Start the registry (as root for volume write access)
echo "Starting registry on ${REGISTRY_ADDR}:${REGISTRY_PORT} (TLS)..."
docker run -d \
    --name "$REGISTRY_NAME" \
    --restart=always \
    --user root \
    -p "${REGISTRY_PORT}:5000" \
    -v "${VOLUME}:/var/lib/registry" \
    "$IMAGE"

echo "Registry running at ${REGISTRY_ADDR}:${REGISTRY_PORT}"

# Tag and push the registry image to itself (bootstrap)
docker tag "$IMAGE" "${REGISTRY_ADDR}:${REGISTRY_PORT}/convocate/registry:latest"
docker push "${REGISTRY_ADDR}:${REGISTRY_PORT}/convocate/registry:latest"

echo "Registry bootstrapped — image pushed to itself."
echo "Catalog: $(curl -sk https://${REGISTRY_ADDR}:${REGISTRY_PORT}/v2/_catalog)"
