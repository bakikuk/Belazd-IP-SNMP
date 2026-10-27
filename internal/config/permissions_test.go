package config

import "testing"

func TestPermissionLevelOrder(t *testing.T) {
	p := ModulePermissions{Queues: PermRead, Settings: PermWrite}
	if !p.AtLeast(ModuleQueues, PermRead) {
		t.Fatal("read should satisfy read")
	}
	if p.AtLeast(ModuleQueues, PermWrite) {
		t.Fatal("read should not satisfy write")
	}
	if !p.AtLeast(ModuleSettings, PermRead) {
		t.Fatal("write should satisfy read")
	}
}

func TestParsePermissionLevel(t *testing.T) {
	l, err := ParsePermissionLevel("full")
	if err != nil || l != PermFull {
		t.Fatalf("full: %v err=%v", l, err)
	}
	if _, err := ParsePermissionLevel("bogus"); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidatePasswordPolicy(t *testing.T) {
	if err := ValidatePasswordPolicy("short1"); err == nil {
		t.Fatal("expected too short")
	}
	if err := ValidatePasswordPolicy("allletters"); err == nil {
		t.Fatal("expected no digit")
	}
	if err := ValidatePasswordPolicy("12345678"); err == nil {
		t.Fatal("expected no letter")
	}
	if err := ValidatePasswordPolicy("secret12"); err != nil {
		t.Fatal(err)
	}
}
