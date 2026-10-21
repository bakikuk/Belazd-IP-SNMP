#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
BIN_NAME="Belazd-IP-SNMP"
if [[ ! -f "${ROOT}/go.mod" ]]; then
  echo "error: go.mod not found in ${ROOT} — run this from the full repo (scripts/ is inside the project)." >&2
  exit 1
fi
if [[ "${1:-}" == "--install-deps" ]]; then
  sudo apt-get update
  sudo apt-get install -y golang-go libpcap-dev
  shift
fi
if ! command -v go >/dev/null 2>&1; then
  echo "error: 'go' not found. On Ubuntu 24 run:" >&2
  echo "  ./scripts/build.sh --install-deps" >&2
  echo "Or: sudo apt-get install -y golang-go libpcap-dev" >&2
  exit 1
fi
go mod download
go test ./...
mkdir -p dist
COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo none)"
CGO_ENABLED=1 go build -ldflags "-X main.version=0.1.0 -X main.commit=${COMMIT}" -o "dist/${BIN_NAME}" ./cmd/bwmon
echo "Built dist/${BIN_NAME}"
echo "Smoke: ./dist/${BIN_NAME} -simulate -insecure-snmp-open -insecure-http-open -config config.example.yaml"
