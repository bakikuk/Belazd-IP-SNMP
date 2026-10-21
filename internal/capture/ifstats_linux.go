//go:build linux

package capture

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bakikuk/beladz-ip-snmp/internal/accounting"
	"github.com/bakikuk/beladz-ip-snmp/internal/config"
)

type ifStatsRunner struct {
	store   *accounting.Store
	queues  []ifaceQueue
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

type ifaceQueue struct {
	index int
	iface string
}

func newIfStatsRunner(store *accounting.Store, cfg *config.Config) Runner {
	var qs []ifaceQueue
	for _, q := range cfg.Queues {
		if !config.IsIfaceTarget(q.Target) {
			continue
		}
		name, err := config.ParseIfaceName(q.Target)
		if err != nil {
			continue
		}
		qs = append(qs, ifaceQueue{index: q.Index, iface: name})
	}
	if len(qs) == 0 {
		return nil
	}
	return &ifStatsRunner{store: store, queues: qs}
}

func (r *ifStatsRunner) Start() error {
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		t := time.NewTicker(time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				for _, q := range r.queues {
					rx, err1 := readNetStat(q.iface, "rx_bytes")
					tx, err2 := readNetStat(q.iface, "tx_bytes")
					if err1 != nil || err2 != nil {
						continue
					}
					r.store.SetIfaceKernel(q.index, rx, tx)
				}
			}
		}
	}()
	return nil
}

func (r *ifStatsRunner) Stop() {
	if r.cancel != nil {
		r.cancel()
	}
	r.wg.Wait()
}

func readNetStat(iface, stat string) (uint64, error) {
	p := filepath.Join("/sys/class/net", iface, "statistics", stat)
	b, err := os.ReadFile(p)
	if err != nil {
		return 0, err
	}
	return strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64)
}
