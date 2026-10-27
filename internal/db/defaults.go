package db

import (
	"log"
	"net"

	"github.com/bakikuk/belazd-ip-snmp/internal/config"
)

// StandaloneDefaults returns first-run settings for a copy-and-run Linux deploy.
func StandaloneDefaults() *config.Config {
	c := config.Default()
	c.HTTP.Listen = config.FixedHTTPListen
	c.HTTP.Allowed = []string{"0.0.0.0/0"}
	iface := firstNonLoopbackInterface()
	c.Capture.Interface = iface
	c.Capture.Interfaces = []string{iface}
	c.SNMP.Communities = []config.CommunityEntry{{
		Community: "public",
		Allowed:   []string{"127.0.0.1"},
	}}
	c.UserGroups = []config.UserGroup{{
		ID:          config.AdminGroupID,
		Name:        config.AdminGroupName,
		Permissions: config.FullModulePermissions(),
	}}
	return c
}

func firstNonLoopbackInterface() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "eth0"
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}
		if name := iface.Name; name != "" {
			return name
		}
	}
	return "eth0"
}

// ApplyStandaloneDefaults saves defaults when DB has no operational data.
func (s *Store) ApplyStandaloneDefaults() error {
	empty, err := s.IsEmpty()
	if err != nil || !empty {
		return err
	}
	c := StandaloneDefaults()
	log.Printf("belazd: first-run defaults — http.allowed=0.0.0.0/0 (tighten in Settings), capture.interface=%s; add first user in Users (no login until then)", c.Capture.Interface)
	return s.SaveConfig(c)
}
