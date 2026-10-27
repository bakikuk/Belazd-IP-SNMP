package config

import (
	"fmt"
	"net"
	"strings"
)

// NormalizeInterfaces merges legacy Interface into Interfaces and sets Interface to the default.
func (c *CaptureConfig) NormalizeInterfaces() {
	if len(c.Interfaces) == 0 && strings.TrimSpace(c.Interface) != "" {
		c.Interfaces = []string{strings.TrimSpace(c.Interface)}
	}
	var out []string
	seen := map[string]bool{}
	for _, name := range c.Interfaces {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	c.Interfaces = out
	if len(c.Interfaces) > 0 {
		c.Interface = c.Interfaces[0]
	} else {
		c.Interface = strings.TrimSpace(c.Interface)
	}
}

// DefaultInterface returns the first configured capture NIC.
func (c *CaptureConfig) DefaultInterface() string {
	c.NormalizeInterfaces()
	if len(c.Interfaces) > 0 {
		return c.Interfaces[0]
	}
	return strings.TrimSpace(c.Interface)
}

// InterfaceAllowed reports whether name is in the configured capture list.
func (c *CaptureConfig) InterfaceAllowed(name string) bool {
	c.NormalizeInterfaces()
	name = strings.TrimSpace(name)
	for _, n := range c.Interfaces {
		if n == name {
			return true
		}
	}
	return false
}

// ValidateCaptureInterfaces checks each configured NIC exists on the host.
func (c *CaptureConfig) ValidateCaptureInterfaces() error {
	c.NormalizeInterfaces()
	for _, name := range c.Interfaces {
		if _, err := net.InterfaceByName(name); err != nil {
			return fmt.Errorf("config: capture interface %q: %w", name, err)
		}
	}
	return nil
}

// HasIPCaptureQueues reports whether any queue needs pcap (include/exclude IP).
func (c *Config) HasIPCaptureQueues() bool {
	for _, q := range c.Queues {
		if IsIPTarget(q.Target) {
			return true
		}
	}
	return false
}

// ValidateQueueCaptureIface checks capture_iface on create/update (strict).
func (c *Config) ValidateQueueCaptureIface(q Queue) error {
	if IsIfaceTarget(q.Target) {
		return nil
	}
	if !IsIPTarget(q.Target) {
		return nil
	}
	c.Capture.NormalizeInterfaces()
	name := strings.TrimSpace(q.CaptureIface)
	if name == "" {
		name = c.Capture.DefaultInterface()
	}
	if name == "" {
		return fmt.Errorf("config: capture interface required — add NICs in Settings")
	}
	if !c.Capture.InterfaceAllowed(name) {
		return fmt.Errorf("config: capture_iface %q not in Settings capture list", name)
	}
	return nil
}

// ResolveQueueCaptureIface returns effective NIC and optional soft error for orphans at runtime.
func (c *Config) ResolveQueueCaptureIface(q Queue) (iface string, errMsg string) {
	if IsIfaceTarget(q.Target) {
		name, err := ParseIfaceName(q.Target)
		if err != nil {
			return "", err.Error()
		}
		return name, ""
	}
	if !IsIPTarget(q.Target) {
		return "", ""
	}
	c.Capture.NormalizeInterfaces()
	name := strings.TrimSpace(q.CaptureIface)
	if name == "" {
		name = c.Capture.DefaultInterface()
	}
	if name == "" {
		return "", "no capture NIC configured in Settings"
	}
	if !c.Capture.InterfaceAllowed(name) {
		return name, fmt.Sprintf("capture NIC %q not in Settings", name)
	}
	return name, ""
}

// GroupIPQueuesByCaptureIface returns queues grouped by effective capture NIC (skips invalid/orphan).
func (c *Config) GroupIPQueuesByCaptureIface() (groups map[string][]Queue, skipped []string) {
	groups = make(map[string][]Queue)
	for _, q := range c.Queues {
		if IsIfaceTarget(q.Target) || !IsIPTarget(q.Target) {
			continue
		}
		iface, errMsg := c.ResolveQueueCaptureIface(q)
		if errMsg != "" {
			skipped = append(skipped, fmt.Sprintf("queue %d: %s", q.Index, errMsg))
			continue
		}
		groups[iface] = append(groups[iface], q)
	}
	return groups, skipped
}
