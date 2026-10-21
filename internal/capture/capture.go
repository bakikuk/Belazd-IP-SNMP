package capture

// Runner captures packets into the accounting store.
type Runner interface {
	Start() error
	Stop()
}

// Config for capture.
type Config struct {
	Interface    string
	Snaplen      int
	ForceWide    bool
	BPFFragments []string
	Simulate     bool
}
