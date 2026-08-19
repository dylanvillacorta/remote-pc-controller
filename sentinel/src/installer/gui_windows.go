//go:build windows

package installer

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"

	"remote-pc-controller/sentinel/src/config"
)

const IDYES = 6

// Win32 Constants
const (
	WM_CREATE         = 0x0001
	WM_DESTROY        = 0x0002
	WM_SETFONT        = 0x0030
	WM_COMMAND        = 0x0111
	WM_TIMER          = 0x0113
	WM_CTLCOLORSTATIC = 0x0138
	WM_CTLCOLOREDIT   = 0x0133
	WM_CTLCOLORBTN    = 0x0135

	WS_OVERLAPPED  = 0x00000000
	WS_CAPTION     = 0x00C00000
	WS_SYSMENU     = 0x00080000
	WS_MINIMIZEBOX = 0x00020000
	WS_VISIBLE     = 0x10000000
	WS_CHILD       = 0x40000000
	WS_TABSTOP     = 0x00010000
	WS_BORDER      = 0x00800000
	WS_VSCROLL     = 0x00200000

	WS_EX_CLIENTEDGE = 0x00000200

	ES_NUMBER      = 0x2000
	ES_MULTILINE   = 0x0004
	ES_AUTOVSCROLL = 0x0040
	ES_WANTRETURN  = 0x1000

	EM_SETLIMITTEXT = 0x00C5
	EN_CHANGE       = 0x0300

	BS_PUSHBUTTON   = 0x0000
	BS_AUTOCHECKBOX = 0x0003
	BS_GROUPBOX     = 0x0007

	BM_GETCHECK = 0x00F0
	BM_SETCHECK = 0x00F1
	BST_CHECKED = 1

	SW_SHOW = 5

	CF_UNICODETEXT = 13

	OFN_FILEMUSTEXIST = 0x00001000
	OFN_PATHMUSTEXIST = 0x00000800

	// Control IDs
	ID_BTN_INSTALL   = 101
	ID_BTN_START     = 102
	ID_BTN_STOP      = 103
	ID_BTN_UNINSTALL = 104
	ID_BTN_LOAD_FILE = 105
	ID_BTN_PASTE     = 106

	ID_TXT_PORT    = 201
	ID_TXT_DEV     = 202
	ID_TXT_SKEW    = 203
	ID_TXT_KEY     = 204
	ID_CHK_FW      = 205
	ID_CHK_NOTIFY  = 206
)

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	gdi32    = windows.NewLazySystemDLL("gdi32.dll")
	comdlg32 = windows.NewLazySystemDLL("comdlg32.dll")

	procRegisterClassExW     = user32.NewProc("RegisterClassExW")
	procCreateWindowExW      = user32.NewProc("CreateWindowExW")
	procDefWindowProcW       = user32.NewProc("DefWindowProcW")
	procShowWindow           = user32.NewProc("ShowWindow")
	procUpdateWindow         = user32.NewProc("UpdateWindow")
	procGetMessageW          = user32.NewProc("GetMessageW")
	procTranslateMessage     = user32.NewProc("TranslateMessage")
	procDispatchMessageW     = user32.NewProc("DispatchMessageW")
	procPostQuitMessage      = user32.NewProc("PostQuitMessage")
	procSendMessageW         = user32.NewProc("SendMessageW")
	procSetWindowTextW       = user32.NewProc("SetWindowTextW")
	procGetWindowTextW       = user32.NewProc("GetWindowTextW")
	procGetWindowTextLengthW = user32.NewProc("GetWindowTextLengthW")
	procEnableWindow         = user32.NewProc("EnableWindow")
	procSetTimer             = user32.NewProc("SetTimer")
	procKillTimer            = user32.NewProc("KillTimer")
	procOpenClipboard        = user32.NewProc("OpenClipboard")
	procCloseClipboard       = user32.NewProc("CloseClipboard")
	procGetClipboardData     = user32.NewProc("GetClipboardData")
	procGlobalLock           = windows.NewLazySystemDLL("kernel32.dll").NewProc("GlobalLock")
	procGlobalUnlock         = windows.NewLazySystemDLL("kernel32.dll").NewProc("GlobalUnlock")

	procCreateFontW      = gdi32.NewProc("CreateFontW")
	procCreateSolidBrush = gdi32.NewProc("CreateSolidBrush")
	procSetBkMode        = gdi32.NewProc("SetBkMode")
	procGetOpenFileNameW = comdlg32.NewProc("GetOpenFileNameW")
)

