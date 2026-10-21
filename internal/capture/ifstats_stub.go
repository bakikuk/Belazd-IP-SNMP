//go:build !linux

package capture

import (
	"github.com/bakikuk/beladz-ip-snmp/internal/accounting"
	"github.com/bakikuk/beladz-ip-snmp/internal/config"
)

func newIfStatsRunner(store *accounting.Store, cfg *config.Config) Runner {
	return nil
}
