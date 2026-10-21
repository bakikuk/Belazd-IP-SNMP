//go:build (linux || windows) && cgo

package capture

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/bakikuk/belazd-ip-snmp/internal/accounting"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
)

const maxBPFClauses = 64

type pcapRunner struct {
	store   *accounting.Store
	cfg     Config
	handle  *pcap.Handle
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// NewPcap creates a libpcap-based runner.
func NewPcap(store *accounting.Store, cfg Config) (Runner, error) {
	if cfg.Interface == "" {
		return nil, fmt.Errorf("capture: interface required")
	}
	return &pcapRunner{store: store, cfg: cfg}, nil
}

func (r *pcapRunner) Start() error {
	filter := buildBPFFilter(r.cfg.ForceWide, r.cfg.BPFFragments)
	h, err := pcap.OpenLive(r.cfg.Interface, int32(r.cfg.Snaplen), true, pcap.BlockForever)
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
	r.store.AccountIPv4(ip.SrcIP, ip.DstIP, length)
}

func buildBPFFilter(forceWide bool, fragments []string) string {
	if forceWide || len(fragments) == 0 {
		return "ip"
	}
	if len(fragments) > maxBPFClauses {
		return "ip"
	}
	return strings.Join(fragments, " or ")
}
