package db

import (
	"log"
	"os"
	"path/filepath"

	"github.com/bakikuk/belazd-ip-snmp/internal/buildseed"
	"github.com/bakikuk/belazd-ip-snmp/internal/config"
)

// BootstrapOptions for first-start import and seeding.
type BootstrapOptions struct {
	ConfigYAMLPath string
	InsecureSNMP   bool
	InsecureHTTP   bool
}

// Bootstrap runs one-time YAML import and standalone defaults. Web users live in SQLite only (Users UI).
func (s *Store) Bootstrap(opts BootstrapOptions) error {
	empty, err := s.IsEmpty()
	if err != nil {
		return err
	}
	if empty && opts.ConfigYAMLPath != "" {
		if _, err := os.Stat(opts.ConfigYAMLPath); err == nil {
			cfg, err := config.Load(opts.ConfigYAMLPath, opts.InsecureSNMP, opts.InsecureHTTP)
			if err != nil {
				return err
			}
			if err := s.SaveConfig(cfg); err != nil {
				return err
			}
			log.Printf("belazd: imported settings from %s into %s", opts.ConfigYAMLPath, s.path)
			bak := opts.ConfigYAMLPath + ".bak"
			if err := os.Rename(opts.ConfigYAMLPath, bak); err == nil {
				log.Printf("belazd: renamed %s -> %s", opts.ConfigYAMLPath, bak)
			}
		}
	}
	empty, err = s.IsEmpty()
	if err != nil {
		return err
	}
	if empty {
		if err := s.ApplyStandaloneDefaults(); err != nil {
			return err
		}
	}

	hasUsers, err := s.HasUsers()
	if err != nil {
		return err
	}
	if !hasUsers {
		seedUser, seedPass, ok := buildseed.Credentials()
		if ok {
			cfg, err := s.LoadConfig()
			if err != nil {
				return err
			}
			if err := cfg.BootstrapAdminFromEnv(seedUser, seedPass); err != nil {
				return err
			}
			if err := s.SaveConfig(cfg); err != nil {
				return err
			}
			log.Printf("belazd: seeded admin user %q from compile-time build (runtime does not use .env)", seedUser)
		}
	}
	return nil
}

// ConfigYAMLBesideExe returns config.yaml next to the executable (migration path).
func ConfigYAMLBesideExe() string {
	return filepath.Join(ExecutableDir(), "config.yaml")
}
