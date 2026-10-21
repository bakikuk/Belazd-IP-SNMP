//go:build !linux

package capture

import (
	"github.com/bwmon/bwmon/internal/accounting"
	"github.com/bwmon/bwmon/internal/config"
)

func newIfStatsRunner(store *accounting.Store, cfg *config.Config) Runner {
	return nil
}
