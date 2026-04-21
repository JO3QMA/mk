#!/bin/sh
# Generate self-signed certs for the two drop-in test instances (`a`, `b`).
# Mirrors tests/federation/common/gen-certs.sh but with different domains —
# keeping it separate avoids cross-suite coupling if the federation suite
# changes its domain set later.
set -e

CERT_DIR=/certs
mkdir -p "$CERT_DIR"

DOMAINS="a b"

for domain in $DOMAINS; do
  if [ ! -f "$CERT_DIR/$domain.crt" ]; then
    openssl req -x509 -newkey rsa:2048 -nodes \
      -keyout "$CERT_DIR/$domain.key" \
      -out "$CERT_DIR/$domain.crt" \
      -days 1 \
      -subj "/CN=$domain" \
      -addext "subjectAltName=DNS:$domain" \
      2>/dev/null
    echo "Generated cert for $domain"
  fi
done
