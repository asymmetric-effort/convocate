#!/bin/bash
# Generate a TOTP code for pdv-test user from OpenBao seed.
#
# Usage: ./tools/pdv-totp.sh [openbao-addr] [openbao-token]
# Outputs: TOTP code on stdout
#
# This is a placeholder. The actual TOTP generation requires the seed and
# crypto primitives, which will be implemented when the clusters are live.
# For now it documents the interface that the CD pipeline and PDV tests
# will consume.

set -euo pipefail

OPENBAO_ADDR="${1:-${OPENBAO_ADDR:-https://openbao.convocate.svc:8200}}"
OPENBAO_TOKEN="${2:-${OPENBAO_TOKEN:-}}"

if [ -z "$OPENBAO_TOKEN" ]; then
  echo "ERROR: OpenBao token required (arg 2 or OPENBAO_TOKEN env)" >&2
  exit 1
fi

# Read the TOTP seed from OpenBao
SEED=$(curl -sf \
  -H "X-Vault-Token: ${OPENBAO_TOKEN}" \
  "${OPENBAO_ADDR}/v1/totp/code/pdv-test" \
  | jq -r '.data.code')

if [ -z "$SEED" ] || [ "$SEED" = "null" ]; then
  echo "ERROR: Failed to read TOTP code from OpenBao" >&2
  exit 1
fi

echo "$SEED"
