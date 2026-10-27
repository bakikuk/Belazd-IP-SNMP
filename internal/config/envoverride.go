package config

// applyEnvOverrides is reserved for non-secret boot overrides.
// Operational settings live in belazd.sqlite (Settings UI).
// Legacy helpers only; runtime auth lives in belazd.sqlite.
func applyEnvOverrides(c *Config) {}
