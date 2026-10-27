package buildseed

import (
	"encoding/base64"
	"testing"
)

func TestCredentialsEmpty(t *testing.T) {
	AdminUserB64 = ""
	AdminPassB64 = ""
	if _, _, ok := Credentials(); ok {
		t.Fatal("expected disabled")
	}
}

func TestCredentialsDecode(t *testing.T) {
	AdminUserB64 = base64.StdEncoding.EncodeToString([]byte("admin"))
	AdminPassB64 = base64.StdEncoding.EncodeToString([]byte("secret9x"))
	u, p, ok := Credentials()
	if !ok || u != "admin" || p != "secret9x" {
		t.Fatalf("ok=%v u=%q p=%q", ok, u, p)
	}
}
