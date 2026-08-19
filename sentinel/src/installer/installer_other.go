//go:build !windows

package installer

import "fmt"

// Run is a no-op on non-Windows platforms.
func Run(args []string) error {
	return fmt.Errorf("service management is only supported on Windows")
}
