# Belazd-IP-SNMP — per-IP bandwidth SNMP agent

Standalone **SNMP Counter64** source for **MRTG**, Zabbix, Cacti, and similar pollers. Counts bytes in/out per IPv4 host or CIDR on a Linux gateway (packet capture), exposes a configurable enterprise OID tree, optional web UI.

**History and graphs live on the poller** (MRTG RRD, etc.), not in this process.

**Documentation:** [guides/README.md](guides/README.md) (Linux install, systemd, MRTG, Zabbix)

## Quick start (lab)

```bash
cp .env.example .env   # optional: first admin embedded at **build** (not used at runtime)
./scripts/build.sh --install-deps   # first time: installs golang-go + libpcap-dev
./scripts/build.sh
./dist/Belazd-IP-SNMP -simulate -insecure-snmp-open -insecure-http-open
# creates belazd.sqlite beside the binary; Web UI http://127.0.0.1:8051
```

Windows: `.\scripts\build.ps1` (Npcap + CGO). See below.

SNMP walk (with matching community + source IP, or lab flags):

```bash
snmpwalk -v2c -c mrtgSecret 127.0.0.1:1161 .1.3.6.1.4.1.55555
```

Use `-insecure-snmp-open` / `-insecure-http-open` only for local testing.

## Storage (SQLite)

All settings, queues, users, and groups live in **`belazd.sqlite`** (default: next to the `Belazd-IP-SNMP` binary). Override with **`-data /path/to/belazd.sqlite`**.

- **Web UI** is always **`http://<host>:8051`** (hardcoded).
- **First run** on an empty DB: open **`http.allowed`** (`0.0.0.0/0`) with a log warning — tighten in **Settings**.
- **One-time migration:** place legacy **`config.yaml`** beside the binary; first start imports it into SQLite and renames to **`config.yaml.bak`**.
- **Web login:** accounts live in SQLite. **Build** with `.env` (`BELAZD_HTTP_USER` / `BELAZD_HTTP_PASSWORD`) — those values are **embedded in the binary** and seed the first admin on first start only. The **running app never reads `.env`**. Without `.env` at build time, add the first user under **Users** (UI open until then).

## Counter semantics

- **bytesIn**: traffic with destination in the target (host or CIDR)
- **bytesOut**: traffic with source in the target

Counters are **bytes** (L3/IP length). MRTG/Zabbix usually apply **×8** for **bit/s** graphs. Web UI shows both **kbit/s** (Δbytes×8÷Δt) and **KB/s** (Δbytes÷Δt).

### Queue target expressions

| Form | Example | Meaning |
|------|---------|---------|
| **Include** | `10.0.0.1,203.0.113.0/24` | Count traffic to/from **any** listed IPv4/CIDR (one SNMP index). |
| **Exclude** | `!10.0.0.50,!192.168.1.0/24` | Count **all other** IPv4 on the queue’s **capture NIC** except listed entries (wide capture — more CPU). |
| **Interface** | `@eth0` / `if:eth0` | **Kernel NIC** `rx_bytes` / `tx_bytes` (bpytop/nload style). Uses the **target** name, not Settings capture list. |

Do **not** mix include and exclude in one target. The web UI **Add Target** builder writes these strings for you.

### Multi-NIC capture (Settings + per queue)

- **Settings → Capture interfaces**: one NIC name per line. **First line = default** for new Include/Exclude queues.
- **Add/Edit queue → Capture NIC**: which wire to sniff for that IP/exclude queue (must be in the Settings list).
- The app opens **one libpcap handle per NIC** that has at least one IP/exclude queue assigned.
- If a queue references a NIC no longer in Settings, it shows an error in the UI and is **skipped** until fixed (other queues keep working).

### Per-IP vs total interface (bpytop / nload)

| Target | What it measures |
|--------|------------------|
| Single IP or include list | L3 bytes in **captured** packets matching those IPs on the queue’s capture NIC |
| `0.0.0.0/0` in include list | Still **pcap**, not NIC totals; usually **≠** terminal Mbps |
| `@eth0` or `if:eth0` | **Kernel NIC counters** — aligns with bpytop/nload on that interface |

Use the same interface name as the queue’s capture NIC when comparing to `tcpdump` on that interface.

## Security

- **Strict startup**: non-empty `snmp.communities[].allowed` and `http.allowed` required unless insecure flags.
- SNMP: community + **source IP** must match an entry (silent drop otherwise).
- HTTP: same allow-list; optional login via SQLite users — restrict with `http.allowed` and optionally **ufw**.

## Ubuntu 24 deploy

See [guides/linux-requirements.md](guides/linux-requirements.md) for full package and permission details.

```bash
sudo apt install libpcap0.8t64
sudo mkdir -p /opt/belazd-ip-snmp
sudo install -m 755 dist/Belazd-IP-SNMP /opt/belazd-ip-snmp/
sudo cp deploy/belazd-ip-snmp.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now belazd-ip-snmp
```

Capabilities in unit: `CAP_NET_BIND_SERVICE`, `CAP_NET_RAW` for `:161` and capture.

**Manual binary (no systemd):** packet capture needs raw socket access:

```bash
sudo setcap cap_net_raw,cap_net_bind_service=+ep /usr/local/bin/Belazd-IP-SNMP
./Belazd-IP-SNMP
```

Or run once as root: `sudo ./Belazd-IP-SNMP ...` (not recommended long-term). Without caps, use `-simulate` for SNMP/UI testing only.

## MRTG / Zabbix

See [guides/mrtg-snmp.md](guides/mrtg-snmp.md) and [guides/zabbix-items.md](guides/zabbix-items.md).

## Windows / Npcap

Install [Npcap](https://npcap.com/) (Admin). Build with `CGO_ENABLED=1`. Default SNMP port `:1161` if not running as Admin.

## Resource notes

Main CPU cost is **packet capture**, not SNMP polling. Use narrow targets/BPF; ~600 Mbps on 10G NIC is typically fine with focused CIDRs.

## Build

| Platform | Command |
|----------|---------|
| Linux | `./scripts/build.sh` |
| Windows | `.\scripts\build.ps1` |

Output: `dist/Belazd-IP-SNMP` or `dist/Belazd-IP-SNMP.exe`.
