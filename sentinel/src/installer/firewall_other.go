//go:build !windows

package installer

func AddFirewallRule(port int, exePath string) error { return nil }
func RemoveFirewallRule() error                       { return nil }
func ParsePort(portStr string) (int, error)           { return 9876, nil }
func CheckPortAvailable(port int) error               { return nil }
