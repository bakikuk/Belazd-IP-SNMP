package config

import (
	"fmt"
	"strings"
)

const (
	AdminGroupID   = "administrators"
	AdminGroupName = "Administrators"
)

// ValidateUsersAndGroups checks user/group references when users are configured.
func (c *Config) ValidateUsersAndGroups() error {
	if len(c.Users) == 0 {
		return nil
	}
	groupIDs := make(map[string]int, len(c.UserGroups))
	for i, g := range c.UserGroups {
		if err := ValidateGroupID(g.ID); err != nil {
			return fmt.Errorf("user_groups[%d].id: %w", i, err)
		}
		if strings.TrimSpace(g.Name) == "" {
			return fmt.Errorf("user_groups[%d].name required", i)
		}
		if _, dup := groupIDs[g.ID]; dup {
			return fmt.Errorf("duplicate user group id %q", g.ID)
		}
		groupIDs[g.ID] = i
	}
	if len(groupIDs) == 0 {
		return fmt.Errorf("config: users require at least one user_group")
	}
	seenUser := map[string]bool{}
	for i, u := range c.Users {
		if err := ValidateUsername(u.Username); err != nil {
			return fmt.Errorf("users[%d]: %w", i, err)
		}
		if seenUser[u.Username] {
			return fmt.Errorf("duplicate username %q", u.Username)
		}
		seenUser[u.Username] = true
		if _, ok := groupIDs[u.GroupID]; !ok {
			return fmt.Errorf("users[%d]: unknown group_id %q", i, u.GroupID)
		}
		if strings.TrimSpace(u.PasswordHash) == "" {
			return fmt.Errorf("users[%d]: password_hash required", i)
		}
	}
	return nil
}

// GroupByID returns a group pointer from config (nil if missing).
func (c *Config) GroupByID(id string) *UserGroup {
	for i := range c.UserGroups {
		if c.UserGroups[i].ID == id {
			return &c.UserGroups[i]
		}
	}
	return nil
}

// PermissionsForUser returns module permissions for username (none if unknown).
func (c *Config) PermissionsForUser(username string) ModulePermissions {
	for _, u := range c.Users {
		if u.Username == username {
			if g := c.GroupByID(u.GroupID); g != nil {
				return g.Permissions
			}
			break
		}
	}
	return ModulePermissions{}
}

// CountPrivilegedUsers counts users whose group has users+user_groups at full.
func (c *Config) CountPrivilegedUsers() int {
	n := 0
	for _, u := range c.Users {
		g := c.GroupByID(u.GroupID)
		if g == nil {
			continue
		}
		if g.Permissions.AtLeast(ModuleUsers, PermFull) && g.Permissions.AtLeast(ModuleUserGroups, PermFull) {
			n++
		}
	}
	return n
}

// UsersInGroup counts members of a group.
func (c *Config) UsersInGroup(groupID string) int {
	n := 0
	for _, u := range c.Users {
		if u.GroupID == groupID {
			n++
		}
	}
	return n
}

// IsPrivilegedUser reports full users+user_groups access.
func (c *Config) IsPrivilegedUser(username string) bool {
	p := c.PermissionsForUser(username)
	return p.AtLeast(ModuleUsers, PermFull) && p.AtLeast(ModuleUserGroups, PermFull)
}