type WNDCLASSEXW struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     windows.Handle
	HIcon         windows.Handle
	HCursor       windows.Handle
	HbrBackground windows.Handle
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       windows.Handle
}

type MSG struct {
	Hwnd    windows.Handle
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      struct{ X, Y int32 }
}

type OPENFILENAMEW struct {
	StructSize      uint32
	HwndOwner       windows.Handle
	Instance        windows.Handle
	Filter          *uint16
	CustomFilter    *uint16
	MaxCustFilter   uint32
	FilterIndex     uint32
	File            *uint16
	MaxFile         uint32
	FileTitle       *uint16
	MaxFileTitle    uint32
	InitialDir      *uint16
	Title           *uint16
	Flags           uint32
	FileOffset      uint16
	FileExtension   uint16
	DefExt          *uint16
	CustData        uintptr
	Hook            uintptr
	TemplateName    *uint16
	PvReserved      uintptr
	DwReserved      uint32
	FlagsEx         uint32
}

// GUI state
type guiState struct {
	hMainWnd   windows.Handle
	hFont      windows.Handle
	hFontSmall windows.Handle
	hFontBold  windows.Handle
	hFontTitle windows.Handle
	hFontMono  windows.Handle

	hStatusPill    windows.Handle
	hTxtPort       windows.Handle
	hLblPortStatus windows.Handle
	hTxtDev        windows.Handle
	hTxtSkew       windows.Handle
	hChkFw         windows.Handle
	hChkNotify     windows.Handle
	hTxtKey        windows.Handle
	hLblLog        windows.Handle

	hBtnInstall   windows.Handle
	hBtnStart     windows.Handle
	hBtnStop      windows.Handle
	hBtnUninstall windows.Handle

	bgBrush windows.Handle

	exePath        string
	envPath        string
	serviceRunning bool
	servicePID     int
}

var gState guiState

// ShowErrorDialog displays a native Windows MessageBox with the error.
func ShowErrorDialog(msg string) {
	messageBox(0, msg, "Sentinel - Error", windows.MB_ICONERROR|windows.MB_OK)
}

func messageBox(parent windows.Handle, msg, title string, flags uint32) int32 {
	titlePtr, _ := syscall.UTF16PtrFromString(title)
	msgPtr, _ := syscall.UTF16PtrFromString(msg)
	ret, _ := windows.MessageBox(windows.HWND(parent), msgPtr, titlePtr, flags)
	return ret
}

