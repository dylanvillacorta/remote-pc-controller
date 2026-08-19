//go:build windows

package installer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"

	"remote-pc-controller/sentinel/src/config"
)

const (
	ServiceName        = "RemotePcController"
	ServiceDisplayName = "Remote PC Controller - Sentinel Agent"
	ServiceDescription = "Sentinel agent for Remote PC Controller. Listens for authenticated commands from the Relay server and executes permitted actions (e.g., hibernate)."
)

// Run dispatches the service subcommand. args should be the arguments after "service".
func Run(args []string) error {
	if len(args) == 0 {
		return LaunchGUI()
	}
	switch strings.ToLower(args[0]) {
	case "gui":
		return LaunchGUI()
	case "install":
		return install()
	case "uninstall":
		return uninstall()
	case "start":
		return start()
	case "stop":
		return stop()
	case "status":
		return status()
	default:
		printUsage()
		return fmt.Errorf("unknown service command: %s", args[0])
	}
}

func printUsage() {
	fmt.Println("Usage: sentinel.exe service <command>")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  gui         Open the graphical installer & service manager")
	fmt.Println("  install     Register Sentinel as a Windows service (requires admin)")
	fmt.Println("  uninstall   Remove Sentinel from Windows services (requires admin)")
	fmt.Println("  start       Start the Sentinel service (requires admin)")
	fmt.Println("  stop        Stop the Sentinel service (requires admin)")
	fmt.Println("  status      Show the current service status (requires admin)")
}

func install() error {
	if !isAdmin() {
		return fmt.Errorf("this command requires administrator privileges — run the terminal as Administrator")
	}

	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}

	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to service manager: %w", err)
	}
	defer m.Disconnect()

	// Load configured port from .env (or default 9876)
	exeDir := filepath.Dir(executable)
	envPath := filepath.Join(exeDir, ".env")
	values, _ := config.LoadWithDefaults(envPath)
	port, _ := ParsePort(values["PORT"])
	if port == 0 {
		port = 9876
	}

	// Check if service already exists; if so, stop and delete it for clean reinstallation
	s, err := m.OpenService(ServiceName)
	if err == nil {
		st, _ := s.Query()
		if st.State != svc.Stopped {
			_, _ = s.Control(svc.Stop)
			_ = waitForState(s, svc.Stopped, 10*time.Second)
		}
		_ = s.Delete()
		s.Close()
	}

	// Check if port is free to bind on all network interfaces (0.0.0.0)
	if err := CheckPortAvailable(port); err != nil {
		return fmt.Errorf("el puerto %d no está disponible en 0.0.0.0: %w", port, err)
	}

	s, err = m.CreateService(ServiceName, executable, mgr.Config{
		DisplayName:  ServiceDisplayName,
		Description:  ServiceDescription,
		StartType:    mgr.StartAutomatic,
		ErrorControl: mgr.ErrorNormal,
	})
	if err != nil {
		return fmt.Errorf("create service: %w", err)
	}
	defer s.Close()

	// Configure recovery actions: restart after 5s, 30s, 60s.
	recoveryActions := []mgr.RecoveryAction{
		{Type: mgr.ServiceRestart, Delay: 5 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 30 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 60 * time.Second},
	}
	if err := s.SetRecoveryActions(recoveryActions, 86400); err != nil {
		fmt.Printf("  warning: could not set recovery actions: %v\n", err)
	}

	// Install the event log source so Windows Event Viewer can display our logs cleanly.
	if err := eventlog.InstallAsEventCreate(ServiceName, eventlog.Error|eventlog.Warning|eventlog.Info); err != nil {
		fmt.Printf("  warning: could not register event log source: %v\n", err)
	}

	// Add Windows Firewall inbound rule for the configured port
	_ = AddFirewallRule(port, executable)

	fmt.Printf("Service %q installed successfully.\n", ServiceName)
	fmt.Println("  Display Name: " + ServiceDisplayName)
	fmt.Println("  Start Type:   Automatic")
	fmt.Println("  Binary:       " + executable)
	fmt.Printf("  Listen Port:  %d (all IPs / 0.0.0.0)\n", port)
	fmt.Println()
	fmt.Println("Run 'sentinel.exe service start' to start the service.")
	return nil
}

func uninstall() error {
	if !isAdmin() {
		return fmt.Errorf("this command requires administrator privileges — run the terminal as Administrator")
	}

	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to service manager: %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(ServiceName)
	if err != nil {
		return fmt.Errorf("service %q is not installed", ServiceName)
	}
	defer s.Close()

	// Try to stop the service if it's running.
	st, err := s.Query()
	if err == nil && st.State != svc.Stopped {
		fmt.Println("Stopping service...")
		if _, err := s.Control(svc.Stop); err != nil {
			fmt.Printf("  warning: could not stop service: %v\n", err)
		} else {
			_ = waitForState(s, svc.Stopped, 15*time.Second)
		}
	}

	if err := s.Delete(); err != nil {
		return fmt.Errorf("delete service: %w", err)
	}

	if err := eventlog.Remove(ServiceName); err != nil {
		fmt.Printf("  warning: could not remove event log source: %v\n", err)
	}

	_ = RemoveFirewallRule()

	fmt.Printf("Service %q uninstalled successfully.\n", ServiceName)
	return nil
}

