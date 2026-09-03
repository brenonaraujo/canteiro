#!/usr/bin/env bash
# Deploy / smoke Canteiro on brenon.cloud.
# Public origin is the Swarm edge on :18083 (tunnel already points here).
# Do NOT mutate Kong key-auth on /canteiro.
set -euo pipefail

STACK_NAME=canteiro
HOST="${CANTEIRO_PUBLIC_HOST:-https://canteiro.brenon.cloud}"
ENV_FILE="${ENV_FILE:-deploy/swarm/.env}"

echo "=== 1. Stack file ==="
echo "Portainer stack '${STACK_NAME}' (endpoint 3): paste deploy/swarm/stack.yml"
echo "Env: SERVICE_TAG (GHCR tag, default latest), EDGE_PUBLISHED_PORT=18083"
echo "Do not change Kong service/route/plugins for /canteiro."
echo ""

if [[ -n "${DOCKER_HOST:-}" ]] && command -v docker >/dev/null 2>&1; then
  echo "=== 1b. docker stack deploy (DOCKER_HOST=${DOCKER_HOST}) ==="
  extra=()
  if [[ -f "$ENV_FILE" ]]; then
    extra+=(--env-file "$ENV_FILE")
  fi
  docker stack deploy "${extra[@]}" -c deploy/swarm/stack.yml "$STACK_NAME"
  echo "stack deploy requested"
else
  echo "=== 1b. skipped docker stack deploy (no DOCKER_HOST) ==="
  echo "Update Portainer stack id 197 from this repo file after the web image exists on GHCR."
fi

echo ""
echo "=== 2. Tunnel / DNS (no-op if already set) ==="
echo "Ingress: canteiro.brenon.cloud → http://192.168.1.101:18083  (above *.brenon.cloud)"
echo "DNS:     CNAME canteiro → 5ea9935b-fac5-4161-a6b0-6c1afaf4bce3.cfargotunnel.com proxied"
echo "Do not CNAME to api.brenon.cloud."

echo ""
echo "=== 3. Public smoke ==="
echo "# HTML SPA (not Gin 404)"
echo "curl -sS -D- -o /tmp/canteiro.html ${HOST}/"
echo "grep -i canteiro /tmp/canteiro.html"
echo ""
echo "# API still on the same host"
echo "curl -sS ${HOST}/healthz"
echo "curl -sS -D- -o /tmp/canteiro-listings.json ${HOST}/listings"

echo ""
echo "Running public curls now:"
echo "--- GET / ---"
curl -sS -D- -o /tmp/canteiro.html "${HOST}/" | tr -d '\r' | awk 'NR<=16'
echo "body_head=$(head -c 200 /tmp/canteiro.html | tr '\n' ' ')"
echo ""
echo "--- GET /healthz ---"
curl -sS -D- -o /tmp/canteiro-healthz.json "${HOST}/healthz" | tr -d '\r' | awk 'NR<=16'
echo "body=$(cat /tmp/canteiro-healthz.json)"
echo ""
echo "--- GET /listings ---"
curl -sS -D- -o /tmp/canteiro-listings.json "${HOST}/listings" | tr -d '\r' | awk 'NR<=16'
echo "body=$(head -c 300 /tmp/canteiro-listings.json)"
echo ""
echo "Kong LAN is extra only (not DoD): curl -sS http://192.168.1.101:8000/canteiro/healthz"
