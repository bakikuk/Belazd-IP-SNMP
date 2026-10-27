package config

import (
	"fmt"
	"strings"
	"unicode"
)

// ValidatePasswordPolicy enforces min 8 chars with at least one letter and one digit.
func ValidatePasswordPolicy(password string) error {
	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	var hasLetter, hasDigit bool
	for _, r := range password {
		if unicode.IsLetter(r) {
			hasLetter = true
		}
		if unicode.IsDigit(r) {
			hasDigit = true
		}
	}
	if !hasLetter || !hasDigit {
		return fmt.Errorf("password must include at least one letter and one digit")
	}
	return nil
}

// ValidateUsername checks a login name.
func ValidateUsername(username string) error {
	u := strings.TrimSpace(username)
	if u == "" {
		return fmt.Errorf("username required")
	}
	if len(u) > 64 {
		return fmt.Errorf("username too long")
	}
	for _, r := range u {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' || r == '.' {
			continue
		}
		return fmt.Errorf("username may only contain letters, digits, _, -, .")
	}
	return nil
}

// ValidateGroupID checks a user group id slug.
func ValidateGroupID(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("group id required")
	}
	if len(id) > 64 {
		return fmt.Errorf("group id too long")
	}
	for _, r := range id {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' {
			continue
		}
		return fmt.Errorf("group id may only contain letters, digits, _, -")
	}
	return nil
}
