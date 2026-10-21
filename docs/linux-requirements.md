# Linux requirements (BWMON)

BWMON runs on a **Linux gateway or router** that sees the traffic you want to count (typically the machine that forwards customer/NAT traffic). **Ubuntu 24.04 LTS** is the primary target; other recent **Debian/Ubuntu** releases usually work with the same package names.

---

## Summary

| Phase | What you need |
|--------|----------------|
| **Run the binary** | `libpcap` runtime library, correct network interface name, **CAP_NET_RAW** (+ **CAP_NET_BIND_SERVICE** if SNMP uses port 161) |
| **Build from source** | Go toolchain, **libpcap development headers**, C compiler (CGO) |
| **Optional** | `systemd`, `.env` for web login, MRTG/Zabbix on **another** host (not on this box) |

---

## Runtime (production server)

Install on the machine where you deploy `bwmon` (not needed on your laptop unless you run it there).

### Required packages

**Debian / Ubuntu**

```bash
sudo apt-get update
sudo apt-get install -y libpcap0.8
```

On some releases the package is named `libpcap0.8t64` or similar; `apt-cache search libpcap0` shows the exact name. Ubuntu 24.04 often lists **`libpcap0.8t64`** or **`libpcap0.25`** depending on archive — install whichever `apt` offers for “libpcap runtime”.

Nothing else is required at runtime if you copy a **pre-built** `dist/bwmon` binary from another machine (same OS/arch, e.g. `linux/amd64`).

### Kernel / OS

- **Linux** with normal networking (`/sys/class/net/<iface>/statistics/` for `@eth0`-style queues).
- **IPv4** forwarding/gateway role if you count customer traffic passing through the box.
- Interface name in `config.yaml` → `capture.interface` must exist (`ip link`).

### Permissions (capture + SNMP)

| Capability | Why |
|------------|-----|
| **CAP_NET_RAW** | Packet capture (libpcap) |
| **CAP_NET_BIND_SERVICE** | Bind SNMP to **UDP 161** (privileged port) |

**Recommended:** run under **systemd** with `AmbientCapabilities` (see `deploy/bwmon.service`).

**Alternative:** file capabilities on the binary:

```bash
sudo setcap cap_net_raw,cap_net_bind_service=+ep /usr/local/bin/bwmon
```

**Lab only:** `sudo ./bwmon ...` or `-simulate` (no capture, counters fake).

Without **CAP_NET_RAW**, capture fails at startup unless you use **`-simulate`**.

### Ports and firewall

| Service | Default in example config | Notes |
|---------|---------------------------|--------|
| SNMP | `0.0.0.0:161` | Change in Settings if 161 is in use |
| Web UI | `0.0.0.0:8080` | Restrict with `http.allowed` in config |

If you use **ufw**, allow your admin subnet and poller IPs, for example:

```bash
sudo ufw allow from 192.168.10.0/24 to any port 8080 proto tcp
sudo ufw allow from 10.0.0.50 to any port 161 proto udp
```

### Config files on the server

| File | Purpose |
|------|---------|
| `/etc/bwmon/config.yaml` (or local `config.yaml`) | SNMP, HTTP ACL, queues, capture — **Settings** in UI |
| `.env` next to binary or config | **Optional:** `BWMON_HTTP_USER`, `BWMON_HTTP_PASSWORD` only |

Do not commit real `config.yaml` or `.env` to git (see `.gitignore`).

---

## Build from source (on the gateway or a build VM)

Same architecture as the deploy target (usually **amd64**).

### Required packages (Ubuntu / Debian)

```bash
sudo apt-get update
sudo apt-get install -y golang-go libpcap-dev build-essential git
```

Or use the project script (installs Go + libpcap dev; add `build-essential` if `go build` fails with a C compiler error):

```bash
./scripts/build.sh --install-deps
sudo apt-get install -y build-essential   # if CGO complains about gcc
./scripts/build.sh
```

Output: **`dist/bwmon`**.

### Build notes

- **CGO_ENABLED=1** is required for libpcap capture (default in `scripts/build.sh`).
- **Go 1.21+** (module declares 1.26; distro `golang-go` on Ubuntu 24 is sufficient for this project).

---

## Optional (helpful, not required)

| Tool | Use |
|------|-----|
| **systemd** | Service install: `deploy/bwmon.service` |
| **snmp** / **snmpwalk** | Test OIDs: `sudo apt install snmp` |
| **iproute2** | `ip link`, `ip addr` — pick `capture.interface` |
| **MRTG / Zabbix / Cacti** | Graphing — runs on **monitoring servers**, not bundled with BWMON |

---

## Not required on the Linux gateway

- Docker (unless you choose to containerize yourself)
- Database, Redis, nginx (BWMON serves its own small web UI)
- Node.js, Python runtime
- RRDtool / graphing (done on the poller)

---

## Quick checklist before `systemctl start bwmon`

1. [ ] `libpcap` runtime installed  
2. [ ] `bwmon` binary in place (e.g. `/usr/local/bin/bwmon`)  
3. [ ] `config.yaml` edited: `capture.interface`, `snmp.communities`, `http.allowed`, queues  
4. [ ] Optional `.env` for web login  
5. [ ] Capabilities or systemd unit with **CAP_NET_RAW** / **CAP_NET_BIND_SERVICE**  
6. [ ] Firewall allows poller → SNMP and admins → HTTP  
7. [ ] Smoke test: `snmpwalk -v2c -c <community> <host>:161 .1.3.6.1.4.1.<PEN>` from an allowed source IP  

---

## Other Linux families (short)

| Distro | Runtime | Build |
|--------|---------|--------|
| **RHEL / Rocky / Alma** | `libpcap` (`yum install libpcap`) | `libpcap-devel`, `golang`, `gcc` |
| **Alpine** | `libpcap` | `libpcap-dev`, `go`, `build-base` |

Use the same capability / systemd ideas as on Ubuntu.
