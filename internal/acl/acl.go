package acl

import (
	"fmt"
	"net"
	"strings"
)

// NetSet holds parsed IPv4 networks and host addresses.
type NetSet struct {
	nets []*net.IPNet
	ips  []net.IP
}

// Parse builds a NetSet from CIDR or host strings (IPv4 only).
func Parse(entries []string) (*NetSet, error) {
	s := &NetSet{}
	for _, raw := range entries {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return nil, fmt.Errorf("acl: empty entry")
		}
		if strings.Contains(raw, "/") {
			_, n, err := net.ParseCIDR(raw)
			if err != nil {
				return nil, fmt.Errorf("acl: parse CIDR %q: %w", raw, err)
			}
			if n.IP.To4() == nil {
				return nil, fmt.Errorf("acl: IPv6 not supported: %q", raw)
			}
			s.nets = append(s.nets, n)
			continue
		}
		ip := net.ParseIP(raw)
		if ip == nil || ip.To4() == nil {
			return nil, fmt.Errorf("acl: invalid IPv4: %q", raw)
		}
		s.ips = append(s.ips, ip.To4())
	}
	return s, nil
}

// Contains reports whether ip is allowed.
func (s *NetSet) Contains(ip net.IP) bool {
	if s == nil {
		return false
	}
	ip = ip.To4()
	if ip == nil {
		return false
	}
	for _, h := range s.ips {
		if h.Equal(ip) {
			return true
		}
	}
	for _, n := range s.nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}
