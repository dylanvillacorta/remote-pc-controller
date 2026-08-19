//go:build !windows

package installer

func IsAdmin() bool                     { return false }
func AttachConsoleIfCLI()               {}
func IsLaunchedFromExplorer() bool      { return false }
func ElevateSelf(args ...string) error { return nil }
