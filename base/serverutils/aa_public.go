// serverutils contains small helpers often used in http servers setup/testing
package serverutils

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
)

// Returns a port not in use that can be used to run a server
func GetFreePort(t testing.TB) int {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

// Used to adjust a dirPath used by servers
func AdjustServerDirOrDie(dirPath *string, defaultDirName string) {
	if *dirPath == "" {
		currentPath, err := os.Getwd()
		if err != nil {
			panic(fmt.Sprintf("unable to get current working dir: %s", err))
		}
		d := filepath.Join(currentPath, defaultDirName)
		*dirPath = d
	}
	if !filepath.IsAbs(*dirPath) {
		panic(
			fmt.Sprintf("%s is not an absolute path to a directory",
				*dirPath))
	}
	d, err := os.Stat(*dirPath)
	if err == nil && !d.IsDir() {
		panic(fmt.Sprintf("%s must be a directory, but is a file",
			*dirPath))
	}
	if err != nil {
		if os.IsNotExist(err) {
			// Create directory with 0700 permissions.
			//
			// - Owner (the app/user running this code) can read, write, and
			// enter the directory.
			// - Group and Others have no permissions at all.
			//
			// This makes the directory private, which is recommended if it
			// stores sensitive data (tokens, configs, user-specific files).
			//
			// Note: On Linux/macOS the actual permissions may still be
			// restricted further by the system umask, but never loosened.
			// On Windows, the numeric mode is mostly ignored, but MkdirAll
			// still works as expected.
			err = os.MkdirAll(*dirPath, 0700)
			if err != nil {
				panic(fmt.Sprintf("failed to create %v: %s", *dirPath, err))
			}
		} else {
			panic(fmt.Sprintf("failed to start %s", *dirPath))
		}
	}
}
