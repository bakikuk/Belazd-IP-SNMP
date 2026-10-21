package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnvLoginVars(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte("BWMON_HTTP_USER=admin\nBWMON_HTTP_PASSWORD=secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_ = LoadDotEnv(envPath)
	user, pass, on, err := LoadHTTPAuthFromEnv()
	if err != nil || !on || user != "admin" || pass != "secret" {
		t.Fatalf("auth: user=%q on=%v err=%v", user, on, err)
	}
}

func TestExpandEnvInYAML(t *testing.T) {
	t.Setenv("BWMON_TEST_VAR", "expanded")
	in := []byte(`value: "${BWMON_TEST_VAR}"`)
	out := expandEnvInYAML(in)
	if string(out) != `value: "expanded"` {
		t.Fatalf("got %q", out)
	}
}
