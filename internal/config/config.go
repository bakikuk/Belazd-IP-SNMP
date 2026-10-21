package config

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config is the on-disk configuration for Beladz-IP-SNMP.
type Config struct {
	Capture CaptureConfig `yaml:"capture"`
	SNMP    SNMPConfig    `yaml:"snmp"`
	HTTP    HTTPConfig    `yaml:"http"`
	Queues  []Queue       `yaml:"queues"`
	Meta    Meta          `yaml:"meta"`
}

type CaptureConfig struct {
	Interface string `yaml:"interface"`
	Snaplen   int    `yaml:"snaplen"`
}

type SNMPConfig struct {
	Listen           string           `yaml:"listen"`
	PEN              int              `yaml:"pen"`
	OIDBase          string           `yaml:"oid_base"`
	OIDSuffix        OIDSuffix        `yaml:"oid_suffix"`
	Communities      []CommunityEntry `yaml:"communities"`
	InsecureAllowAll bool             `yaml:"insecure_allow_all"`
}

type CommunityEntry struct {
	Community string   `yaml:"community"`
	Allowed   []string `yaml:"allowed"`
}

type OIDSuffix struct {
	QueueCount string `yaml:"queue_count"`
	TargetName string `yaml:"target_name"`
	BytesIn    string `yaml:"bytes_in"`
	BytesOut   string `yaml:"bytes_out"`
}

type HTTPConfig struct {
	Listen           string   `yaml:"listen"`
	Allowed          []string `yaml:"allowed"`
	SessionSecret    string   `yaml:"session_secret,omitempty"`
	InsecureAllowAll bool     `yaml:"insecure_allow_all"`
}

type Queue struct {
	Index  int    `yaml:"index"`
	Target string `yaml:"target"`
	Label  string `yaml:"label,omitempty"`
}

type Meta struct {
	NextIndex int `yaml:"next_index"`
}

// Default returns a template config (not valid for strict startup).
func Default() *Config {
	return &Config{
		Capture: CaptureConfig{Interface: "eth0", Snaplen: 65535},
		SNMP: SNMPConfig{
			Listen: ":1161",
			PEN:    55555,
			OIDSuffix: OIDSuffix{
				QueueCount: ".1.1.1.0",
				TargetName: ".1.1.2.1.1.2",
				BytesIn:    ".1.1.2.1.1.8",
				BytesOut:   ".1.1.2.1.1.9",
			},
		},
		HTTP: HTTPConfig{Listen: ":8080"},
		Meta: Meta{NextIndex: 1},
	}
}

// LoadEnvFiles loads dotenv files (missing paths are ignored).
func LoadEnvFiles(paths ...string) {
	for _, p := range paths {
		if p == "" {
			continue
		}
		_ = LoadDotEnv(p)
	}
}

// Load reads and validates configuration.
// Loads .env (see .env.example) and optional ${VAR} in YAML, then BELADZ_HTTP_* overrides.
func Load(path string, insecureSNMP, insecureHTTP bool, envFiles ...string) (*Config, error) {
	envFiles = dedupePaths(append(envFiles, DefaultEnvPaths(path)...))
	LoadEnvFiles(envFiles...)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	data = expandEnvInYAML(data)
	c := Default()
	if err := yaml.Unmarshal(data, c); err != nil {
		return nil, err
	}
	if c.Capture.Snaplen <= 0 {
		c.Capture.Snaplen = 65535
	}
	if c.Meta.NextIndex <= 0 {
		c.Meta.NextIndex = 1
	}
	if insecureSNMP {
		c.SNMP.InsecureAllowAll = true
	}
	if insecureHTTP {
		c.HTTP.InsecureAllowAll = true
	}
	applyEnvOverrides(c)
	if err := c.Validate(insecureSNMP, insecureHTTP); err != nil {
		return nil, err
	}
	return c, nil
}

