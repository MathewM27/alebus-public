#!/usr/bin/env bash
# Docker Safety Check for Alebus Dev Stack
# Prevents port collisions and topology conflicts before starting dev stack.

set -e

echo "🔍 Docker Safety Check"
echo ""

# ============================================================================
# Check 1: Redis Topology Conflict
# ============================================================================
STANDALONE=$(docker ps --filter "name=alebus-redis-emqx" --format "{{.Names}}" 2>/dev/null | wc -l)
SENTINEL=$(docker ps --filter "name=alebus-redis-master" --format "{{.Names}}" 2>/dev/null | wc -l)

if [[ $STANDALONE -gt 0 && $SENTINEL -gt 0 ]]; then
  echo "❌ ERROR: Both standalone and sentinel Redis stacks are running!"
  echo "   This causes port collisions and runtime ambiguity."
  echo ""
  echo "   Active Redis containers:"
  docker ps --filter "name=redis" --format "  - {{.Names}} ({{.Ports}})"
  echo ""
  echo "   Action: Stop one stack before starting dev stack:"
  echo "   docker compose -f compose/legacy/docker-compose.sentinel.yml down"
  exit 1
fi

# ============================================================================
# Check 2: Prometheus Port Collision (9090)
# ============================================================================
PROMETHEUS=$(docker ps --filter "name=prometheus" --filter "publish=9090" --format "{{.Names}}" 2>/dev/null | wc -l)
if [[ $PROMETHEUS -gt 0 ]]; then
  echo "❌ ERROR: Prometheus is running on port 9090!"
  echo "   This will collide with alebus-simulator (dev API)."
  echo ""
  echo "   Action: Stop observability stack first:"
  echo "   docker compose -f compose/legacy/docker-compose.observability.yml down"
  echo ""
  exit 1
fi

# ============================================================================
# Check 3: Observability Stack Running
# ============================================================================
OBSERVABILITY=$(docker ps --filter "name=alebus-observability" --format "{{.Names}}" 2>/dev/null | wc -l)
if [[ $OBSERVABILITY -gt 0 ]]; then
  echo "⚠️  WARNING: Observability stack is running."
  echo "   This may cause port conflicts (Prometheus on 9090)."
  echo ""
  echo "   Recommendation: Stop observability stack:"
  echo "   docker compose -f compose/legacy/docker-compose.observability.yml down"
  echo ""
  # Don't exit, just warn
fi

# ============================================================================
# Check 4: Port Availability (Best-Effort)
# ============================================================================
echo "ℹ️  Port availability check:"
echo "   Required ports: 5432 (postgres), 6382 (redis), 1883/18083 (emqx), 9090 (simulator), 8080 (api), 9100 (ingestor)"
echo ""

# Cross-platform port checking is difficult. Provide guidance instead.
if command -v lsof &> /dev/null; then
  # Linux/Mac with lsof
  echo "   Checking with lsof..."
  for port in 5432 6382 1883 18083 9090 8080 9100; do
    if lsof -i :$port &> /dev/null; then
      echo "   ⚠️  Port $port is in use"
    fi
  done
elif command -v netstat &> /dev/null; then
  # Windows or Linux with netstat
  echo "   Use 'netstat -ano | findstr :<port>' (Windows) or 'netstat -tuln | grep :<port>' (Linux) to check ports manually."
else
  echo "   Manual check recommended: netstat -ano | findstr :9090 (Windows) or lsof -i :9090 (Linux/Mac)"
fi

echo ""
echo "✅ Safety checks passed."
exit 0
