#!/usr/bin/env bash
# i18n audit for the Canteiro monorepo (backend locales; web if present).
set -euo pipefail

ROOT="${1:-.}"
FAILS=0
REQUIRED_LOCALES=(en pt-BR es)

audit_dir() {
  local dir="$1"
  echo "🔎 i18n audit: ${dir}"
  local missing=()
  local loc
  for loc in "${REQUIRED_LOCALES[@]}"; do
    if [ ! -f "${dir}/${loc}.json" ]; then
      missing+=("$loc")
    fi
  done
  if [ ${#missing[@]} -ne 0 ]; then
    echo "  ❌ missing: ${missing[*]}"
    FAILS=$((FAILS + 1))
    return
  fi
  echo "  ✅ locales present"
  if ! command -v jq >/dev/null 2>&1; then
    echo "  ⚠️  jq missing; skip key parity"
    return
  fi
  local ref_keys loc_keys
  ref_keys=$(jq -r 'keys[]' "${dir}/en.json" | sort)
  for loc in "${REQUIRED_LOCALES[@]}"; do
    [ "$loc" = "en" ] && continue
    loc_keys=$(jq -r 'keys[]' "${dir}/${loc}.json" | sort)
    if [ "$ref_keys" != "$loc_keys" ]; then
      echo "  ❌ ${loc}.json keys differ from en.json"
      FAILS=$((FAILS + 1))
    else
      echo "  ✅ ${loc}.json parity"
    fi
  done
}

FOUND=0
if [ -d "${ROOT}/backend/internal/i18n/locales" ]; then
  audit_dir "${ROOT}/backend/internal/i18n/locales"
  FOUND=1
fi
if [ -d "${ROOT}/web/i18n/locales" ]; then
  audit_dir "${ROOT}/web/i18n/locales"
  FOUND=1
fi
if [ -d "${ROOT}/internal/i18n/locales" ]; then
  audit_dir "${ROOT}/internal/i18n/locales"
  FOUND=1
fi

if [ "$FOUND" -eq 0 ]; then
  echo "❌ no locales directory found"
  exit 1
fi

if [ "$FAILS" -gt 0 ]; then
  echo "❌ ${FAILS} i18n check(s) failed"
  exit 1
fi
echo "✅ Auditoria i18n OK."
