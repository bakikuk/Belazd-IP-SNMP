package api

import (
	"testing"

	"github.com/bakikuk/belazd-ip-snmp/internal/config"
)

func testAuthConfig(t *testing.T) *config.Config {
	t.Helper()
	c := config.Default()
	hash, err := config.HashPassword("secret12")
	if err != nil {
		t.Fatal(err)
	}
	c.UserGroups = []config.UserGroup{{
		ID:          config.AdminGroupID,
		Name:        config.AdminGroupName,
		Permissions: config.FullModulePermissions(),
	}}
	c.Users = []config.UserAccount{{
		Username:     "admin",
		GroupID:      config.AdminGroupID,
		PasswordHash: hash,
	}}
	return c
}

func TestHTTPAuthTokenInvalidAfterBootChange(t *testing.T) {
	cfg := testAuthConfig(t)
	a1 := NewHTTPAuth(cfg, "secret-key", []byte("boot-a"))
	tok, err := a1.issueToken("admin")
	if err != nil {
		t.Fatal(err)
	}
	a2 := NewHTTPAuth(cfg, "secret-key", []byte("boot-b"))
	if _, ok := a2.validateToken(tok); ok {
		t.Fatal("token must not validate after process restart (boot id)")
	}
}

func TestHTTPAuthTokenRoundTrip(t *testing.T) {
	a := NewHTTPAuth(testAuthConfig(t), "secret-key", []byte("boot-a"))
	if !a.checkCredentials("admin", "secret12") || a.checkCredentials("admin", "wrong") {
		t.Fatal("credentials")
	}
	tok, err := a.issueToken("admin")
	if err != nil {
		t.Fatal(err)
	}
	user, ok := a.validateToken(tok)
	if !ok || user != "admin" {
		t.Fatalf("validate: user=%q ok=%v", user, ok)
	}
	perms := a.permissionsFor("admin")
	if !perms.AtLeast(config.ModuleQueues, config.PermFull) {
		t.Fatal("expected full queues")
	}
}

func TestBootstrapAdminFromEnv(t *testing.T) {
	c := config.Default()
	if err := c.BootstrapAdminFromEnv("admin", "bootstrap1"); err != nil {
		t.Fatal(err)
	}
	if len(c.Users) != 1 || len(c.UserGroups) != 1 {
		t.Fatalf("users=%d groups=%d", len(c.Users), len(c.UserGroups))
	}
	if !config.CheckPassword(c.Users[0].PasswordHash, "bootstrap1") {
		t.Fatal("hash mismatch")
	}
}
