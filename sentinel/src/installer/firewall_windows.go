//go:build windows

package installer

import (
	"fmt"
	"net"
	"os/exec"
	"strconv"
)

const FirewallRuleName = "Remote PC Controller - Sentinel"

// AddFirewallRule creates or updates an inbound allow rule in Windows Firewall
// for the given TCP port and binary path.
func AddFirewallRule(port int, exePath string) error {
	// First remove any existing rule with this name to avoid duplicates
	_ = RemoveFirewallRule()

	cmd := exec.Command(
		"netsh", "advfirewall", "firewall", "add", "rule",
		fmt.Sprintf("name=%s", FirewallRuleName),
		"dir=in",
		"action=allow",
		"protocol=TCP",
		fmt.Sprintf("localport=%d", port),
		fmt.Sprintf("program=%s", exePath),
		fmt.Sprintf("description=Permite conexiones entrantes al agente Sentinel de Remote PC Controller en el puerto %d", port),
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("netsh firewall add rule (%w): %s", err, string(out))
	}
	return nil
}

// RemoveFirewallRule deletes the Sentinel inbound rule from Windows Firewall.
func RemoveFirewallRule() error {
	cmd := exec.Command(
		"netsh", "advfirewall", "firewall", "delete", "rule",
		fmt.Sprintf("name=%s", FirewallRuleName),
	)
	_ = cmd.Run() // Ignore error if rule did not exist
	return nil
}

// ParsePort helper
func ParsePort(portStr string) (int, error) {
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("el puerto %q no es válido: debe ser un número entre 1 y 65535", portStr)
	}
	return port, nil
}

// CheckPortAvailable verifies that a TCP port is free to bind on all network interfaces (0.0.0.0).
func CheckPortAvailable(port int) error {
	addr := fmt.Sprintf("0.0.0.0:%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("el puerto %d ya está en uso por otra aplicación o bloqueado en 0.0.0.0: %w", port, err)
	}
	_ = ln.Close()
	return nil
}
