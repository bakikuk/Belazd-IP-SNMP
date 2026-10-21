# Linux daemon (systemd)

Example files to run **Beladz-IP-SNMP** as a **systemd** service on Ubuntu/Debian (gateway host).

| File | Purpose |
|------|---------|
| [Beladz-IP-SNMP.service](Beladz-IP-SNMP.service) | systemd unit — capabilities, paths, `.env` login |
| [Beladz-IP-SNMP.env.example](Beladz-IP-SNMP.env.example) | Copy to `/etc/Beladz-IP-SNMP/.env` (web login only) |

Application config (SNMP, queues, HTTP ACL) lives in **`/etc/Beladz-IP-SNMP/config.yaml`** — edit via web UI or by hand. Use [../config.example.yaml](../config.example.yaml) as a template.

## Install layout

```text
/usr/local/bin/Beladz-IP-SNMP # binary from dist/Beladz-IP-SNMP
/etc/Beladz-IP-SNMP/config.yaml        # main config (not in git)
/etc/Beladz-IP-SNMP/.env               # BELADZ_HTTP_USER / BELADZ_HTTP_PASSWORD (optional)
/etc/systemd/system/Beladz-IP-SNMP.service
```

## Quick install

```bash
sudo apt-get install -y libpcap0.8t64
sudo install -m 755 dist/Beladz-IP-SNMP /usr/local/bin/Beladz-IP-SNMP
sudo mkdir -p /etc/Beladz-IP-SNMP
sudo install -m 600 config.example.yaml /etc/Beladz-IP-SNMP/config.yaml
sudo install -m 600 guides/Beladz-IP-SNMP.env.example /etc/Beladz-IP-SNMP/.env
# edit /etc/Beladz-IP-SNMP/config.yaml and /etc/Beladz-IP-SNMP/.env
sudo install -m 644 guides/Beladz-IP-SNMP.service /etc/systemd/system/Beladz-IP-SNMP.service
sudo systemctl daemon-reload
sudo systemctl enable --now Beladz-IP-SNMP
sudo systemctl status Beladz-IP-SNMP
```

## Useful commands

```bash
sudo journalctl -u Beladz-IP-SNMP -f
sudo systemctl restart Beladz-IP-SNMP
```

After changing **`config.yaml`** from the UI, a **restart** is only required if listen addresses or capture interface changed; queue/OID updates reload in-process.

## Without systemd

```bash
sudo setcap cap_net_raw,cap_net_bind_service=+ep /usr/local/bin/Beladz-IP-SNMP
cd /etc/Beladz-IP-SNMP && /usr/local/bin/Beladz-IP-SNMP -config /etc/Beladz-IP-SNMP/config.yaml -env /etc/Beladz-IP-SNMP/.env
```

See also [linux-requirements.md](linux-requirements.md).