// Validate checks configuration semantics.
func (c *Config) Validate(insecureSNMP, insecureHTTP bool) error {
	if c.SNMP.PEN <= 0 && strings.TrimSpace(c.SNMP.OIDBase) == "" {
		return fmt.Errorf("config: snmp.pen or snmp.oid_base required")
	}
	if err := validateOIDSuffix(c.SNMP.OIDSuffix); err != nil {
		return err
	}
	if !insecureSNMP && !c.SNMP.InsecureAllowAll {
		if len(c.SNMP.Communities) == 0 {
			return fmt.Errorf("config: snmp.communities required (or use --insecure-snmp-open)")
		}
		for i, com := range c.SNMP.Communities {
			if strings.TrimSpace(com.Community) == "" {
				return fmt.Errorf("config: snmp.communities[%d].community empty", i)
			}
			if len(com.Allowed) == 0 {
				return fmt.Errorf("config: snmp.communities[%d].allowed must be non-empty", i)
			}
		}
	}
	if !insecureHTTP && !c.HTTP.InsecureAllowAll {
		if len(c.HTTP.Allowed) == 0 {
			return fmt.Errorf("config: http.allowed required (or use --insecure-http-open)")
		}
	}
	seen := map[int]bool{}
	for i, q := range c.Queues {
		if q.Index <= 0 {
			return fmt.Errorf("config: queues[%d].index invalid", i)
		}
		if seen[q.Index] {
			return fmt.Errorf("config: duplicate queue index %d", q.Index)
		}
		seen[q.Index] = true
		if err := ValidateTarget(q.Target); err != nil {
			return fmt.Errorf("queues[%d]: %w", i, err)
		}
	}
	return nil
}

func validateOIDSuffix(s OIDSuffix) error {
	for _, part := range []struct {
		name, val string
	}{
		{"queue_count", s.QueueCount},
		{"target_name", s.TargetName},
		{"bytes_in", s.BytesIn},
		{"bytes_out", s.BytesOut},
	} {
		if !strings.HasPrefix(part.val, ".") {
			return fmt.Errorf("config: oid_suffix.%s must start with '.'", part.name)
		}
	}
	return nil
}

// ValidateTarget checks include/exclude IP lists or interface target (@eth0 / if:eth0).
func ValidateTarget(target string) error {
	spec, err := ParseTarget(target)
	if err != nil {
		return err
	}
	if spec.Mode == TargetModeIface {
		if _, err := net.InterfaceByName(spec.IfaceName); err != nil {
			return fmt.Errorf("interface %q: %w", spec.IfaceName, err)
		}
		return nil
	}
	if len(spec.Entries) == 0 {
		return fmt.Errorf("target needs at least one IP or CIDR")
	}
	return nil
}

// TableBase returns the enterprise OID root for suffix concatenation (no leading dot).
func (c *Config) TableBase() string {
	if b := strings.TrimSpace(c.SNMP.OIDBase); b != "" {
		return strings.TrimPrefix(b, ".")
	}
	return fmt.Sprintf("1.3.6.1.4.1.%d", c.SNMP.PEN)
}

// OIDFull joins base + suffix + optional index.
func (c *Config) OIDFull(suffix string, index int) string {
	base := c.TableBase()
	suf := strings.TrimPrefix(suffix, ".")
	if index > 0 {
		return "." + base + "." + suf + "." + fmt.Sprintf("%d", index)
	}
	return "." + base + "." + suf
}

// Save writes config atomically.
func (c *Config) Save(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// AssignNextIndex returns the next queue index and bumps meta.
func (c *Config) AssignNextIndex() int {
	max := 0
	for _, q := range c.Queues {
		if q.Index > max {
			max = q.Index
		}
	}
	if c.Meta.NextIndex <= max {
		c.Meta.NextIndex = max + 1
	}
	idx := c.Meta.NextIndex
	c.Meta.NextIndex++
	return idx
}

// DisplayName for SNMP/UI.
func (q Queue) DisplayName() string {
	if strings.TrimSpace(q.Label) != "" {
		return q.Label
	}
	return q.Target
}