// LaunchGUI creates and runs the 100% native Win32 graphical installer window.
func LaunchGUI() error {
	if !IsAdmin() {
		return ElevateSelf("service", "gui")
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}
	exeDir := filepath.Dir(executable)
	envPath := filepath.Join(exeDir, ".env")

	gState.exePath = executable
	gState.envPath = envPath

	// Load defaults or existing values
	values, _ := config.LoadWithDefaults(envPath)
	if values["PUBLIC_KEY"] == "" {
		keysPaths := []string{
			filepath.Join(exeDir, "keys.txt"),
			filepath.Join(exeDir, "..", "scripts", "keys.txt"),
			filepath.Join(exeDir, "scripts", "keys.txt"),
		}
		for _, kp := range keysPaths {
			if data, err := os.ReadFile(kp); err == nil {
				keyStr := extractPublicKeyFromText(string(data))
				if keyStr != "" {
					values["PUBLIC_KEY"] = keyStr
					break
				}
			}
		}
	}

	className, _ := syscall.UTF16PtrFromString("SentinelInstallerGUI")
	windowTitle, _ := syscall.UTF16PtrFromString("Remote PC Controller - Instalador de Sentinel")

	gState.bgBrush = createSolidBrush(0xF4F0EC) // Soft gray

	wc := WNDCLASSEXW{
		CbSize:        uint32(unsafe.Sizeof(WNDCLASSEXW{})),
		Style:         0,
		LpfnWndProc:   syscall.NewCallback(wndProc),
		HInstance:     0,
		HbrBackground: gState.bgBrush,
		LpszClassName: className,
	}

	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	// Create fonts
	gState.hFont = createFont("Segoe UI", 15, false)
	gState.hFontSmall = createFont("Segoe UI", 13, false)
	gState.hFontBold = createFont("Segoe UI", 15, true)
	gState.hFontTitle = createFont("Segoe UI", 20, true)
	gState.hFontMono = createFont("Consolas", 13, false)

	windowStyle := uint32(WS_OVERLAPPED | WS_CAPTION | WS_SYSMENU | WS_MINIMIZEBOX | WS_VISIBLE)

	width := int32(640)
	height := int32(760)
	screenWidth := getSystemMetrics(0)
	screenHeight := getSystemMetrics(1)
	posX := (screenWidth - width) / 2
	posY := (screenHeight - height) / 2

	hMain, _, err := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windowTitle)),
		uintptr(windowStyle),
		uintptr(posX),
		uintptr(posY),
		uintptr(width),
		uintptr(height),
		0, 0, 0, 0,
	)
	if hMain == 0 {
		return fmt.Errorf("CreateWindowExW failed: %w", err)
	}

	gState.hMainWnd = windows.Handle(hMain)

	createControls(gState.hMainWnd, values)
	updateServiceStatus()
	validatePortLive()

	// Periodic timer to keep status updated every 1.5 seconds
	procSetTimer.Call(uintptr(gState.hMainWnd), 1, 1500, 0)

	procShowWindow.Call(hMain, uintptr(SW_SHOW))
	procUpdateWindow.Call(hMain)

	var msg MSG
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if r == 0 || int32(r) == -1 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}

	procKillTimer.Call(uintptr(gState.hMainWnd), 1)
	return nil
}

func createControls(hwnd windows.Handle, values map[string]string) {
	// Header Title
	createLabel(hwnd, "Sentinel Agent - Control de Servicio", 20, 16, 580, 26, gState.hFontTitle)
	createLabel(hwnd, "Configura e instala el agente para inicio automático como Servicio de Windows", 20, 44, 580, 18, gState.hFont)

	// Status Group
	createGroupBox(hwnd, " Estado del Servicio ", 20, 70, 584, 55, gState.hFontBold)
	createLabel(hwnd, "Estado actual:", 36, 92, 100, 20, gState.hFontBold)
	gState.hStatusPill = createLabel(hwnd, "Consultando...", 140, 92, 440, 20, gState.hFontBold)

	// Configuration Group
	createGroupBox(hwnd, " Parámetros de Configuración (.env) ", 20, 134, 584, 434, gState.hFontBold)

	createLabel(hwnd, "Puerto de Escucha:", 36, 158, 140, 18, gState.hFont)
	// Create Port TextBox with ES_NUMBER so only digits (0-9) can be typed
	gState.hTxtPort = createNumericEdit(hwnd, values["PORT"], 36, 178, 110, 24, ID_TXT_PORT, gState.hFont, 5)

	// Live Port Status Indicator Label
	gState.hLblPortStatus = createLabel(hwnd, "Comprobando...", 156, 180, 420, 20, gState.hFontSmall)

	createLabel(hwnd, "Device ID (Nombre en Relay):", 36, 208, 220, 18, gState.hFont)
	gState.hTxtDev = createEdit(hwnd, values["DEVICE_ID"], 36, 228, 240, 24, ID_TXT_DEV, gState.hFont)

	createLabel(hwnd, "Reloj Skew (seg):", 300, 208, 140, 18, gState.hFont)
	gState.hTxtSkew = createNumericEdit(hwnd, values["CLOCK_SKEW_SECONDS"], 300, 228, 120, 24, ID_TXT_SKEW, gState.hFont, 4)

	gState.hChkFw = createCheckBox(hwnd, "Abrir regla de entrada en el Firewall de Windows automáticamente", 36, 260, 540, 22, ID_CHK_FW, gState.hFont, true)

	notifChecked := true
	if values["ENABLE_NOTIFICATIONS"] == "false" {
		notifChecked = false
	}
	gState.hChkNotify = createCheckBox(hwnd, "Mostrar notificaciones de escritorio en fallos de validación y eventos", 36, 286, 540, 22, ID_CHK_NOTIFY, gState.hFont, notifChecked)

	createLabel(hwnd, "Clave Pública RSA (PEM):", 36, 314, 200, 18, gState.hFont)
	createButton(hwnd, "📂 Cargar Archivo...", 340, 310, 130, 24, ID_BTN_LOAD_FILE, gState.hFont)
	createButton(hwnd, "📋 Pegar", 480, 310, 95, 24, ID_BTN_PASTE, gState.hFont)

	gState.hTxtKey = createMultilineEdit(hwnd, values["PUBLIC_KEY"], 36, 338, 550, 214, ID_TXT_KEY, gState.hFontMono)

	// Action Buttons
	gState.hBtnInstall = createButton(hwnd, "🚀 Instalar Servicio", 20, 582, 175, 42, ID_BTN_INSTALL, gState.hFontBold)
	gState.hBtnStart = createButton(hwnd, "▶ Iniciar", 205, 582, 115, 42, ID_BTN_START, gState.hFontBold)
	gState.hBtnStop = createButton(hwnd, "⏹ Detener", 330, 582, 115, 42, ID_BTN_STOP, gState.hFontBold)
	gState.hBtnUninstall = createButton(hwnd, "🗑 Desinstalar", 455, 582, 149, 42, ID_BTN_UNINSTALL, gState.hFontBold)

	// Log Bar
	gState.hLblLog = createLabel(hwnd, "Listo.", 20, 636, 584, 30, gState.hFont)
}

