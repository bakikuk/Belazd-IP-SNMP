package db

import (
	"path/filepath"
	"testing"

	"github.com/bakikuk/belazd-ip-snmp/internal/config"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.sqlite")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	c := config.Default()
	c.HTTP.Allowed = []string{"10.0.0.0/8"}
	c.Queues = []config.Queue{{Index: 1, Target: "10.0.0.1", Label: "a", CaptureIface: "eth0"}}
	c.UserGroups = []config.UserGroup{{
		ID:          config.AdminGroupID,
		Name:        config.AdminGroupName,
		Permissions: config.FullModulePermissions(),
	}}
	hash, _ := config.HashPassword("secret12")
	c.Users = []config.UserAccount{{Username: "admin", GroupID: config.AdminGroupID, PasswordHash: hash}}

	if err := s.SaveConfig(c); err != nil {
		t.Fatal(err)
	}
	loaded, err := s.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Queues) != 1 || loaded.Queues[0].Target != "10.0.0.1" {
		t.Fatalf("queues=%v", loaded.Queues)
	}
	if loaded.Queues[0].CaptureIface != "eth0" {
		t.Fatalf("capture_iface=%q", loaded.Queues[0].CaptureIface)
	}
	has, _ := s.HasUsers()
	if !has {
		t.Fatal("expected users")
	}
}

func TestBootstrapStandaloneNoUsers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.sqlite")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.Bootstrap(BootstrapOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := s.Bootstrap(BootstrapOptions{}); err != nil {
		t.Fatal(err)
	}
	cfg, _ := s.LoadConfig()
	if len(cfg.Users) != 0 {
		t.Fatalf("expected no users until UI create, got %v", cfg.Users)
	}
	if len(cfg.UserGroups) != 1 || cfg.UserGroups[0].ID != config.AdminGroupID {
		t.Fatalf("user_groups=%v", cfg.UserGroups)
	}
}
