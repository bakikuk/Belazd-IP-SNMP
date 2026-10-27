package config

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// BootstrapAdminFromEnv creates administrators group + user when config has no users (YAML import / tests).
func (c *Config) BootstrapAdminFromEnv(username, password string) error {
	if len(c.Users) > 0 {
		return nil
	}
	if err := ValidateUsername(username); err != nil {
		return err
	}
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	c.UserGroups = []UserGroup{{
		ID:          AdminGroupID,
		Name:        AdminGroupName,
		Permissions: FullModulePermissions(),
	}}
	c.Users = []UserAccount{{
		Username:     username,
		GroupID:      AdminGroupID,
		PasswordHash: hash,
	}}
	return c.ValidateUsersAndGroups()
}

// HashPassword bcrypt-hashes a plaintext password.
func HashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(b), nil
}

// CheckPassword compares plaintext to bcrypt hash.
func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
