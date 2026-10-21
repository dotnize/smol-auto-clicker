package main

import (
	"syscall"
	"unsafe"
)

// The small subset of Win32 used by the app. No cgo or third-party packages.
var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")

	getModuleHandle  = kernel32.NewProc("GetModuleHandleW")
	registerClass    = user32.NewProc("RegisterClassExW")
	createWindow     = user32.NewProc("CreateWindowExW")
	defWindowProc    = user32.NewProc("DefWindowProcW")
	destroyWindow    = user32.NewProc("DestroyWindow")
	showWindow       = user32.NewProc("ShowWindow")
	getMessage       = user32.NewProc("GetMessageW")
	translateMessage = user32.NewProc("TranslateMessage")
	dispatchMessage  = user32.NewProc("DispatchMessageW")
	isDialogMessage  = user32.NewProc("IsDialogMessageW")
	postQuitMessage  = user32.NewProc("PostQuitMessage")
	sendMessage      = user32.NewProc("SendMessageW")
	setWindowText    = user32.NewProc("SetWindowTextW")
	getWindowText    = user32.NewProc("GetWindowTextW")
	enableWindow     = user32.NewProc("EnableWindow")
	setFocus         = user32.NewProc("SetFocus")
	loadCursor       = user32.NewProc("LoadCursorW")
	loadIcon         = user32.NewProc("LoadIconW")
	messageBox       = user32.NewProc("MessageBoxW")
	registerHotKey   = user32.NewProc("RegisterHotKey")
	unregisterHotKey = user32.NewProc("UnregisterHotKey")
	setTimer         = user32.NewProc("SetTimer")
	killTimer        = user32.NewProc("KillTimer")
	sendInput        = user32.NewProc("SendInput")
	getStockObject   = gdi32.NewProc("GetStockObject")
)

const (
	wmDestroy = 0x0002
	wmClose   = 0x0010
	wmCommand = 0x0111
	wmTimer   = 0x0113
	wmHotKey  = 0x0312
	wmSetFont = 0x0030

	wsChild   = 0x40000000
	wsVisible = 0x10000000
	wsTabStop = 0x00010000
	wsBorder  = 0x00800000

	cbAddString = 0x0143
	cbGetCurSel = 0x0147
	cbSetCurSel = 0x014E
)

type windowClass struct {
	Size, Style          uint32
	WndProc              uintptr
	ClassExtra, WndExtra int32
	Instance, Icon       uintptr
	Cursor, Background   uintptr
	MenuName, ClassName  *uint16
	SmallIcon            uintptr
}

type message struct {
	Window         uintptr
	ID             uint32
	WParam, LParam uintptr
	Time           uint32
	X, Y           int32
	Private        uint32
}

type mouseInput struct {
	X, Y        int32
	Data, Flags uint32
	Time        uint32
	ExtraInfo   uintptr
}

// MOUSEINPUT is the largest member of INPUT's union. Go supplies the
// pointer-size alignment required between Type and Mouse on 64-bit Windows.
type input struct {
	Type  uint32
	Mouse mouseInput
}

func wide(s string) *uint16 {
	p, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		panic(err)
	}
	return p
}

func text(window uintptr, value string) {
	setWindowText.Call(window, uintptr(unsafe.Pointer(wide(value))))
}

func alert(window uintptr, value string) {
	messageBox.Call(window, uintptr(unsafe.Pointer(wide(value))),
		uintptr(unsafe.Pointer(wide(appName))), 0x10) // MB_ICONERROR
}

func inject(flags ...uint32) int {
	events := make([]input, len(flags))
	for i, flag := range flags {
		events[i].Mouse.Flags = flag
	}
	n, _, _ := sendInput.Call(uintptr(len(events)),
		uintptr(unsafe.Pointer(&events[0])), unsafe.Sizeof(events[0]))
	return int(n)
}
