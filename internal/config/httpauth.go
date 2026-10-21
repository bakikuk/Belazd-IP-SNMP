package config

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
)

// HTTPUser is a web UI login account (from .env only).
type HTTPUser struct {
	Username string
	Password string
}

// RandomSessionSecret returns a URL-safe secret for login cookies.
func RandomSessionSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// LoadHTTPAuthFromEnv returns credentials when BELAZD_HTTP_USER and BELAZD_HTTP_PASSWORD are set.
func LoadHTTPAuthFromEnv() (user, password string, enabled bool, err error) {
	user = strings.TrimSpace(os.Getenv("BELAZD_HTTP_USER"))
	password = os.Getenv("BELAZD_HTTP_PASSWORD")
	if user == "" && password == "" {
		return "", "", false, nil
	}
	if user == "" || password == "" {
		return "", "", false, fmt.Errorf("config: set both BELAZD_HTTP_USER and BELAZD_HTTP_PASSWORD in .env")
	}
	return user, password, true, nil
}

func HTTPUsersFromEnv() ([]HTTPUser, bool, error) {
	u, p, on, err := LoadHTTPAuthFromEnv()
	if err != nil || !on {
		return nil, on, err
	}
	return []HTTPUser{{Username: u, Password: p}}, true, nil
}