func start() error {
	if !isAdmin() {
		return fmt.Errorf("this command requires administrator privileges — run the terminal as Administrator")
	}

	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to service manager: %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(ServiceName)
	if err != nil {
		return fmt.Errorf("el servicio %q no está instalado. Haz clic en 'Instalar Servicio' primero.", ServiceName)
	}
	defer s.Close()

	st, err := s.Query()
	if err == nil && st.State == svc.Running {
		return nil
	}

	if err := s.Start(); err != nil {
		return fmt.Errorf("start service: %w", err)
	}

	fmt.Printf("Service %q start signal sent.\n", ServiceName)
	if err := waitForState(s, svc.Running, 15*time.Second); err != nil {
		return fmt.Errorf("el servicio no pasó a estado en ejecución: %w", err)
	}
	return nil
}

func stop() error {
	if !isAdmin() {
		return fmt.Errorf("this command requires administrator privileges — run the terminal as Administrator")
	}

	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to service manager: %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(ServiceName)
	if err != nil {
		return nil // Service not installed, nothing to stop
	}
	defer s.Close()

	st, err := s.Query()
	if err != nil {
		return fmt.Errorf("query service: %w", err)
	}
	if st.State == svc.Stopped {
		return nil
	}

	if _, err := s.Control(svc.Stop); err != nil {
		return fmt.Errorf("stop service: %w", err)
	}

	fmt.Printf("Service %q stop signal sent.\n", ServiceName)
	if err := waitForState(s, svc.Stopped, 15*time.Second); err != nil {
		return fmt.Errorf("el servicio no se detuvo a tiempo: %w", err)
	}
	return nil
}

func status() error {
	if !isAdmin() {
		return fmt.Errorf("this command requires administrator privileges — run the terminal as Administrator")
	}

	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to service manager: %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(ServiceName)
	if err != nil {
		return fmt.Errorf("service %q is not installed", ServiceName)
	}
	defer s.Close()

	st, err := s.Query()
	if err != nil {
		return fmt.Errorf("query service: %w", err)
	}

	cfg, _ := s.Config()

	fmt.Printf("Service: %s\n", ServiceName)
	fmt.Printf("  Display Name: %s\n", cfg.DisplayName)
	fmt.Printf("  Status:       %s\n", stateString(st.State))
	fmt.Printf("  PID:          %d\n", st.ProcessId)
	fmt.Printf("  Start Type:   %s\n", startTypeString(cfg.StartType))
	fmt.Printf("  Binary:       %s\n", cfg.BinaryPathName)
	return nil
}

// waitForState polls the service state until it matches the desired state or times out.
func waitForState(s *mgr.Service, desired svc.State, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		st, err := s.Query()
		if err != nil {
			return fmt.Errorf("query service state: %w", err)
		}
		if st.State == desired {
			return nil
		}
		time.Sleep(300 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for service to reach state %s", stateString(desired))
}

func stateString(state svc.State) string {
	switch state {
	case svc.Stopped:
		return "Stopped"
	case svc.StartPending:
		return "Start Pending"
	case svc.StopPending:
		return "Stop Pending"
	case svc.Running:
		return "Running"
	case svc.ContinuePending:
		return "Continue Pending"
	case svc.PausePending:
		return "Pause Pending"
	case svc.Paused:
		return "Paused"
	default:
		return fmt.Sprintf("Unknown (%d)", state)
	}
}

func startTypeString(startType uint32) string {
	switch startType {
	case mgr.StartAutomatic:
		return "Automatic"
	case mgr.StartManual:
		return "Manual"
	case mgr.StartDisabled:
		return "Disabled"
	default:
		return fmt.Sprintf("Unknown (%d)", startType)
	}
}

// isAdmin checks whether the current process has administrator privileges.
func isAdmin() bool {
	var sid *windows.SID
	err := windows.AllocateAndInitializeSid(
		&windows.SECURITY_NT_AUTHORITY,
		2,
		windows.SECURITY_BUILTIN_DOMAIN_RID,
		windows.DOMAIN_ALIAS_RID_ADMINS,
		0, 0, 0, 0, 0, 0,
		&sid,
	)
	if err != nil {
		return false
	}
	defer windows.FreeSid(sid)

	token := windows.Token(0)
	member, err := token.IsMember(sid)
	if err != nil {
		return false
	}
	return member
}
