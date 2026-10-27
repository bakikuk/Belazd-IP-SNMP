package buildseed

import (
	"encoding/base64"
	"strings"
)

// AdminUserB64 and AdminPassB64 are set at link time from .env via scripts/build.* (-ldflags).
// Empty in dev builds (`go build` without those flags).
var (
	AdminUserB64 string
	AdminPassB64 string
)

// Credentials returns compile-time admin credentials when both were embedded at build.
func Credentials() (user, password string, ok bool) {
	u, p, ok := decodePair(AdminUserB64, AdminPassB64)
	return u, p, ok
}

func decodePair(userB64, passB64 string) (user, password string, ok bool) {
	userB64 = strings.TrimSpace(userB64)
	passB64 = strings.TrimSpace(passB64)
	if userB64 == "" || passB64 == "" {
		return "", "", false
	}
	ub, err := base64.StdEncoding.DecodeString(userB64)
	if err != nil {
		return "", "", false
	}
	pb, err := base64.StdEncoding.DecodeString(passB64)
	if err != nil {
		return "", "", false
	}
	user = strings.TrimSpace(string(ub))
	password = string(pb)
	if user == "" || password == "" {
		return "", "", false
	}
	return user, password, true
}
