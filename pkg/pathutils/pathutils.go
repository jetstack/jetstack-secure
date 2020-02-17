package pathutils

import (
	"os/user"
	"path/filepath"
)

// HomeDir returns the home directory of the current user.
func HomeDir() string {
	usr, err := user.Current()
	if err != nil {
		return ""
	}
	return usr.HomeDir
}

// ExpandHome takes a path and converts a leading '~' to the current users home
// directory.
func ExpandHome(path string) string {
	if len(path) == 0 {
		return ""
	}

	if path[:2] == "~/" {
		return filepath.Join(HomeDir(), path[2:])
	}
	return path
}
