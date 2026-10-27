package accounting

import "testing"

func TestIPv4DatagramLength(t *testing.T) {
	if got := IPv4DatagramLength(1500, 100); got != 1500 {
		t.Fatalf("total length: got %d", got)
	}
	if got := IPv4DatagramLength(0, 128); got != 128 {
		t.Fatalf("fallback captured: got %d", got)
	}
}
