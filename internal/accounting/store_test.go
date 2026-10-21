package accounting

import (
	"net"
	"testing"

	"github.com/bakikuk/beladz-ip-snmp/internal/config"
)

func TestStoreCounters(t *testing.T) {
	queues := []config.Queue{
		{Index: 1, Target: "10.0.0.1"},
		{Index: 2, Target: "192.168.1.0/24"},
	}
	s := NewStore(queues)
	s.AccountIPv4(net.ParseIP("1.2.3.4"), net.ParseIP("10.0.0.1"), 100)
	s.AccountIPv4(net.ParseIP("10.0.0.1"), net.ParseIP("1.2.3.4"), 200)
	s.AccountIPv4(net.ParseIP("1.2.3.4"), net.ParseIP("192.168.1.50"), 50)
	s.AccountIPv4(net.ParseIP("192.168.1.50"), net.ParseIP("8.8.8.8"), 80)

	snap := s.Snapshot()
	if snap[1].BytesIn != 100 || snap[1].BytesOut != 200 {
		t.Fatalf("host queue: got in=%d out=%d", snap[1].BytesIn, snap[1].BytesOut)
	}
	if snap[2].BytesIn != 50 || snap[2].BytesOut != 80 {
		t.Fatalf("cidr queue: got in=%d out=%d", snap[2].BytesIn, snap[2].BytesOut)
	}
}

func TestStoreIncludeMulti(t *testing.T) {
	s := NewStore([]config.Queue{{Index: 1, Target: "10.0.0.1,10.0.0.2"}})
	s.AccountIPv4(net.ParseIP("1.2.3.4"), net.ParseIP("10.0.0.2"), 40)
	snap := s.Snapshot()
	if snap[1].BytesIn != 40 {
		t.Fatalf("in=%d", snap[1].BytesIn)
	}
}

func TestStoreExclude(t *testing.T) {
	s := NewStore([]config.Queue{{Index: 1, Target: "!10.0.0.99"}})
	s.AccountIPv4(net.ParseIP("1.2.3.4"), net.ParseIP("10.0.0.99"), 100)
	s.AccountIPv4(net.ParseIP("1.2.3.4"), net.ParseIP("10.0.0.1"), 50)
	snap := s.Snapshot()
	if snap[1].BytesIn != 50 {
		t.Fatalf("in=%d", snap[1].BytesIn)
	}
}
