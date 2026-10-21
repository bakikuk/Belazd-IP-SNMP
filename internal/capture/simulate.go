package capture

import (
	"context"
	"sync"
	"time"

	"github.com/bakikuk/belazd-ip-snmp/internal/accounting"
)

type simulateRunner struct {
	store  *accounting.Store
	queues []int
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewSimulate returns a runner that increments counters periodically.
func NewSimulate(store *accounting.Store, indices []int) Runner {
	return &simulateRunner{store: store, queues: indices}
}

func (r *simulateRunner) Start() error {
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
				for _, idx := range r.queues {
					r.store.SimulateAdd(idx, 1024, 512)
				}
			}
		}
	}()
	return nil
}

func (r *simulateRunner) Stop() {
	if r.cancel != nil {
		r.cancel()
	}
	r.wg.Wait()
}
