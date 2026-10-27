# Linux daemon (systemd)

Example files to run **Belazd-IP-SNMP** as a **systemd** service on Ubuntu/Debian (gateway host).

| File | Purpose |
|------|---------|
| [../deploy/belazd-ip-snmp.service](../deploy/belazd-ip-snmp.service) | systemd unit — capabilities, SQLite path |

Application state lives in **`belazd.sqlite`** next to the binary (created automatically). Web users and passwords are stored there only.

## Install layout

```text
/opt/belazd-ip-snmp/Belazd-IP-SNMP   # binary
/opt/belazd-ip-snmp/belazd.sqlite    # database (auto-created)
/etc/systemd/system/belazd-ip-snmp.service
```

## Quick install

```bash
sudo apt-get install -y libpcap0.8
sudo mkdir -p /opt/belazd-ip-snmp
sudo cp dist/Belazd-IP-SNMP /opt/belazd-ip-snmp/
sudo cp deploy/belazd-ip-snmp.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now belazd-ip-snmp
sudo systemctl status belazd-ip-snmp
```

Web UI: **http://&lt;host&gt;:8051** — on first run, open **Users** to create the first account (Administrators group is pre-created).

## Useful commands

```bash
sudo journalctl -u belazd-ip-snmp -f
sudo systemctl restart belazd-ip-snmp
```

Queue/OID updates reload in-process. Restart if **SNMP listen** or **capture interface** changed in Settings.

## Without systemd

```bash
sudo setcap cap_net_raw,cap_net_bind_service=+ep ./Belazd-IP-SNMP
./Belazd-IP-SNMP
```

See also [linux-requirements.md](linux-requirements.md).
