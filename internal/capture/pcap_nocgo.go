//go:build (linux || windows) && !cgo

package capture

import (
	"fmt"

	"github.com/bakikuk/beladz-ip-snmp/internal/accounting"
)

func NewPcap(store *accounting.Store, cfg Config) (Runner, error) {
	return nil, fmt.Errorf("capture: rebuild with CGO_ENABLED=1 and libpcap (see README)")
}