func wndProc(hwnd windows.Handle, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_COMMAND:
		id := int(wParam & 0xFFFF)
		code := int((wParam >> 16) & 0xFFFF)
		if code == 0 { // Button Click
			handleButtonClick(id)
		} else if code == EN_CHANGE && id == ID_TXT_PORT {
			validatePortLive()
		}
		return 0

	case WM_TIMER:
		updateServiceStatus()
		return 0

	case WM_CTLCOLORSTATIC:
		hdc := windows.Handle(wParam)
		procSetBkMode.Call(uintptr(hdc), 1) // TRANSPARENT
		return uintptr(gState.bgBrush)

	case WM_DESTROY:
		procPostQuitMessage.Call(0)
		return 0
	}

	r, _, _ := procDefWindowProcW.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
	return r
}

func validatePortLive() {
	portStr := strings.TrimSpace(getWindowText(gState.hTxtPort))
	if portStr == "" {
		setWindowText(gState.hLblPortStatus, "⚠️ Ingresa un puerto (1-65535)")
		return
	}

	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		setWindowText(gState.hLblPortStatus, "❌ Puerto inválido (rango permitido: 1 a 65535)")
		return
	}

	// If our own Sentinel service is currently running, it's bound to this port
	if gState.serviceRunning {
		setWindowText(gState.hLblPortStatus, fmt.Sprintf("🟢 En uso activo por este servicio (0.0.0.0:%d)", port))
		return
	}

	// Check if port is free to bind on all IPs
	if err := CheckPortAvailable(port); err != nil {
		setWindowText(gState.hLblPortStatus, fmt.Sprintf("❌ Ocupado por otra app en 0.0.0.0:%d", port))
		return
	}

	setWindowText(gState.hLblPortStatus, fmt.Sprintf("✅ Puerto %d disponible en todas las IPs (0.0.0.0)", port))
}

