package config

import (
	"net"
	"testing"
)

func TestParseTargetIncludeMulti(t *testing.T) {
	spec, err := ParseTarget("10.0.0.1,192.168.1.0/24")
	if err != nil || spec.Mode != TargetModeInclude || len(spec.Entries) != 2 {
		t.Fatalf("spec=%+v err=%v", spec, err)
	}
	if !spec.ContainsIP(net.ParseIP("10.0.0.1")) {
		t.Fatal("host")
	}
	if !spec.ContainsIP(net.ParseIP("192.168.1.55")) {
		t.Fatal("cidr")
	}
}

func TestParseTargetExclude(t *testing.T) {
	spec, err := ParseTarget("!10.0.0.50,!192.168.1.0/24")
	if err != nil || spec.Mode != TargetModeExclude || len(spec.Entries) != 2 {
		t.Fatalf("spec=%+v err=%v", spec, err)
	}
	if spec.MatchesIn(net.ParseIP("10.0.0.50")) {
		t.Fatal("excluded dst")
	}
	if !spec.MatchesIn(net.ParseIP("10.0.0.51")) {
		t.Fatal("other dst")
	}
}

func TestParseTargetMixedInvalid(t *testing.T) {
	_, err := ParseTarget("10.0.0.1,!10.0.0.2")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseTargetIface(t *testing.T) {
	spec, err := ParseTarget("@eth0")
	if err != nil || spec.Mode != TargetModeIface || spec.IfaceName != "eth0" {
		t.Fatalf("spec=%+v err=%v", spec, err)
	}
}

func TestParseTargetSingleIPBackwardCompat(t *testing.T) {
	spec, err := ParseTarget("203.0.113.5")
	if err != nil || len(spec.Entries) != 1 || spec.Mode != TargetModeInclude {
		t.Fatalf("spec=%+v err=%v", spec, err)
	}
}

func TestCaptureBPFPlanExcludeForcesWide(t *testing.T) {
	_, wide := CaptureBPFPlan([]Queue{
		{Index: 1, Target: "!10.0.0.1"},
	})
	if !wide {
		t.Fatal("exclude queue should force wide capture")
	}
}

func TestCaptureBPFPlanIncludeFragments(t *testing.T) {
	frags, wide := CaptureBPFPlan([]Queue{
		{Index: 1, Target: "10.0.0.1,10.0.0.0/24"},
	})
	if wide || len(frags) != 2 {
		t.Fatalf("frags=%v wide=%v", frags, wide)
	}
}

func TestTargetKind(t *testing.T) {
	if TargetKind("@eth0") != "iface" {
		t.Fatal("iface")
	}
	if TargetKind("!1.2.3.4") != "exclude" {
		t.Fatal("exclude")
	}
	if TargetKind("1.2.3.4,5.6.7.8") != "include" {
		t.Fatal("include")
	}
}
