package api

import (
	"path/filepath"
	"testing"

	"github.com/bakikuk/belazd-ip-snmp/internal/config"
	"github.com/bakikuk/belazd-ip-snmp/internal/db"
)

func TestBootstrapStandaloneIntoDatabase(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "belazd.sqlite")

	database, err := db.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := database.Bootstrap(db.BootstrapOptions{InsecureHTTP: true, InsecureSNMP: true}); err != nil {
		t.Fatal(err)
	}
	cfg, err := database.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Users) != 0 {
		t.Fatalf("users=%v", cfg.Users)
	}
	if len(cfg.UserGroups) != 1 {
		t.Fatalf("user_groups=%v", cfg.UserGroups)
	}
	_ = config.AdminGroupID
}
