package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func applyEnvOverrides(c *Config) {
	applyHTTPListenFromEnv(c)
}

func applyHTTPListenFromEnv(c *Config) {
	if v := strings.TrimSpace(os.Getenv("BELADZ_HTTP_LISTEN")); v != "" {
		c.HTTP.Listen = v
		return
	}
	if v := strings.TrimSpace(os.Getenv("BELADZ_HTTP_PORT")); v != "" {
		c.HTTP.Listen = normalizeHTTPListenPort(v)
	}
}

func normalizeHTTPListenPort(port string) string {
	if strings.Contains(port, ":") {
		return port
	}
	if _, err := strconv.Atoi(port); err != nil {
		return "0.0.0.0:" + port
	}
	return fmt.Sprintf("0.0.0.0:%s", port)
}