func handleButtonClick(id int) {
	switch id {
	case ID_BTN_LOAD_FILE:
		filePath := openFileDialog(gState.hMainWnd)
		if filePath != "" {
			if data, err := os.ReadFile(filePath); err == nil {
				keyStr := extractPublicKeyFromText(string(data))
				if keyStr == "" {
					keyStr = strings.TrimSpace(string(data))
				}
				setWindowText(gState.hTxtKey, keyStr)
				setLog(fmt.Sprintf("Archivo cargado: %s", filepath.Base(filePath)))
			}
		}

	case ID_BTN_PASTE:
		text := getClipboardText()
		if text != "" {
			setWindowText(gState.hTxtKey, text)
			setLog("Clave pegada desde el portapapeles.")
		}

	case ID_BTN_INSTALL:
		portStr := strings.TrimSpace(getWindowText(gState.hTxtPort))
		port, err := ParsePort(portStr)
		if err != nil {
			messageBox(gState.hMainWnd, fmt.Sprintf("Puerto inválido: %v", err), "Puerto Inválido", windows.MB_ICONWARNING|windows.MB_OK)
			return
		}

		pubKey := strings.TrimSpace(getWindowText(gState.hTxtKey))
		if pubKey == "" {
			messageBox(gState.hMainWnd, "Por favor ingresa o carga la Clave Pública RSA antes de instalar.", "Clave Requerida", windows.MB_ICONWARNING|windows.MB_OK)
			return
		}
		if _, err := config.ValidatePublicKeyPEM(pubKey); err != nil {
			messageBox(gState.hMainWnd, fmt.Sprintf("La Clave Pública RSA proporcionada no es válida:\n\n%v", err), "Clave RSA Inválida", windows.MB_ICONWARNING|windows.MB_OK)
			return
		}

		// Stop any currently running instance before validating and reinstalling
		_ = stop()

		// Verify that the port is free on all interfaces (0.0.0.0)
		if err := CheckPortAvailable(port); err != nil {
			messageBox(gState.hMainWnd, fmt.Sprintf("El puerto %d no está disponible para escuchar en todas las IPs (0.0.0.0):\n\n%v\n\nPor favor selecciona otro puerto libre o cierra la aplicación que lo esté utilizando.", port, err), "Puerto No Disponible", windows.MB_ICONERROR|windows.MB_OK)
			return
		}

		setLog(fmt.Sprintf("Puerto %d verificado. Guardando e instalando servicio...", port))
		saveCurrentEnv()

		if err := install(); err != nil {
			setLog(fmt.Sprintf("Error al instalar: %v", err))
			messageBox(gState.hMainWnd, err.Error(), "Error de Instalación", windows.MB_ICONERROR|windows.MB_OK)
			return
		}

		if isChecked(gState.hChkFw) {
			_ = AddFirewallRule(port, gState.exePath)
		}

		// Auto start after install
		if err := start(); err != nil {
			setLog(fmt.Sprintf("Servicio instalado pero no se pudo iniciar: %v", err))
		} else {
			setLog(fmt.Sprintf("¡Servicio instalado y escuchando en 0.0.0.0:%d!", port))
		}
		updateServiceStatus()
		validatePortLive()

	case ID_BTN_START:
		portStr := strings.TrimSpace(getWindowText(gState.hTxtPort))
		port, _ := ParsePort(portStr)
		if port > 0 {
			if err := CheckPortAvailable(port); err != nil {
				messageBox(gState.hMainWnd, fmt.Sprintf("No se puede iniciar: el puerto %d no está disponible en 0.0.0.0:\n\n%v", port, err), "Puerto En Uso", windows.MB_ICONERROR|windows.MB_OK)
				return
			}
		}

		// Save current config before starting
		saveCurrentEnv()
		if isChecked(gState.hChkFw) && port > 0 {
			_ = AddFirewallRule(port, gState.exePath)
		}

		setLog("Iniciando servicio...")
		if err := start(); err != nil {
			setLog(fmt.Sprintf("Error al iniciar: %v", err))
			messageBox(gState.hMainWnd, err.Error(), "Error al Iniciar Servicio", windows.MB_ICONERROR|windows.MB_OK)
		} else {
			setLog(fmt.Sprintf("Servicio iniciado correctamente (escuchando en 0.0.0.0:%d).", port))
		}
		updateServiceStatus()
		validatePortLive()

	case ID_BTN_STOP:
		setLog("Deteniendo servicio...")
		if err := stop(); err != nil {
			setLog(fmt.Sprintf("Error al detener: %v", err))
			messageBox(gState.hMainWnd, err.Error(), "Error al Detener Servicio", windows.MB_ICONERROR|windows.MB_OK)
		} else {
			setLog("Servicio detenido.")
		}
		updateServiceStatus()
		validatePortLive()

	case ID_BTN_UNINSTALL:
		if messageBox(gState.hMainWnd, "¿Estás seguro de que deseas desinstalar el servicio Sentinel?", "Confirmar Desinstalación", windows.MB_YESNO|windows.MB_ICONQUESTION) == IDYES {
			setLog("Desinstalando servicio...")
			if err := uninstall(); err != nil {
				setLog(fmt.Sprintf("Error al desinstalar: %v", err))
				messageBox(gState.hMainWnd, err.Error(), "Error de Desinstalación", windows.MB_ICONERROR|windows.MB_OK)
			} else {
				setLog("Servicio desinstalado correctamente.")
			}
			updateServiceStatus()
			validatePortLive()
		}
	}
}

