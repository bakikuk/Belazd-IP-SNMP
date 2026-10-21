//go:build !linux && !windows

package capture

import (
	"fmt"

	"github.com/bwmon/bwmon/internal/accounting"
)

func NewPcap(store *accounting.Store, cfg Config) (Runner, error) {
	return nil, fmt.Errorf("capture: pcap not supported on this platform")
}
