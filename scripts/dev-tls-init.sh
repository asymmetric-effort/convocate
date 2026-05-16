#!/bin/sh
# Generates TLS certificates for the local dev stack.
# Runs as an init container — exits after writing certs to /tls/.
set -eu

CERT_DIR="/tls"
CA_CERT="$CERT_DIR/ca.crt"
CA_KEY="$CERT_DIR/ca.key"

# Skip if certs already exist.
if [ -f "$CA_CERT" ] && [ -f "$CERT_DIR/redis.crt" ]; then
  echo "dev-tls-init: certs already exist, skipping."
  exit 0
fi

echo "dev-tls-init: generating self-signed CA and service certs..."

# Generate CA key and cert.
openssl ecparam -genkey -name prime256v1 -noout -out "$CA_KEY" 2>/dev/null
openssl req -new -x509 -key "$CA_KEY" -sha256 \
  -subj "/CN=convocate-dev-ca/O=convocate" \
  -days 3650 -out "$CA_CERT" 2>/dev/null

# Generate Redis cert.
openssl ecparam -genkey -name prime256v1 -noout \
  -out "$CERT_DIR/redis.key" 2>/dev/null
openssl req -new -key "$CERT_DIR/redis.key" \
  -subj "/CN=redis/O=convocate" \
  -out /tmp/redis.csr 2>/dev/null
cat > /tmp/redis-ext.cnf << 'EXTEOF'
subjectAltName=DNS:redis,DNS:localhost,IP:127.0.0.1
EXTEOF
openssl x509 -req -in /tmp/redis.csr -CA "$CA_CERT" -CAkey "$CA_KEY" \
  -CAcreateserial -days 3650 -sha256 \
  -extfile /tmp/redis-ext.cnf \
  -out "$CERT_DIR/redis.crt" 2>/dev/null

# Generate OpenBao cert.
openssl ecparam -genkey -name prime256v1 -noout \
  -out "$CERT_DIR/openbao.key" 2>/dev/null
openssl req -new -key "$CERT_DIR/openbao.key" \
  -subj "/CN=openbao/O=convocate" \
  -out /tmp/openbao.csr 2>/dev/null
cat > /tmp/openbao-ext.cnf << 'EXTEOF'
subjectAltName=DNS:openbao,DNS:localhost,IP:127.0.0.1
EXTEOF
openssl x509 -req -in /tmp/openbao.csr -CA "$CA_CERT" -CAkey "$CA_KEY" \
  -CAcreateserial -days 3650 -sha256 \
  -extfile /tmp/openbao-ext.cnf \
  -out "$CERT_DIR/openbao.crt" 2>/dev/null

# Generate Router cert.
openssl ecparam -genkey -name prime256v1 -noout \
  -out "$CERT_DIR/router.key" 2>/dev/null
openssl req -new -key "$CERT_DIR/router.key" \
  -subj "/CN=router/O=convocate" \
  -out /tmp/router.csr 2>/dev/null
cat > /tmp/router-ext.cnf << 'EXTEOF'
subjectAltName=DNS:router,DNS:localhost,IP:127.0.0.1
EXTEOF
openssl x509 -req -in /tmp/router.csr -CA "$CA_CERT" -CAkey "$CA_KEY" \
  -CAcreateserial -days 3650 -sha256 \
  -extfile /tmp/router-ext.cnf \
  -out "$CERT_DIR/router.crt" 2>/dev/null

# Generate Dispatch client cert.
openssl ecparam -genkey -name prime256v1 -noout \
  -out "$CERT_DIR/dispatch.key" 2>/dev/null
openssl req -new -key "$CERT_DIR/dispatch.key" \
  -subj "/CN=dev-host-1/O=convocate" \
  -out /tmp/dispatch.csr 2>/dev/null
openssl x509 -req -in /tmp/dispatch.csr -CA "$CA_CERT" -CAkey "$CA_KEY" \
  -CAcreateserial -days 3650 -sha256 \
  -out "$CERT_DIR/dispatch.crt" 2>/dev/null

# Make keys readable by service containers.
chmod 644 "$CERT_DIR"/*.key "$CERT_DIR"/*.crt

echo "dev-tls-init: done. Generated:"
ls -la "$CERT_DIR"
