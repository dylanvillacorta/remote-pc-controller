//go:build windows

package installer

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// IsAdmin reports whether the current process has administrator privileges.
func IsAdmin() bool {
	return isAdmin()
}

var (
	modkernel32        = windows.NewLazySystemDLL("kernel32.dll")
	procAttachConsole  = modkernel32.NewProc("AttachConsole")
)

const attachParentProcess uint32 = ^uint32(0) // (DWORD)-1

// AttachConsoleIfCLI attaches the current process to its parent console (cmd/powershell)
// so that CLI output is visible in the terminal when built with -H windowsgui.
func AttachConsoleIfCLI() {
	r1, _, _ := procAttachConsole.Call(uintptr(attachParentProcess))
	if r1 != 0 {
		conout, err := os.OpenFile("CONOUT$", os.O_RDWR, 0)
		if err == nil {
			os.Stdout = conout
			os.Stderr = conout
		}
	}
}

// IsLaunchedFromExplorer checks if the current process was started directly by Windows Explorer (double click).
func IsLaunchedFromExplorer() bool {
	ppid := os.Getppid()
	if ppid <= 0 {
		return false
	}
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return false
	}
	defer windows.CloseHandle(snapshot)

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	if err := windows.Process32First(snapshot, &entry); err != nil {
		return false
	}
	for {
		if entry.ProcessID == uint32(ppid) {
			name := windows.UTF16ToString(entry.ExeFile[:])
			return strings.EqualFold(name, "explorer.exe")
		}
		if err := windows.Process32Next(snapshot, &entry); err != nil {
			break
		}
	}
	return false
}

// ElevateSelf relaunches the current executable with administrator privileges (UAC prompt).
// It returns an error if ShellExecute fails, or exits the calling process upon success.
func ElevateSelf(args ...string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}

	verbPtr, err := syscall.UTF16PtrFromString("runas")
	if err != nil {
		return err
	}
	exePtr, err := syscall.UTF16PtrFromString(exe)
	if err != nil {
		return err
	}
	argStr := strings.Join(args, " ")
	argPtr, err := syscall.UTF16PtrFromString(argStr)
	if err != nil {
		return err
	}
	cwd, _ := os.Getwd()
	cwdPtr, _ := syscall.UTF16PtrFromString(cwd)

	var showCmd int32 = windows.SW_NORMAL
	err = windows.ShellExecute(0, verbPtr, exePtr, argPtr, cwdPtr, showCmd)
	if err != nil {
		return fmt.Errorf("ShellExecute runas: %w", err)
	}

	// Exit the un-elevated caller
	os.Exit(0)
	return nil
}
