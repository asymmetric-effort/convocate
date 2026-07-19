#!/usr/bin/env bash
set -euo pipefail

IMAGE="${1:?Usage: $0 <image>}"

# Static validation — verify all expected tools exist
docker run --rm "$IMAGE" go version
docker run --rm "$IMAGE" node --version
docker run --rm "$IMAGE" bun --version
docker run --rm "$IMAGE" docker --version
docker run --rm "$IMAGE" kubectl version --client
docker run --rm "$IMAGE" helm version --short
docker run --rm "$IMAGE" ansible --version
docker run --rm "$IMAGE" /usr/local/bin/leakdetector --version
docker run --rm "$IMAGE" npx playwright --version
docker run --rm "$IMAGE" git --version
docker run --rm "$IMAGE" make --version
docker run --rm "$IMAGE" jq --version

echo "PASS: $(basename "$0")"
