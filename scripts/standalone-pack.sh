#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
./scripts/build.sh
PACK=dist/belazd-ip-snmp-linux-amd64
rm -rf "$PACK"
mkdir -p "$PACK"
cp dist/Belazd-IP-SNMP "$PACK/"
cp deploy/belazd-ip-snmp.service "$PACK/"
cat > "$PACK/INSTALL.txt" << 'EOF'
Belazd-IP-SNMP standalone (Linux amd64)

1. sudo apt-get install -y libpcap0.8
2. sudo mkdir -p /opt/belazd-ip-snmp
3. sudo cp Belazd-IP-SNMP /opt/belazd-ip-snmp/
4. sudo cp belazd-ip-snmp.service /etc/systemd/system/
5. sudo systemctl daemon-reload && sudo systemctl enable --now belazd-ip-snmp
6. Web UI: http://<host>:8051 (first admin was embedded at build from .env if you used it; else add user under Users). Tighten http.allowed in Settings.

Database: belazd.sqlite created beside the binary (/opt/belazd-ip-snmp/belazd.sqlite).
One-time migration: place config.yaml next to binary before first start.
EOF
tar -czf dist/belazd-ip-snmp-linux-amd64.tar.gz -C dist belazd-ip-snmp-linux-amd64
echo "Packed dist/belazd-ip-snmp-linux-amd64.tar.gz"
