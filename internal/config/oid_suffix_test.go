package config

import (
	"encoding/json"
	"testing"
)

func TestDecodeOIDSuffixJSONLegacyPascalCase(t *testing.T) {
	raw := json.RawMessage(`{"QueueCount":".1.1.1.0","TargetName":".1.1.2.1.1.2","BytesIn":".1.1.2.1.1.8","BytesOut":".1.1.2.1.1.9"}`)
	got := DecodeOIDSuffixJSON(raw)
	if got.QueueCount != ".1.1.1.0" || got.BytesOut != ".1.1.2.1.1.9" {
		t.Fatalf("decode=%+v", got)
	}
}

func TestFillDefaultsIfEmpty(t *testing.T) {
	var s OIDSuffix
	s.FillDefaultsIfEmpty()
	d := Default().SNMP.OIDSuffix
	if s.QueueCount != d.QueueCount {
		t.Fatalf("queue_count=%q want %q", s.QueueCount, d.QueueCount)
	}
}

func TestRepairSNMPFromCoreJSON(t *testing.T) {
	core := []byte(`{"snmp":{"listen":":1161","pen":55555,"oid_suffix":{"QueueCount":".1.1.1.0","TargetName":".1.1.2.1.1.2","BytesIn":".1.1.2.1.1.8","BytesOut":".1.1.2.1.1.9"}}}`)
	snmp := SNMPConfig{Listen: ":1161", PEN: 55555}
	RepairSNMPFromCoreJSON(&snmp, core)
	if snmp.OIDSuffix.QueueCount != ".1.1.1.0" {
		t.Fatalf("suffix=%+v", snmp.OIDSuffix)
	}
}