func saveCurrentEnv() {
	port := getWindowText(gState.hTxtPort)
	dev := getWindowText(gState.hTxtDev)
	skew := getWindowText(gState.hTxtSkew)
	key := getWindowText(gState.hTxtKey)

	notifVal := "true"
	if !isChecked(gState.hChkNotify) {
		notifVal = "false"
	}

	values := map[string]string{
		"PORT":                 port,
		"DEVICE_ID":            dev,
		"CLOCK_SKEW_SECONDS":   skew,
		"PUBLIC_KEY":           key,
		"ENABLE_NOTIFICATIONS": notifVal,
	}
	_ = config.Save(gState.envPath, values)
}

func updateServiceStatus() {
	m, err := mgr.Connect()
	if err != nil {
		setWindowText(gState.hStatusPill, "⚪ Acceso SCM requerido")
		enableControl(gState.hBtnInstall, true)
		enableControl(gState.hBtnStart, false)
		enableControl(gState.hBtnStop, false)
		enableControl(gState.hBtnUninstall, false)
		gState.serviceRunning = false
		return
	}
	defer m.Disconnect()

	s, err := m.OpenService(ServiceName)
	if err != nil {
		setWindowText(gState.hStatusPill, "⚪ No Instalado")
		enableControl(gState.hBtnInstall, true)
		enableControl(gState.hBtnStart, false)
		enableControl(gState.hBtnStop, false)
		enableControl(gState.hBtnUninstall, false)
		gState.serviceRunning = false
		return
	}
	defer s.Close()

	st, err := s.Query()
	if err != nil {
		setWindowText(gState.hStatusPill, "⚪ Error al consultar estado")
		return
	}

	switch st.State {
	case svc.Running:
		gState.serviceRunning = true
		gState.servicePID = int(st.ProcessId)
		pidText := ""
		if st.ProcessId > 0 {
			pidText = fmt.Sprintf(" (PID: %d)", st.ProcessId)
		}
		setWindowText(gState.hStatusPill, "🟢 En Ejecución"+pidText)
		enableControl(gState.hBtnInstall, true)
		enableControl(gState.hBtnStart, false)
		enableControl(gState.hBtnStop, true)
		enableControl(gState.hBtnUninstall, true)

	case svc.Stopped:
		gState.serviceRunning = false
		gState.servicePID = 0
		setWindowText(gState.hStatusPill, "🔴 Detenido")
		enableControl(gState.hBtnInstall, true)
		enableControl(gState.hBtnStart, true)
		enableControl(gState.hBtnStop, false)
		enableControl(gState.hBtnUninstall, true)

	default:
		gState.serviceRunning = false
		setWindowText(gState.hStatusPill, fmt.Sprintf("🟡 %s", stateString(st.State)))
		enableControl(gState.hBtnInstall, true)
		enableControl(gState.hBtnStart, false)
		enableControl(gState.hBtnStop, true)
		enableControl(gState.hBtnUninstall, true)
	}
}

func setLog(msg string) {
	setWindowText(gState.hLblLog, fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), msg))
}

// Win32 Helper functions
func createLabel(parent windows.Handle, text string, x, y, w, h int32, font windows.Handle) windows.Handle {
	className, _ := syscall.UTF16PtrFromString("STATIC")
	textPtr, _ := syscall.UTF16PtrFromString(text)
	hwnd, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(textPtr)),
		uintptr(WS_CHILD|WS_VISIBLE),
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		uintptr(parent), 0, 0, 0,
	)
	if font != 0 {
		procSendMessageW.Call(hwnd, WM_SETFONT, uintptr(font), 1)
	}
	return windows.Handle(hwnd)
}

