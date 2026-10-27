package config

import (
	"encoding/json"
	"strings"
)

// FillDefaultsIfEmpty sets any blank OID suffix field from Default().
func (s *OIDSuffix) FillDefaultsIfEmpty() {
	d := Default().SNMP.OIDSuffix
	if strings.TrimSpace(s.QueueCount) == "" {
		s.QueueCount = d.QueueCount
	}
	if strings.TrimSpace(s.TargetName) == "" {
		s.TargetName = d.TargetName
	}
	if strings.TrimSpace(s.BytesIn) == "" {
		s.BytesIn = d.BytesIn
	}
	if strings.TrimSpace(s.BytesOut) == "" {
		s.BytesOut = d.BytesOut
	}
}

// MergeNonEmpty copies non-empty fields from other.
func (s *OIDSuffix) MergeNonEmpty(other OIDSuffix) {
	if strings.TrimSpace(other.QueueCount) != "" {
		s.QueueCount = other.QueueCount
	}
	if strings.TrimSpace(other.TargetName) != "" {
		s.TargetName = other.TargetName
	}
	if strings.TrimSpace(other.BytesIn) != "" {
		s.BytesIn = other.BytesIn
	}
	if strings.TrimSpace(other.BytesOut) != "" {
		s.BytesOut = other.BytesOut
	}
}

// DecodeOIDSuffixJSON reads oid_suffix from settings JSON (snake_case or legacy PascalCase).
func DecodeOIDSuffixJSON(raw json.RawMessage) OIDSuffix {
	if len(raw) == 0 {
		return OIDSuffix{}
	}
	var s OIDSuffix
	if err := json.Unmarshal(raw, &s); err == nil && s.anyNonEmpty() {
		return s
	}
	var leg struct {
		QueueCount string `json:"QueueCount"`
		TargetName string `json:"TargetName"`
		BytesIn    string `json:"BytesIn"`
		BytesOut   string `json:"BytesOut"`
	}
	if err := json.Unmarshal(raw, &leg); err != nil {
		return OIDSuffix{}
	}
	return OIDSuffix{
		QueueCount: leg.QueueCount,
		TargetName: leg.TargetName,
		BytesIn:    leg.BytesIn,
		BytesOut:   leg.BytesOut,
	}
}

func (s OIDSuffix) anyNonEmpty() bool {
	return strings.TrimSpace(s.QueueCount) != "" ||
		strings.TrimSpace(s.TargetName) != "" ||
		strings.TrimSpace(s.BytesIn) != "" ||
		strings.TrimSpace(s.BytesOut) != ""
}

// RepairSNMPFromCoreJSON fixes oid_suffix after loading core settings (legacy DB rows).
func RepairSNMPFromCoreJSON(snmp *SNMPConfig, coreJSON []byte) {
	if snmp == nil || len(coreJSON) == 0 {
		return
	}
	var probe struct {
		SNMP struct {
			OIDSuffix json.RawMessage `json:"oid_suffix"`
		} `json:"snmp"`
	}
	if err := json.Unmarshal(coreJSON, &probe); err != nil {
		snmp.OIDSuffix.FillDefaultsIfEmpty()
		return
	}
	decoded := DecodeOIDSuffixJSON(probe.SNMP.OIDSuffix)
	snmp.OIDSuffix.MergeNonEmpty(decoded)
	snmp.OIDSuffix.FillDefaultsIfEmpty()
}
