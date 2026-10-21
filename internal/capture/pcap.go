//go:build (linux || windows) && cgo

package capture

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/bwmon/bwmon/internal/accounting"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
)

const maxBPFClauses = 64

type pcapRunner struct {
	store     *accounting.Store
	iface     string
	snaplen   int
	targets   []string
	handle    *pcap.Handle
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	useWideBP bool
}

// NewPcap creates a libpcap-based runner.
func NewPcap(store *accounting.Store, cfg Config) (Runner, error) {
	if cfg.Interface == "" {
		return nil, fmt.Errorf("capture: interface required")
	}
	return &pcapRunner{
		store:   store,
		iface:   cfg.Interface,
		snaplen: cfg.Snaplen,
		targets: cfg.Targets,
	}, nil
}

func (r *pcapRunner) Start() error {
	filter, wide := buildBPFFilter(r.targets)
	h, err := pcap.OpenLive(r.iface, int32(r.snaplen), true, pcap.BlockForever)
	if err != nil {
		return err
	}
	if filter != "" {
		if err := h.SetBPFFilter(filter); err != nil {
			h.Close()
			return fmt.Errorf("bpf: %w", err)
		}
	}
	r.handle = h
	r.useWideBP = wide
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	src := gopacket.NewPacketSource(h, h.LinkType())
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case pkt, ok := <-src.Packets():
				if !ok {
					return
				}
				r.process(pkt)
			}
		}
	}()
	return nil
}

func (r *pcapRunner) Stop() {
	if r.cancel != nil {
		r.cancel()
	}
	r.wg.Wait()
	if r.handle != nil {
		r.handle.Close()
		r.handle = nil
	}
}

func (r *pcapRunner) process(pkt gopacket.Packet) {
	ipLayer := pkt.Layer(layers.LayerTypeIPv4)
	if ipLayer == nil {
		return
	}
	ip, ok := ipLayer.(*layers.IPv4)
	if !ok {
		return
	}
	length := uint64(len(ipLayer.LayerContents()))
	if r.useWideBP {
		if !packetMatchesTargets(ip.SrcIP, ip.DstIP, r.targets) {
			return
		}
	}
	r.store.AccountIPv4(ip.SrcIP, ip.DstIP, length)
}

func buildBPFFilter(targets []string) (filter string, wide bool) {
	if len(targets) == 0 {
		return "ip", true
	}
	if len(targets) > maxBPFClauses {
		return "ip", true
	}
	var parts []string
	for _, t := range targets {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if strings.Contains(t, "/") {
			ip, n, err := net.ParseCIDR(t)
			if err != nil {
				continue
			}
			mask, _ := n.Mask.Size()
			if ip.To4() != nil {
				parts = append(parts, fmt.Sprintf("net %s/%d", ip.String(), mask))
			}
		} else {
			parts = append(parts, "host "+t)
		}
	}
	if len(parts) == 0 {
		return "ip", true
	}
	return strings.Join(parts, " or "), false
}

func packetMatchesTargets(src, dst net.IP, targets []string) bool {
	for _, t := range targets {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if strings.Contains(t, "/") {
			_, n, err := net.ParseCIDR(t)
			if err != nil {
				continue
			}
			if n.Contains(src) || n.Contains(dst) {
				return true
			}
			continue
		}
		ip := net.ParseIP(t)
		if ip != nil && (ip.Equal(src) || ip.Equal(dst)) {
			return true
		}
	}
	return false
}
