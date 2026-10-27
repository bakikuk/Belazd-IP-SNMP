# Linux requirements (Belazd-IP-SNMP)

Belazd-IP-SNMP runs on a **Linux gateway or router** that sees the traffic you want to count (typically the machine that forwards customer/NAT traffic). **Ubuntu 24.04 LTS** is the primary target; other recent **Debian/Ubuntu** releases usually work with the same package names.

---

## Summary

| Phase | What you need |
|--------|----------------|
| **Run the binary** | `libpcap` runtime library, correct network interface name, **CAP_NET_RAW** (+ **CAP_NET_BIND_SERVICE** if SNMP uses port 161) |
| **Build from source** | Go toolchain, **libpcap development headers**, C compiler (CGO) |
| **Optional** | `systemd`, MRTG/Zabbix on **another** host (not on this box) |

---

## Runtime (production server)

Install on the machine where you deploy Belazd-IP-SNMP (not needed on your laptop unless you run it there).

### Required packages

**Debian / Ubuntu**

```bash
sudo apt-get update
sudo apt-get install -y libpcap0.8
```

On some releases the package is named `libpcap0.8t64` or similar; `apt-cache search libpcap0` shows the exact name. Ubuntu 24.04 often lists **`libpcap0.8t64`** or **`libpcap0.25`** depending on archive — install whichever `apt` offers for “libpcap runtime”.

Nothing else is required at runtime if you copy a **pre-built** `dist/Belazd-IP-SNMP` binary from another machine (same OS/arch, e.g. `linux/amd64`).

### Kernel / OS

- **Linux** with normal networking (`/sys/class/net/<iface>/statistics/` for `@eth0`-style queues).
- **IPv4** forwarding/gateway role if you count customer traffic passing through the box.
- Interface name in Settings → `capture.interface` must exist (`ip link`).

### Permissions (capture + SNMP)

| Capability | Why |
|------------|-----|
| **CAP_NET_RAW** | Packet capture (libpcap) |
| **CAP_NET_BIND_SERVICE** | Bind SNMP to **UDP 161** (privileged port) |

**Recommended:** run under **systemd** with `AmbientCapabilities` (see [systemd-daemon.md](systemd-daemon.md) and [Belazd-IP-SNMP.service](Belazd-IP-SNMP.service)).

**Alternative:** file capabilities on the binary:

```bash
sudo setcap cap_net_raw,cap_net_bind_service=+ep /usr/local/bin/Belazd-IP-SNMP
```

**Lab only:** `sudo ./Belazd-IP-SNMP ...` or `-simulate` (no capture, counters fake).

Without **CAP_NET_RAW**, capture fails at startup unless you use **`-simulate`**.

### Ports and firewall

| Service | Default in example config | Notes |
|---------|---------------------------|--------|
| SNMP | `0.0.0.0:161` | Change in Settings if 161 is in use |
| Web UI | `0.0.0.0:8051` (fixed) | Restrict with `http.allowed` in Settings |

If you use **ufw**, allow your admin subnet and poller IPs, for example:

```bash
sudo ufw allow from 192.168.10.0/24 to any port 8051 proto tcp
sudo ufw allow from 10.0.0.50 to any port 161 proto udp
```

### Config files on the server

| File | Purpose |
|------|---------|
| `belazd.sqlite` (beside binary) | All settings, users, queues — Web UI |
| `config.yaml` | **Legacy migration only** — imported once into SQLite |

Do not commit `belazd.sqlite` to git (see `.gitignore`).

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

Output: **`dist/Belazd-IP-SNMP`**.

### Build notes

- **CGO_ENABLED=1** is required for libpcap capture (default in `scripts/build.sh`).
- **Go 1.21+** (module declares 1.26; distro `golang-go` on Ubuntu 24 is sufficient for this project).

---

## Optional (helpful, not required)

| Tool | Use |
|------|-----|
| **systemd** | [systemd-daemon.md](systemd-daemon.md), [Belazd-IP-SNMP.service](Belazd-IP-SNMP.service) |
| **snmp** / **snmpwalk** | Test OIDs: `sudo apt install snmp` — see [mrtg-snmp.md](mrtg-snmp.md) |
| **iproute2** | `ip link`, `ip addr` — pick `capture.interface` |
| **MRTG / Zabbix / Cacti** | [mrtg-snmp.md](mrtg-snmp.md), [zabbix-items.md](zabbix-items.md) |

---

## Not required on the Linux gateway

- Docker (unless you choose to containerize yourself)
- Database, Redis, nginx (Belazd-IP-SNMP serves its own small web UI)
- Node.js, Python runtime
- RRDtool / graphing (done on the poller)

---

## Quick checklist before `systemctl start Belazd-IP-SNMP`

1. [ ] `libpcap` runtime installed  
2. [ ] `Belazd-IP-SNMP` binary in place (e.g. `/usr/local/bin/Belazd-IP-SNMP`)  
3. [ ] `belazd.sqlite` created (first start) or legacy `config.yaml` for one-time import  
4. [ ] First admin: baked in at **build** from `.env`, or create under **Users** on the VM  
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
