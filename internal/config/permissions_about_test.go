package config

import "testing"

func TestBackfillLegacyAbout(t *testing.T) {
	m := ModulePermissions{Queues: PermFull, Settings: PermFull, Users: PermFull, UserGroups: PermFull}
	m.BackfillLegacyAbout()
	if m.About != PermFull {
		t.Fatalf("about=%v", m.About)
	}
	m2 := ModulePermissions{Queues: PermRead}
	m2.BackfillLegacyAbout()
	if m2.About != PermRead {
		t.Fatalf("about=%v", m2.About)
	}
	m3 := ModulePermissions{}
	m3.BackfillLegacyAbout()
	if m3.About != PermNone {
		t.Fatalf("about=%v", m3.About)
	}
}
