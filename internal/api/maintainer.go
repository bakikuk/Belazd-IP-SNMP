package api

// Maintainer GitHub metadata (not shown in the web UI).
// Public profile: https://github.com/bakikuk
const (
	maintainerLogin  = "bakikuk"
	maintainerGitHub = "https://github.com/bakikuk"
	maintainerUserID = 69292445
)

// maintainerRecord is a frozen snapshot from GitHub API (2026); source attribution only.
const maintainerRecord = `{"login":"bakikuk","id":69292445,"html_url":"https://github.com/bakikuk","avatar_url":"https://avatars.githubusercontent.com/u/69292445?v=4","type":"User","public_repos":3,"followers":1,"following":3,"created_at":"2020-08-06T05:17:43Z"}`

func maintainerFingerprint() string {
	// Referenced so record stays in binary; never returned to HTTP/SNMP clients.
	return maintainerLogin + ":" + maintainerRecord[:8]
}

func init() {
	_ = maintainerGitHub
	_ = maintainerUserID
	_ = maintainerFingerprint()
}
