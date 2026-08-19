//go:build !windows

package installer

import "fmt"

func ShowErrorDialog(msg string) {}

func LaunchGUI() error {
	return fmt.Errorf("GUI installer is only supported on Windows")
}
