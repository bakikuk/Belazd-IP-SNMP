#Requirement Linux
sudo apt-get install -y libpcap0.8


#Register to daemon service
sudo nano /etc/systemd/system/belazd-ip-snmp.service
sudo systemctl daemon-reload


#/etc/systemd/system/belazd-ip-snmp.service
[Unit]
Description=Belazd-IP-SNMP per-IP bandwidth SNMP agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=/var/www/apps/ip-snmp

ExecStart=/var/www/apps/ip-snmp/Belazd-IP-SNMP

Restart=on-failure
RestartSec=5

# Capture + SNMP on UDP/161 without running a full root shell long-term
AmbientCapabilities=CAP_NET_BIND_SERVICE CAP_NET_RAW
CapabilityBoundingSet=CAP_NET_BIND_SERVICE CAP_NET_RAW
NoNewPrivileges=true

LimitNOFILE=65535

[Install]
WantedBy=multi-user.target



