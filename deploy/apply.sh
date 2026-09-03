#!/usr/bin/env bash
# Deploy Canteiro on brenon.cloud home lab
# Usage: bash deploy/apply.sh
set -euo pipefail

STACK_NAME=canteiro
SWARM_MANAGER=192.168.1.101
KONG_ADMIN="http://$SWARM_MANAGER:8001"
IMAGE="ghcr.io/brenonaraujo/canteiro:latest"
ENV_FILE="deploy/swarm/.env"

echo "=== 1. Deploy stack ==="
docker -H "$SWARM_MANAGER" stack deploy -c deploy/swarm/stack.yml "$STACK_NAME"
echo "✅ Stack deployed. Waiting for services..."
sleep 10

echo "=== 2. Kong: create service ==="
curl -sS -X POST "$KONG_ADMIN/services" -H 'Content-Type: application/json' -d '{
  "name": "canteiro",
  "url": "http://canteiro:8080",
  "connect_timeout": 10000,
  "read_timeout": 60000,
  "write_timeout": 60000,
  "tags": ["backend","canteiro"]
}' 2>/dev/null || echo "Kong service may already exist"

echo "=== 3. Kong: create route ==="
curl -sS -X POST "$KONG_ADMIN/services/canteiro/routes" -H 'Content-Type: application/json' -d '{
  "name": "canteiro-path",
  "paths": ["/canteiro"],
  "strip_path": true,
  "protocols": ["http","https"],
  "tags": ["canteiro"]
}' 2>/dev/null || echo "Kong route may already exist"

echo "=== 4. Health check ==="
sleep 5
curl -sS "http://$SWARM_MANAGER:8000/canteiro/healthz" 2>/dev/null && echo "✅ Health OK" || echo "⚠️ Health check failed"

echo ""
echo "=== Next steps ==="
echo "1. Add tunnel ingress via Cloudflare Zero Trust UI"
echo "2. Add DNS CNAME for canteiro.brenon.cloud"
echo "3. Test: curl https://canteiro.brenon.cloud/healthz"