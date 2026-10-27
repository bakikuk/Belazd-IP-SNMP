package db

import (
	"encoding/base64"
	"path/filepath"
	"testing"

	"github.com/bakikuk/belazd-ip-snmp/internal/buildseed"
)

func TestBootstrapCompileTimeAdminOnce(t *testing.T) {
	t.Cleanup(func() {
		buildseed.AdminUserB64 = ""
		buildseed.AdminPassB64 = ""
	})
	buildseed.AdminUserB64 = base64.StdEncoding.EncodeToString([]byte("buildadmin"))
	buildseed.AdminPassB64 = base64.StdEncoding.EncodeToString([]byte("buildpass1"))

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
	cfg, _ := s.LoadConfig()
	if len(cfg.Users) != 1 || cfg.Users[0].Username != "buildadmin" {
		t.Fatalf("users=%v", cfg.Users)
	}
	buildseed.AdminUserB64 = base64.StdEncoding.EncodeToString([]byte("other"))
	buildseed.AdminPassB64 = base64.StdEncoding.EncodeToString([]byte("otherpass1"))
	if err := s.Bootstrap(BootstrapOptions{}); err != nil {
		t.Fatal(err)
	}
	cfg2, _ := s.LoadConfig()
	if cfg2.Users[0].Username != "buildadmin" {
		t.Fatal("compile-time seed must not overwrite existing user")
	}
}
