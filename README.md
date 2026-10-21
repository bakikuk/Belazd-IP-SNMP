# BWMON — per-IP bandwidth SNMP agent

Standalone **SNMP Counter64** source for **MRTG**, Zabbix, Cacti, and similar pollers. Counts bytes in/out per IPv4 host or CIDR on a Linux gateway (packet capture), exposes a configurable enterprise OID tree, optional web UI.

**History and graphs live on the poller** (MRTG RRD, etc.), not in this process.

**Documentation:** [guides/README.md](guides/README.md) (Linux install, systemd, MRTG, Zabbix)

## Quick start (lab)

```bash
cp config.example.yaml config.yaml
cp .env.example .env
# .env = web login only; edit SNMP / HTTP allowed in the UI or config.yaml
./scripts/build.sh --install-deps   # first time: installs golang-go + libpcap-dev
./scripts/build.sh                # later builds
./dist/Belazd-IP-SNMP -simulate -config config.yaml
```

Windows: `.\scripts\build.ps1` (Npcap + CGO). See below.

SNMP walk (with matching community + source IP, or lab flags):

```bash
snmpwalk -v2c -c mrtgSecret 127.0.0.1:1161 .1.3.6.1.4.1.55555
```

Use `-insecure-snmp-open` / `-insecure-http-open` only for local testing.

## `.env` (login only)

BWMON loads **`.env`** by default (`-env .env`; use `-env ""` to disable). Use it only for the web UI account:

- **`BWMON_HTTP_USER`**
- **`BWMON_HTTP_PASSWORD`**

SNMP communities, **`http.allowed`**, capture, OIDs, and queues are configured in **Settings** (saved to **`config.yaml`**). Login **session signing** uses **`http.session_secret`** in `config.yaml` (auto-created on first start when login is enabled; you can rotate it in Settings). Shell environment variables win over `.env`. Omit both login variables for IP-only access (no sign-in screen).

## Counter semantics

- **bytesIn**: traffic with destination in the target (host or CIDR)
- **bytesOut**: traffic with source in the target

Counters are **bytes** (L3/IP length). MRTG/Zabbix usually apply **×8** for **bit/s** graphs. Web UI shows both **kbit/s** (Δbytes×8÷Δt) and **KB/s** (Δbytes÷Δt).

### Queue target expressions

| Form | Example | Meaning |
|------|---------|---------|
| **Include** | `10.0.0.1,203.0.113.0/24` | Count traffic to/from **any** listed IPv4/CIDR (one SNMP index). |
| **Exclude** | `!10.0.0.50,!192.168.1.0/24` | Count **all other** IPv4 on `capture.interface` except listed entries (wide capture — more CPU). |
| **Interface** | `@eth0` / `if:eth0` | **Kernel NIC** `rx_bytes` / `tx_bytes` (bpytop/nload style). |

Do **not** mix include and exclude in one target. The web UI **Add Target** builder writes these strings for you.

### Per-IP vs total interface (bpytop / nload)

| Target | What it measures |
|--------|------------------|
| Single IP or include list | L3 bytes in **captured** packets matching those IPs (gateway/mirror) |
| `0.0.0.0/0` in include list | Still **pcap**, not NIC totals; usually **≠** terminal Mbps |
| `@eth0` or `if:eth0` | **Kernel NIC counters** — aligns with bpytop/nload on that interface |

Use the same interface name as in `capture.interface` when comparing to terminal tools.

## Security

- **Strict startup**: non-empty `snmp.communities[].allowed` and `http.allowed` required unless insecure flags.
- SNMP: community + **source IP** must match an entry (silent drop otherwise).
- HTTP: same allow-list; no login — restrict with `http.allowed` and optionally **ufw**.

## Ubuntu 24 deploy

See [guides/linux-requirements.md](guides/linux-requirements.md) for full package and permission details.

```bash
sudo apt install libpcap0.8t64
sudo install -m 755 dist/Belazd-IP-SNMP /usr/local/bin/Belazd-IP-SNMP
sudo mkdir -p /etc/bwmon
sudo cp config.example.yaml /etc/bwmon/config.yaml
# edit /etc/bwmon/config.yaml
sudo cp guides/bwmon.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now bwmon
```

Capabilities in unit: `CAP_NET_BIND_SERVICE`, `CAP_NET_RAW` for `:161` and capture.

**Manual binary (no systemd):** packet capture needs raw socket access:

```bash
sudo setcap cap_net_raw,cap_net_bind_service=+ep /usr/local/bin/Belazd-IP-SNMP
./Belazd-IP-SNMP -config /etc/bwmon/config.yaml
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
