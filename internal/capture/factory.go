package capture

import (
	"fmt"

	"github.com/bakikuk/beladz-ip-snmp/internal/accounting"
	"github.com/bakikuk/beladz-ip-snmp/internal/config"
)

// NewRunner starts pcap (IP targets), kernel iface stats (@eth0), or simulate.
func NewRunner(store *accounting.Store, cfg *config.Config, simulate bool) (Runner, error) {
	var indices []int
	hasIP := false
	for _, q := range cfg.Queues {
		indices = append(indices, q.Index)
		if config.IsIPTarget(q.Target) {
			hasIP = true
		}
	}
	if simulate {
		return NewSimulate(store, indices), nil
	}
	var parts []Runner
	if hasIP {
		fragments, forceWide := config.CaptureBPFPlan(cfg.Queues)
		pcap, err := NewPcap(store, Config{
			Interface:    cfg.Capture.Interface,
			Snaplen:      cfg.Capture.Snaplen,
			ForceWide:    forceWide,
			BPFFragments: fragments,
		})
		if err != nil {
			return nil, err
		}
		parts = append(parts, pcap)
	}
	if ifs := newIfStatsRunner(store, cfg); ifs != nil {
		parts = append(parts, ifs)
	}
	if len(parts) == 0 {
		return nil, nil
	}
	if len(parts) == 1 {
		return parts[0], nil
	}
	return &multiRunner{parts: parts}, nil
}

// Restart stops old runner and starts new one.
func Restart(old Runner, store *accounting.Store, cfg *config.Config, simulate bool) (Runner, error) {
	if old != nil {
		old.Stop()
	}
	if len(cfg.Queues) == 0 && !simulate {
		return nil, nil
	}
	r, err := NewRunner(store, cfg, simulate)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, nil
	}
	if err := r.Start(); err != nil {
		return nil, fmt.Errorf("capture start: %w", err)
	}
	return r, nil
}
