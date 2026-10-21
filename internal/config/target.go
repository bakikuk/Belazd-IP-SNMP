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

// TargetMode is how a queue target matches traffic.
type TargetMode int

const (
	TargetModeInclude TargetMode = iota
	TargetModeExclude
	TargetModeIface
)

// TargetEntry is one IPv4 host or CIDR in a target expression.
type TargetEntry struct {
	Host net.IP
	Net  *net.IPNet
}

// TargetSpec is a parsed queue target.
type TargetSpec struct {
	Mode      TargetMode
	Raw       string
	Entries   []TargetEntry
	IfaceName string
}

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

// ParseTarget parses include list, exclude list (!ip), or interface target.
func ParseTarget(target string) (TargetSpec, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return TargetSpec{}, fmt.Errorf("empty target")
	}
	if IsIfaceTarget(target) {
		if strings.Contains(target, ",") {
			return TargetSpec{}, fmt.Errorf("interface target cannot contain commas")
		}
		name, err := ParseIfaceName(target)
		if err != nil {
			return TargetSpec{}, err
		}
		return TargetSpec{Mode: TargetModeIface, Raw: target, IfaceName: name}, nil
	}
	tokens := splitTargetTokens(target)
	if len(tokens) == 0 {
		return TargetSpec{}, fmt.Errorf("empty target")
	}
	exclude := strings.HasPrefix(tokens[0], "!")
	var entries []TargetEntry
	for _, tok := range tokens {
		if exclude {
			if !strings.HasPrefix(tok, "!") {
				return TargetSpec{}, fmt.Errorf("cannot mix include and exclude in one target")
			}
			tok = strings.TrimPrefix(tok, "!")
		} else if strings.HasPrefix(tok, "!") {
			return TargetSpec{}, fmt.Errorf("cannot mix include and exclude in one target")
		}
		e, err := parseTargetEntry(tok)
		if err != nil {
			return TargetSpec{}, fmt.Errorf("%q: %w", tok, err)
		}
		entries = append(entries, e)
	}
	mode := TargetModeInclude
	if exclude {
		mode = TargetModeExclude
	}
	return TargetSpec{Mode: mode, Raw: target, Entries: entries}, nil
}

func splitTargetTokens(target string) []string {
	parts := strings.Split(target, ",")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func parseTargetEntry(token string) (TargetEntry, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return TargetEntry{}, fmt.Errorf("empty entry")
	}
	if strings.Contains(token, "/") {
		ip, n, err := net.ParseCIDR(token)
		if err != nil {
			return TargetEntry{}, err
		}
		if ip.To4() == nil {
			return TargetEntry{}, fmt.Errorf("IPv6 not supported")
		}
		return TargetEntry{Net: n}, nil
	}
	ip := net.ParseIP(token)
	if ip == nil || ip.To4() == nil {
		return TargetEntry{}, fmt.Errorf("invalid IPv4")
	}
	return TargetEntry{Host: ip.To4()}, nil
}

// TargetKind returns include, exclude, or iface for API/UI.
func TargetKind(target string) string {
	if IsIfaceTarget(target) {
		return "iface"
	}
	t := strings.TrimSpace(target)
	if strings.HasPrefix(t, "!") || strings.Contains(t, ",!") {
		return "exclude"
	}
	return "include"
}

// TargetModeLabel is a short badge label.
func TargetModeLabel(target string) string {
	switch TargetKind(target) {
	case "iface":
		name, _ := ParseIfaceName(target)
		return "IFACE " + name
	case "exclude":
		n := len(splitTargetTokens(target))
		return fmt.Sprintf("EXCEPT · %d", n)
	default:
		n := len(splitTargetTokens(target))
		if n <= 1 {
			return "INCLUDE"
		}
		return fmt.Sprintf("INCLUDE · %d", n)
	}
}

// ContainsIP reports whether ip matches any entry (host or CIDR).
func (s TargetSpec) ContainsIP(ip net.IP) bool {
	ip = ip.To4()
	if ip == nil {
		return false
	}
	for _, e := range s.Entries {
		if e.Host != nil && e.Host.Equal(ip) {
			return true
		}
		if e.Net != nil && e.Net.Contains(ip) {
			return true
		}
	}
	return false
}

// MatchesIn is true when bytesIn should increment for dst.
func (s TargetSpec) MatchesIn(dst net.IP) bool {
	switch s.Mode {
	case TargetModeInclude:
		return s.ContainsIP(dst)
	case TargetModeExclude:
		return !s.ContainsIP(dst)
	default:
		return false
	}
}

// MatchesOut is true when bytesOut should increment for src.
func (s TargetSpec) MatchesOut(src net.IP) bool {
	switch s.Mode {
	case TargetModeInclude:
		return s.ContainsIP(src)
	case TargetModeExclude:
		return !s.ContainsIP(src)
	default:
		return false
	}
}

// BPFFragment returns bpf clauses (host/net) for include entries; empty for exclude/iface.
func (s TargetSpec) BPFFragments() []string {
	if s.Mode != TargetModeInclude {
		return nil
	}
	var parts []string
	for _, e := range s.Entries {
		if e.Host != nil {
			parts = append(parts, "host "+e.Host.String())
		} else if e.Net != nil {
			ones, _ := e.Net.Mask.Size()
			parts = append(parts, fmt.Sprintf("net %s/%d", e.Net.IP.String(), ones))
		}
	}
	return parts
}

// CaptureBPFPlan returns merged include BPF fragments and whether all IPv4 must be captured.
func CaptureBPFPlan(queues []Queue) (fragments []string, forceWide bool) {
	for _, q := range queues {
		if IsIfaceTarget(q.Target) {
			continue
		}
		spec, err := ParseTarget(q.Target)
		if err != nil {
			continue
		}
		if spec.Mode == TargetModeExclude {
			forceWide = true
		}
		fragments = append(fragments, spec.BPFFragments()...)
	}
	return fragments, forceWide
}

// IsCatchAllCIDR is 0.0.0.0/0 — poor match for total throughput (use @iface).
func IsCatchAllCIDR(target string) bool {
	spec, err := ParseTarget(target)
	if err != nil || spec.Mode != TargetModeInclude {
		return false
	}
	for _, e := range spec.Entries {
		if e.Net == nil {
			continue
		}
		ones, _ := e.Net.Mask.Size()
		if e.Net.IP.Equal(net.IPv4zero) && ones == 0 {
			return true
		}
	}
	return false
}
