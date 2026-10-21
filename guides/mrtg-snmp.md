# MRTG integration

## Sanity check

```bash
snmpwalk -v2c -c YOUR_COMMUNITY GATEWAY_IP:161 .1.3.6.1.4.1.55555.1.1.2
```

Community and source IP must match `snmp.communities` allow-lists in config.

## Example cfg fragment

```
Target[bwmon-customer]: 2:YOUR_COMMUNITY@GATEWAY_IP:.1.3.6.1.4.1.55555.1.1.2.1.1.8.1:.1.3.6.1.4.1.55555.1.1.2.1.1.9.1
Options[bwmon-customer]: growright,bits
Title[bwmon-customer]: Customer queue 1
```

Replace `.8.1` / `.9.1` with OIDs from the web UI **Queues** table for each queue index.

Counters are **bytes** (Counter64); MRTG `derive` computes rate between polls.
