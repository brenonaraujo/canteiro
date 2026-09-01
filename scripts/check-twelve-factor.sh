#!/usr/bin/env bash
# 12-factor audit for the Canteiro monorepo (Go API under backend/).
set -euo pipefail

ROOT="${1:-.}"
FAILS=0

echo "🔎 12-Factor audit: ${ROOT}"
echo

echo -n "I. Codebase ... "
if [ -d "${ROOT}/.git" ]; then echo "✅"; else echo "❌ no .git"; FAILS=$((FAILS + 1)); fi

echo -n "II. Dependencies ... "
if [ -f "${ROOT}/backend/go.mod" ] && [ -f "${ROOT}/backend/go.sum" ]; then
  echo "✅"
else
  echo "❌ missing backend/go.mod or go.sum"
  FAILS=$((FAILS + 1))
fi

echo -n "III. Config (no hardcoded) ... "
if grep -rE "(DB_URL|DATABASE_URL|API_KEY|secret_key|password)\s*[:=]\s*[\"'][^\"']+[\"']" \
     "${ROOT}/backend/internal" "${ROOT}/backend/cmd" 2>/dev/null \
     | grep -vE "_test\.go" >/dev/null; then
  echo "❌ hardcoded config"
  FAILS=$((FAILS + 1))
else
  echo "✅"
fi

echo -n "IV. Backing services via env ... "
if grep -rE "DATABASE_URL" "${ROOT}/backend/internal" 2>/dev/null >/dev/null; then
  echo "✅"
else
  echo "❌ DATABASE_URL not referenced"
  FAILS=$((FAILS + 1))
fi

echo -n "V. Dockerfile presente ... "
if [ -f "${ROOT}/deploy/Dockerfile.backend" ]; then
  echo "✅"
else
  echo "❌ missing deploy/Dockerfile.backend"
  FAILS=$((FAILS + 1))
fi

echo -n "VI. Stateless ... "
if grep -rE "sync\.Map|session\[" "${ROOT}/backend/internal" 2>/dev/null \
   | grep -vE "_test\.go" >/dev/null; then
  echo "⚠️  (check local state)"
else
  echo "✅"
fi

echo -n "VII. PORT env ... "
if grep -rE "envconfig:\"PORT\"|os\.Getenv\(\"PORT\"\)" \
   "${ROOT}/backend/internal" "${ROOT}/backend/cmd" 2>/dev/null >/dev/null; then
  echo "✅"
else
  echo "❌ PORT env not detected"
  FAILS=$((FAILS + 1))
fi

echo -n "VIII. Concurrency ... "
echo "ℹ️  (DoD)"

echo -n "IX. Graceful shutdown ... "
if grep -rE "signal\.NotifyContext|syscall\.SIGTERM" \
   "${ROOT}/backend/cmd" 2>/dev/null >/dev/null; then
  echo "✅"
else
  echo "❌ no SIGTERM handler"
  FAILS=$((FAILS + 1))
fi

echo -n "X. Dev/prod parity ... "
if [ -f "${ROOT}/scripts/dev.sh" ] && [ -f "${ROOT}/scripts/prod.sh" ]; then
  echo "⚠️  split scripts"
else
  echo "✅"
fi

echo -n "XI. Logs em stdout JSON ... "
if grep -rE "slog\.NewJSONHandler\(os\.Stdout" "${ROOT}/backend/internal" 2>/dev/null >/dev/null; then
  echo "✅"
else
  echo "❌ slog JSON stdout not detected"
  FAILS=$((FAILS + 1))
fi

echo -n "XII. Admin processes (migrate one-shot) ... "
if grep -qE "migrate/migrate" "${ROOT}/docker-compose.yml" 2>/dev/null \
   || [ -d "${ROOT}/backend/cmd/migrate" ]; then
  echo "✅"
else
  echo "❌ migrate one-off not found"
  FAILS=$((FAILS + 1))
fi

echo
if [ "$FAILS" -gt 0 ]; then
  echo "❌ ${FAILS} fator(es) falharam."
  exit 1
fi
echo "✅ Todos os fatores auditados."
