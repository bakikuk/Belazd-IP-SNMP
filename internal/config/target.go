package config

import (
	"fmt"
	"net"
	"strings"
)

const (
	IfacePrefixAt = "@"
	IfacePrefixIf = "if:"
)

// IsIfaceTarget is kernel NIC stats (@eth0 or if:eth0), like bpytop/nload.
func IsIfaceTarget(target string) bool {
	target = strings.TrimSpace(target)
	return strings.HasPrefix(target, IfacePrefixAt) || strings.HasPrefix(target, IfacePrefixIf)
}

// ParseIfaceName returns the interface from an iface target.
func ParseIfaceName(target string) (string, error) {
	target = strings.TrimSpace(target)
	switch {
	case strings.HasPrefix(target, IfacePrefixIf):
		name := strings.TrimPrefix(target, IfacePrefixIf)
		if name == "" {
			return "", fmt.Errorf("missing interface name after if:")
		}
		return name, nil
	case strings.HasPrefix(target, IfacePrefixAt):
		name := strings.TrimPrefix(target, IfacePrefixAt)
		if name == "" {
			return "", fmt.Errorf("missing interface name after @")
		}
		return name, nil
	default:
		return "", fmt.Errorf("not an interface target")
	}
}

// IsIPTarget is host or CIDR matched in packet capture.
func IsIPTarget(target string) bool {
	return !IsIfaceTarget(target)
}

// IsCatchAllCIDR is 0.0.0.0/0 — poor match for total throughput (use @iface).
func IsCatchAllCIDR(target string) bool {
	target = strings.TrimSpace(target)
	if !strings.Contains(target, "/") {
		return false
	}
	_, n, err := net.ParseCIDR(target)
	if err != nil || n.IP.To4() == nil {
		return false
	}
	ones, _ := n.Mask.Size()
	return n.IP.Equal(net.IPv4zero) && ones == 0
}