func createGroupBox(parent windows.Handle, text string, x, y, w, h int32, font windows.Handle) windows.Handle {
	className, _ := syscall.UTF16PtrFromString("BUTTON")
	textPtr, _ := syscall.UTF16PtrFromString(text)
	hwnd, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(textPtr)),
		uintptr(WS_CHILD|WS_VISIBLE|BS_GROUPBOX),
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		uintptr(parent), 0, 0, 0,
	)
	if font != 0 {
		procSendMessageW.Call(hwnd, WM_SETFONT, uintptr(font), 1)
	}
	return windows.Handle(hwnd)
}

func createEdit(parent windows.Handle, text string, x, y, w, h int32, id int, font windows.Handle) windows.Handle {
	className, _ := syscall.UTF16PtrFromString("EDIT")
	textPtr, _ := syscall.UTF16PtrFromString(text)
	hwnd, _, _ := procCreateWindowExW.Call(
		uintptr(WS_EX_CLIENTEDGE),
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(textPtr)),
		uintptr(WS_CHILD|WS_VISIBLE|WS_TABSTOP|WS_BORDER),
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		uintptr(parent), uintptr(id), 0, 0,
	)
	if font != 0 {
		procSendMessageW.Call(hwnd, WM_SETFONT, uintptr(font), 1)
	}
	return windows.Handle(hwnd)
}

func createNumericEdit(parent windows.Handle, text string, x, y, w, h int32, id int, font windows.Handle, maxLen int) windows.Handle {
	className, _ := syscall.UTF16PtrFromString("EDIT")
	textPtr, _ := syscall.UTF16PtrFromString(text)
	hwnd, _, _ := procCreateWindowExW.Call(
		uintptr(WS_EX_CLIENTEDGE),
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(textPtr)),
		uintptr(WS_CHILD|WS_VISIBLE|WS_TABSTOP|WS_BORDER|ES_NUMBER),
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		uintptr(parent), uintptr(id), 0, 0,
	)
	if font != 0 {
		procSendMessageW.Call(hwnd, WM_SETFONT, uintptr(font), 1)
	}
	if maxLen > 0 {
		procSendMessageW.Call(hwnd, EM_SETLIMITTEXT, uintptr(maxLen), 0)
	}
	return windows.Handle(hwnd)
}

func createMultilineEdit(parent windows.Handle, text string, x, y, w, h int32, id int, font windows.Handle) windows.Handle {
	className, _ := syscall.UTF16PtrFromString("EDIT")
	textPtr, _ := syscall.UTF16PtrFromString(text)
	hwnd, _, _ := procCreateWindowExW.Call(
		uintptr(WS_EX_CLIENTEDGE),
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(textPtr)),
		uintptr(WS_CHILD|WS_VISIBLE|WS_TABSTOP|WS_BORDER|WS_VSCROLL|ES_MULTILINE|ES_AUTOVSCROLL|ES_WANTRETURN),
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		uintptr(parent), uintptr(id), 0, 0,
	)
	if font != 0 {
		procSendMessageW.Call(hwnd, WM_SETFONT, uintptr(font), 1)
	}
	return windows.Handle(hwnd)
}

func createButton(parent windows.Handle, text string, x, y, w, h int32, id int, font windows.Handle) windows.Handle {
	className, _ := syscall.UTF16PtrFromString("BUTTON")
	textPtr, _ := syscall.UTF16PtrFromString(text)
	hwnd, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(textPtr)),
		uintptr(WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON),
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		uintptr(parent), uintptr(id), 0, 0,
	)
	if font != 0 {
		procSendMessageW.Call(hwnd, WM_SETFONT, uintptr(font), 1)
	}
	return windows.Handle(hwnd)
}

func createCheckBox(parent windows.Handle, text string, x, y, w, h int32, id int, font windows.Handle, checked bool) windows.Handle {
	className, _ := syscall.UTF16PtrFromString("BUTTON")
	textPtr, _ := syscall.UTF16PtrFromString(text)
	hwnd, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(textPtr)),
		uintptr(WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_AUTOCHECKBOX),
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		uintptr(parent), uintptr(id), 0, 0,
	)
	if font != 0 {
		procSendMessageW.Call(hwnd, WM_SETFONT, uintptr(font), 1)
	}
	if checked {
		procSendMessageW.Call(hwnd, BM_SETCHECK, uintptr(BST_CHECKED), 0)
	}
	return windows.Handle(hwnd)
}

