package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnvLoginVars(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte("BELAZD_HTTP_USER=admin\nBELAZD_HTTP_PASSWORD=secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_ = LoadDotEnv(envPath)
	user, pass, on, err := LoadHTTPAuthFromEnv()
	if err != nil || !on || user != "admin" || pass != "secret" {
		t.Fatalf("auth: user=%q on=%v err=%v", user, on, err)
	}
}

func TestHTTPListenFromEnv(t *testing.T) {
	t.Setenv("BELAZD_HTTP_PORT", "9090")
	c := Default()
	applyEnvOverrides(c)
	if c.HTTP.Listen != "0.0.0.0:9090" {
		t.Fatalf("listen=%q", c.HTTP.Listen)
	}
	t.Setenv("BELAZD_HTTP_LISTEN", "127.0.0.1:8080")
	t.Setenv("BELAZD_HTTP_PORT", "9999")
	c2 := Default()
	applyEnvOverrides(c2)
	if c2.HTTP.Listen != "127.0.0.1:8080" {
		t.Fatalf("listen=%q", c2.HTTP.Listen)
	}
}

func TestExpandEnvInYAML(t *testing.T) {
	t.Setenv("BELAZD_TEST_VAR", "expanded")
	in := []byte(`value: "${BELAZD_TEST_VAR}"`)
	out := expandEnvInYAML(in)
	if string(out) != `value: "expanded"` {
		t.Fatalf("got %q", out)
	}
}
