# Linux daemon (systemd)

Example files to run **bwmon** as a **systemd** service on Ubuntu/Debian (gateway host).

| File | Purpose |
|------|---------|
| [bwmon.service](bwmon.service) | systemd unit — capabilities, paths, `.env` login |
| [bwmon.env.example](bwmon.env.example) | Copy to `/etc/bwmon/.env` (web login only) |

Application config (SNMP, queues, HTTP ACL) lives in **`/etc/bwmon/config.yaml`** — edit via web UI or by hand. Use [../config.example.yaml](../config.example.yaml) as a template.

## Install layout

```text
/usr/local/bin/bwmon          # binary from dist/bwmon
/etc/bwmon/config.yaml        # main config (not in git)
/etc/bwmon/.env               # BWMON_HTTP_USER / BWMON_HTTP_PASSWORD (optional)
/etc/systemd/system/bwmon.service
```

## Quick install

```bash
sudo apt-get install -y libpcap0.8t64
sudo install -m 755 dist/bwmon /usr/local/bin/bwmon
sudo mkdir -p /etc/bwmon
sudo install -m 600 config.example.yaml /etc/bwmon/config.yaml
sudo install -m 600 guides/bwmon.env.example /etc/bwmon/.env
# edit /etc/bwmon/config.yaml and /etc/bwmon/.env
sudo install -m 644 guides/bwmon.service /etc/systemd/system/bwmon.service
sudo systemctl daemon-reload
sudo systemctl enable --now bwmon
sudo systemctl status bwmon
```

## Useful commands

```bash
sudo journalctl -u bwmon -f
sudo systemctl restart bwmon
```

After changing **`config.yaml`** from the UI, a **restart** is only required if listen addresses or capture interface changed; queue/OID updates reload in-process.

## Without systemd

```bash
sudo setcap cap_net_raw,cap_net_bind_service=+ep /usr/local/bin/bwmon
cd /etc/bwmon && /usr/local/bin/bwmon -config /etc/bwmon/config.yaml -env /etc/bwmon/.env
```

See also [linux-requirements.md](linux-requirements.md).
