package api

import (
	"testing"

	"github.com/bwmon/bwmon/internal/config"
)

func TestHTTPAuthTokenRoundTrip(t *testing.T) {
	a := NewHTTPAuth([]config.HTTPUser{{Username: "admin", Password: "x"}}, "secret-key")
	if !a.checkCredentials("admin", "x") || a.checkCredentials("admin", "y") {
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
}
