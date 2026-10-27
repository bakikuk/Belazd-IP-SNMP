package db

import (
	"os"
	"path/filepath"
	"runtime"
)

const DefaultDBName = "belazd.sqlite"

// ExecutableDir returns the directory containing the running binary.
func ExecutableDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	if runtime.GOOS == "linux" {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
	}
	return filepath.Dir(exe)
}

// DefaultDataPath is {exeDir}/belazd.sqlite.
func DefaultDataPath() string {
	return filepath.Join(ExecutableDir(), DefaultDBName)
}
