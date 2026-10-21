//go:build !linux

package capture

import (
	"github.com/bakikuk/belazd-ip-snmp/internal/accounting"
	"github.com/bakikuk/belazd-ip-snmp/internal/config"
)

func newIfStatsRunner(store *accounting.Store, cfg *config.Config) Runner {
	return nil
}
