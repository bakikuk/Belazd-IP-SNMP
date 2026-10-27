#!/usr/bin/env bash
# Reads BELAZD_HTTP_USER / BELAZD_HTTP_PASSWORD from .env for link-time embed only.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
ENV_FILE="${1:-${ROOT}/.env}"
PKG="github.com/bakikuk/belazd-ip-snmp/internal/buildseed"
user=""
pass=""
if [[ -f "$ENV_FILE" ]]; then
  while IFS= read -r line || [[ -n "$line" ]]; do
    line="${line%%#*}"
    line="$(echo "$line" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
    [[ -z "$line" ]] && continue
    case "$line" in
      BELAZD_HTTP_USER=*) user="${line#BELAZD_HTTP_USER=}" ;;
      BELAZD_HTTP_PASSWORD=*) pass="${line#BELAZD_HTTP_PASSWORD=}" ;;
    esac
  done < "$ENV_FILE"
  user="${user%\"}"; user="${user#\"}"
  pass="${pass%\"}"; pass="${pass#\"}"
fi
if [[ -n "$user" && -n "$pass" ]]; then
  ub64="$(printf '%s' "$user" | base64 | tr -d '\n')"
  pb64="$(printf '%s' "$pass" | base64 | tr -d '\n')"
  printf '%s' "-X ${PKG}.AdminUserB64=${ub64} -X ${PKG}.AdminPassB64=${pb64}"
else
  echo "build: no BELAZD_HTTP_USER/PASSWORD in ${ENV_FILE} — first admin via Users UI or add .env before build" >&2
fi
