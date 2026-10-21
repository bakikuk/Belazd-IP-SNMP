# Zabbix SNMP items

1. Add host with SNMP interface (v2c, community from config).
2. Ensure Zabbix poller IP is in `snmp.communities[].allowed`.
3. Create items:
   - **SNMP agent**, OID `.1.3.6.1.4.1.55555.1.1.2.1.1.8.N` (bytes in)
   - **SNMP agent**, OID `.1.3.6.1.4.1.55555.1.1.2.1.1.9.N` (bytes out)
4. Preprocessing: **Change per second**, custom multiplier `8` for bits/s if needed.

Use web UI **02 SNMP / OIDs** for exact paths when PEN/suffixes change.
