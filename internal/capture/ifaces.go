package capture

import (
	"fmt"
	"net"
	"strings"
)

// ListInterfaceNames returns up non-loopback interface names.
func ListInterfaceNames() ([]string, error) {
	ifs, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	var names []string
	for _, iface := range ifs {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		names = append(names, iface.Name)
	}
	return names, nil
}

// FormatInterfaceHint helps when capture.interface is wrong.
func FormatInterfaceHint(wanted string) string {
	names, err := ListInterfaceNames()
	if err != nil || len(names) == 0 {
		return fmt.Sprintf("interface %q not found — run: ip link show", wanted)
	}
	return fmt.Sprintf("interface %q not found — set capture.interface in config.yaml to one of: %s (ip link show)",
		wanted, strings.Join(names, ", "))
}
