package config

import (
	"os"
	"testing"
)

func TestLoadHTTPAuthFromEnv(t *testing.T) {
	t.Setenv("BELADZ_HTTP_USER", "admin")
	t.Setenv("BELADZ_HTTP_PASSWORD", "secret")
	user, pass, on, err := LoadHTTPAuthFromEnv()
	if err != nil || !on || user != "admin" || pass != "secret" {
		t.Fatalf("user=%q pass=%q on=%v err=%v", user, pass, on, err)
	}
}

func TestLoadHTTPAuthPartial(t *testing.T) {
	t.Setenv("BELADZ_HTTP_USER", "admin")
	t.Setenv("BELADZ_HTTP_PASSWORD", "")
	_, _, _, err := LoadHTTPAuthFromEnv()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadHTTPAuthDisabled(t *testing.T) {
	os.Unsetenv("BELADZ_HTTP_USER")
	os.Unsetenv("BELADZ_HTTP_PASSWORD")
	_, _, on, err := LoadHTTPAuthFromEnv()
	if err != nil || on {
		t.Fatalf("on=%v err=%v", on, err)
	}
}
