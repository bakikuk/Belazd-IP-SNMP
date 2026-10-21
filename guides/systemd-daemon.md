# Linux daemon (systemd)

Example files to run **Belazd-IP-SNMP** as a **systemd** service on Ubuntu/Debian (gateway host).

| File | Purpose |
|------|---------|
| [Belazd-IP-SNMP.service](Belazd-IP-SNMP.service) | systemd unit — capabilities, paths, `.env` login |
| [Belazd-IP-SNMP.env.example](Belazd-IP-SNMP.env.example) | Copy to `/etc/Belazd-IP-SNMP/.env` (web login only) |

Application config (SNMP, queues, HTTP ACL) lives in **`/etc/Belazd-IP-SNMP/config.yaml`** — edit via web UI or by hand. Use [../config.example.yaml](../config.example.yaml) as a template.

## Install layout

```text
/usr/local/bin/Belazd-IP-SNMP # binary from dist/Belazd-IP-SNMP
/etc/Belazd-IP-SNMP/config.yaml        # main config (not in git)
/etc/Belazd-IP-SNMP/.env               # BELAZD_HTTP_USER / BELAZD_HTTP_PASSWORD (optional)
/etc/systemd/system/Belazd-IP-SNMP.service
```

## Quick install

```bash
sudo apt-get install -y libpcap0.8t64
sudo install -m 755 dist/Belazd-IP-SNMP /usr/local/bin/Belazd-IP-SNMP
sudo mkdir -p /etc/Belazd-IP-SNMP
sudo install -m 600 config.example.yaml /etc/Belazd-IP-SNMP/config.yaml
sudo install -m 600 guides/Belazd-IP-SNMP.env.example /etc/Belazd-IP-SNMP/.env
# edit /etc/Belazd-IP-SNMP/config.yaml and /etc/Belazd-IP-SNMP/.env
sudo install -m 644 guides/Belazd-IP-SNMP.service /etc/systemd/system/Belazd-IP-SNMP.service
sudo systemctl daemon-reload
sudo systemctl enable --now Belazd-IP-SNMP
sudo systemctl status Belazd-IP-SNMP
```

## Useful commands

```bash
sudo journalctl -u Belazd-IP-SNMP -f
sudo systemctl restart Belazd-IP-SNMP
```

After changing **`config.yaml`** from the UI, a **restart** is only required if listen addresses or capture interface changed; queue/OID updates reload in-process.

## Without systemd

```bash
sudo setcap cap_net_raw,cap_net_bind_service=+ep /usr/local/bin/Belazd-IP-SNMP
cd /etc/Belazd-IP-SNMP && /usr/local/bin/Belazd-IP-SNMP -config /etc/Belazd-IP-SNMP/config.yaml -env /etc/Belazd-IP-SNMP/.env
```

See also [linux-requirements.md](linux-requirements.md).