func isChecked(hwnd windows.Handle) bool {
	res, _, _ := procSendMessageW.Call(uintptr(hwnd), BM_GETCHECK, 0, 0)
	return res == BST_CHECKED
}

func setWindowText(hwnd windows.Handle, text string) {
	textPtr, _ := syscall.UTF16PtrFromString(text)
	procSetWindowTextW.Call(uintptr(hwnd), uintptr(unsafe.Pointer(textPtr)))
}

func getWindowText(hwnd windows.Handle) string {
	length, _, _ := procGetWindowTextLengthW.Call(uintptr(hwnd))
	if length == 0 {
		return ""
	}
	buf := make([]uint16, length+1)
	procGetWindowTextW.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&buf[0])), uintptr(length+1))
	return syscall.UTF16ToString(buf)
}

func enableControl(hwnd windows.Handle, enable bool) {
	var val uintptr = 0
	if enable {
		val = 1
	}
	procEnableWindow.Call(uintptr(hwnd), val)
}

func createFont(fontName string, size int32, bold bool) windows.Handle {
	namePtr, _ := syscall.UTF16PtrFromString(fontName)
	var weight int32 = 400
	if bold {
		weight = 700
	}
	hFont, _, _ := procCreateFontW.Call(
		uintptr(size),
		0, 0, 0,
		uintptr(weight),
		0, 0, 0,
		1, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(namePtr)),
	)
	return windows.Handle(hFont)
}

func createSolidBrush(color uint32) windows.Handle {
	h, _, _ := procCreateSolidBrush.Call(uintptr(color))
	return windows.Handle(h)
}

func getSystemMetrics(index int32) int32 {
	proc := user32.NewProc("GetSystemMetrics")
	r, _, _ := proc.Call(uintptr(index))
	return int32(r)
}

func openFileDialog(parent windows.Handle) string {
	fileBuf := make([]uint16, 1024)
	filter, _ := syscall.UTF16PtrFromString("Keys & Config (*.txt;*.pem;*.pub;*.env)\x00*.txt;*.pem;*.pub;*.env\x00All Files (*.*)\x00*.*\x00\x00")
	title, _ := syscall.UTF16PtrFromString("Seleccionar Clave Pública")

	ofn := OPENFILENAMEW{
		StructSize:  uint32(unsafe.Sizeof(OPENFILENAMEW{})),
		HwndOwner:   parent,
		Filter:      filter,
		File:        &fileBuf[0],
		MaxFile:     uint32(len(fileBuf)),
		Title:       title,
		Flags:       OFN_FILEMUSTEXIST | OFN_PATHMUSTEXIST,
	}

	r, _, _ := procGetOpenFileNameW.Call(uintptr(unsafe.Pointer(&ofn)))
	if r != 0 {
		return syscall.UTF16ToString(fileBuf)
	}
	return ""
}

func getClipboardText() string {
	r, _, _ := procOpenClipboard.Call(0)
	if r == 0 {
		return ""
	}
	defer procCloseClipboard.Call()

	hData, _, _ := procGetClipboardData.Call(CF_UNICODETEXT)
	if hData == 0 {
		return ""
	}

	ptr, _, _ := procGlobalLock.Call(hData)
	if ptr == 0 {
		return ""
	}
	defer procGlobalUnlock.Call(hData)

	return windows.UTF16PtrToString((*uint16)(unsafe.Pointer(ptr)))
}

func extractPublicKeyFromText(text string) string {
	start := strings.Index(text, "-----BEGIN PUBLIC KEY-----")
	if start == -1 {
		start = strings.Index(text, "-----BEGIN RSA PUBLIC KEY-----")
	}
	if start == -1 {
		return ""
	}
	end := strings.Index(text, "-----END PUBLIC KEY-----")
	endLen := len("-----END PUBLIC KEY-----")
	if end == -1 {
		end = strings.Index(text, "-----END RSA PUBLIC KEY-----")
		endLen = len("-----END RSA PUBLIC KEY-----")
	}
	if end == -1 {
		return ""
	}
	return strings.TrimSpace(text[start : end+endLen])
}
