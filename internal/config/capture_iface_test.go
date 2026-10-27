package config

import "testing"

func TestNormalizeInterfacesLegacy(t *testing.T) {
	c := CaptureConfig{Interface: "ens34", Snaplen: 65535}
	c.NormalizeInterfaces()
	if len(c.Interfaces) != 1 || c.Interfaces[0] != "ens34" {
		t.Fatalf("interfaces=%v", c.Interfaces)
	}
}

func TestNormalizeInterfacesDedupe(t *testing.T) {
	c := CaptureConfig{Interfaces: []string{"ens34", "ens34", "ens35", ""}}
	c.NormalizeInterfaces()
	if len(c.Interfaces) != 2 || c.Interfaces[0] != "ens34" || c.Interfaces[1] != "ens35" {
		t.Fatalf("interfaces=%v", c.Interfaces)
	}
}

func TestGroupIPQueuesByCaptureIface(t *testing.T) {
	cfg := Default()
	cfg.Capture.Interfaces = []string{"ens34", "ens35"}
	cfg.Queues = []Queue{
		{Index: 1, Target: "10.0.0.1", CaptureIface: "ens34"},
		{Index: 2, Target: "10.0.0.2", CaptureIface: "ens35"},
		{Index: 3, Target: "@ens36"},
	}
	groups, skipped := cfg.GroupIPQueuesByCaptureIface()
	if len(skipped) != 0 {
		t.Fatalf("skipped=%v", skipped)
	}
	if len(groups["ens34"]) != 1 || len(groups["ens35"]) != 1 {
		t.Fatalf("groups=%v", groups)
	}
}

func TestResolveQueueCaptureIfaceOrphan(t *testing.T) {
	cfg := Default()
	cfg.Capture.Interfaces = []string{"ens34"}
	cfg.Queues = []Queue{{Index: 1, Target: "10.0.0.1", CaptureIface: "ens99"}}
	_, errMsg := cfg.ResolveQueueCaptureIface(cfg.Queues[0])
	if errMsg == "" {
		t.Fatal("expected orphan error")
	}
}
