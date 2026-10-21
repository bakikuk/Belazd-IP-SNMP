package config

import (
	"os"
	"regexp"
)

var yamlEnvRef = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// expandEnvInYAML replaces ${VAR} placeholders using the process environment.
func expandEnvInYAML(data []byte) []byte {
	return yamlEnvRef.ReplaceAllFunc(data, func(m []byte) []byte {
		sub := yamlEnvRef.FindSubmatch(m)
		if len(sub) != 2 {
			return m
		}
		name := string(sub[1])
		val, ok := os.LookupEnv(name)
		if !ok {
			return m
		}
		return []byte(val)
	})
}
