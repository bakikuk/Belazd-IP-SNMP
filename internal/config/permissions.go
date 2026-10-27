package config

import (
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// PermissionModule is a UI/API area guarded by RBAC.
type PermissionModule string

const (
	ModuleQueues     PermissionModule = "queues"
	ModuleSettings   PermissionModule = "settings"
	ModuleUsers      PermissionModule = "users"
	ModuleUserGroups PermissionModule = "user_groups"
	ModuleAbout      PermissionModule = "about"
)

// PermissionLevel ordered access: none < read < write < delete < full.
type PermissionLevel int

const (
	PermNone PermissionLevel = iota
	PermRead
	PermWrite
	PermDelete
	PermFull
)

func (p PermissionLevel) String() string {
	switch p {
	case PermNone:
		return "none"
	case PermRead:
		return "read"
	case PermWrite:
		return "write"
	case PermDelete:
		return "delete"
	case PermFull:
		return "full"
	default:
		return "none"
	}
}

// ParsePermissionLevel parses a level string (YAML/JSON).
func ParsePermissionLevel(s string) (PermissionLevel, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "none":
		return PermNone, nil
	case "read":
		return PermRead, nil
	case "write":
		return PermWrite, nil
	case "delete":
		return PermDelete, nil
	case "full":
		return PermFull, nil
	default:
		return PermNone, fmt.Errorf("invalid permission level %q", s)
	}
}

// UnmarshalYAML decodes a permission level string.
func (p *PermissionLevel) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	level, err := ParsePermissionLevel(s)
	if err != nil {
		return err
	}
	*p = level
	return nil
}

// MarshalYAML encodes a permission level string.
func (p PermissionLevel) MarshalYAML() (interface{}, error) {
	return p.String(), nil
}

// UnmarshalJSON decodes a permission level string.
func (p *PermissionLevel) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	level, err := ParsePermissionLevel(s)
	if err != nil {
		return err
	}
	*p = level
	return nil
}

// MarshalJSON encodes a permission level string.
func (p PermissionLevel) MarshalJSON() ([]byte, error) {
	return json.Marshal(p.String())
}

// ModulePermissions per-module levels for a user group.
type ModulePermissions struct {
	Queues     PermissionLevel `yaml:"queues" json:"queues"`
	Settings   PermissionLevel `yaml:"settings" json:"settings"`
	Users      PermissionLevel `yaml:"users" json:"users"`
	UserGroups PermissionLevel `yaml:"user_groups" json:"user_groups"`
	About      PermissionLevel `yaml:"about" json:"about"`
}

// LevelFor returns the level for a module.
func (m ModulePermissions) LevelFor(mod PermissionModule) PermissionLevel {
	switch mod {
	case ModuleQueues:
		return m.Queues
	case ModuleSettings:
		return m.Settings
	case ModuleUsers:
		return m.Users
	case ModuleUserGroups:
		return m.UserGroups
	case ModuleAbout:
		return m.About
	default:
		return PermNone
	}
}

// AtLeast reports whether level for mod is >= required.
func (m ModulePermissions) AtLeast(mod PermissionModule, required PermissionLevel) bool {
	return m.LevelFor(mod) >= required
}

// PermissionsJSON for API session responses.
func (m ModulePermissions) PermissionsJSON() map[string]string {
	return map[string]string{
		string(ModuleQueues):     m.Queues.String(),
		string(ModuleSettings):   m.Settings.String(),
		string(ModuleUsers):      m.Users.String(),
		string(ModuleUserGroups): m.UserGroups.String(),
		string(ModuleAbout):      m.About.String(),
	}
}

// FullModulePermissions all modules at full.
func FullModulePermissions() ModulePermissions {
	return ModulePermissions{
		Queues:     PermFull,
		Settings:   PermFull,
		Users:      PermFull,
		UserGroups: PermFull,
		About:      PermFull,
	}
}

// BackfillLegacyAbout sets about when older databases omit it (mirrors highest legacy module level).
func (m *ModulePermissions) BackfillLegacyAbout() {
	if m.About != PermNone {
		return
	}
	max := PermNone
	for _, mod := range []PermissionModule{ModuleQueues, ModuleSettings, ModuleUsers, ModuleUserGroups} {
		if l := m.LevelFor(mod); l > max {
			max = l
		}
	}
	if max > PermNone {
		m.About = max
	}
}

// UserGroup defines RBAC for members.
type UserGroup struct {
	ID          string            `yaml:"id" json:"id"`
	Name        string            `yaml:"name" json:"name"`
	Permissions ModulePermissions `yaml:"permissions" json:"permissions"`
}

// UserAccount is a web login stored in config.
type UserAccount struct {
	Username     string `yaml:"username" json:"username"`
	GroupID      string `yaml:"group_id" json:"groupId"`
	PasswordHash string `yaml:"password_hash" json:"-"`
}
