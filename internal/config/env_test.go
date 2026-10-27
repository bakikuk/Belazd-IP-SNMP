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

func TestHTTPListenNotFromEnv(t *testing.T) {
	t.Setenv("BELAZD_HTTP_PORT", "9090")
	t.Setenv("BELAZD_HTTP_LISTEN", "127.0.0.1:9999")
	c := Default()
	applyEnvOverrides(c)
	if c.HTTP.Listen != FixedHTTPListen {
		t.Fatalf("default http listen=%q want %q", c.HTTP.Listen, FixedHTTPListen)
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
