//go:build windows

package notify

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	wtsapi32               = windows.NewLazySystemDLL("wtsapi32.dll")
	kernel32               = windows.NewLazySystemDLL("kernel32.dll")
	procWTSSendMessageW    = wtsapi32.NewProc("WTSSendMessageW")
	procWTSGetActiveConsole = kernel32.NewProc("WTSGetActiveConsoleSessionId")
)

const (
	MB_OK            = 0x00000000
	MB_ICONWARNING   = 0x00000030
	MB_ICONINFO      = 0x00000040
	MB_TOPMOST       = 0x00040000
	MB_SETFOREGROUND = 0x00010000
)

type windowsNotifier struct{}

// NewWindowsNotifier creates a Notifier targeting active Windows desktop sessions.
func NewWindowsNotifier() Notifier {
	return &windowsNotifier{}
}

func (w *windowsNotifier) NotifyValidationFailure(commandID, reason string) {
	title := "Sentinel - Solicitud Rechazada"
	msg := fmt.Sprintf(
		"⚠️ Se rechazó una solicitud remota:\n\n"+
			"Motivo: %s\n"+
			"ID de Comando: %s\n\n"+
			"Verifica que el Relay use la misma clave privada y DEVICE_ID.",
		reason, commandID,
	)
	go sendWTSMessage(title, msg, MB_OK|MB_ICONWARNING|MB_TOPMOST|MB_SETFOREGROUND)
}

func (w *windowsNotifier) NotifyActionExecuted(action, deviceID string) {
	title := "Sentinel - Acción Ejecutada"
	msg := fmt.Sprintf(
		"🚀 Se validó y ejecutó la orden remota:\n\n"+
			"Acción: %s\n"+
			"Dispositivo: %s",
		action, deviceID,
	)
	go sendWTSMessage(title, msg, MB_OK|MB_ICONINFO|MB_TOPMOST|MB_SETFOREGROUND)
}

func sendWTSMessage(title, message string, style uint32) {
	sessionID, _, _ := procWTSGetActiveConsole.Call()
	if sessionID == 0xFFFFFFFF {
		return // No active console session logged in
	}

	titleUTF16, err := syscall.UTF16FromString(title)
	if err != nil {
		return
	}
	msgUTF16, err := syscall.UTF16FromString(message)
	if err != nil {
		return
	}

	titleLen := uint32(len(titleUTF16) * 2)
	msgLen := uint32(len(msgUTF16) * 2)
	var response uint32

	// Non-blocking asynchronous message box on user screen with 15s timeout
	_, _, _ = procWTSSendMessageW.Call(
		0, // WTS_CURRENT_SERVER_HANDLE
		sessionID,
		uintptr(unsafe.Pointer(&titleUTF16[0])),
		uintptr(titleLen),
		uintptr(unsafe.Pointer(&msgUTF16[0])),
		uintptr(msgLen),
		uintptr(style),
		uintptr(15), // 15 seconds timeout
		uintptr(unsafe.Pointer(&response)),
		0, // bWait = FALSE (asynchronous)
	)
}
