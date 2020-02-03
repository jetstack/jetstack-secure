package homeutils

import (
	"os/user"
	"path/filepath"
)

// HomeDir returns the path of the home directory
// of the user the application is running as.
func HomeDir() string {
	usr, err := user.Current()
	if err != nil {
		return ""
	}
	return usr.HomeDir
}

// ExpandHome takes a path starting with a '~' and expands this to a
// full path using the home directory of the user the application
// is running as.
func ExpandHome(path string) string {
	if len(path) == 0 {
		return ""
	}

	if path[:2] == "~/" {
		return filepath.Join(HomeDir(), path[2:])
	}
	return path
}
