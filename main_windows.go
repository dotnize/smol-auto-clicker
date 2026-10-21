package main

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

const (
	appName = "Smol Auto Clicker"

	idStart = 100
	idStop  = 101
	idMode  = 102

	hotkeyToggle = 1
	hotkeyStop   = 2
	timerAction  = 1
)

// All state and input live on the window's OS thread; no background workers.
var (
	window, buttonBox, modeBox, intervalEdit uintptr
	delayEdit                                uintptr
	startButton, stopButton, statusLabel     uintptr
	active, pending, holdMode                bool
	interval                                 uintptr
	downFlag, upFlag, heldUp                 uint32
)

func main() {
	runtime.LockOSThread()
	if err := run(); err != nil {
		alert(0, err.Error())
	}
}

func run() error {
	instance, _, _ := getModuleHandle.Call(0)
	cursor, _, _ := loadCursor.Call(0, 32512)  // IDC_ARROW
	icon, _, err := loadIcon.Call(instance, 1) // Embedded app icon.
	if icon == 0 {
		return fmt.Errorf("load app icon: %v", err)
	}
	className := wide("SmolAutoClicker")
	wc := windowClass{
		Size:    uint32(unsafe.Sizeof(windowClass{})),
		WndProc: syscall.NewCallback(windowProc), Instance: instance,
		Cursor: cursor, Icon: icon, Background: 16, // COLOR_BTNFACE + 1
		ClassName: className,
	}
	if ok, _, err := registerClass.Call(uintptr(unsafe.Pointer(&wc))); ok == 0 {
		return fmt.Errorf("register window: %v", err)
	}
	window, _, err = createWindow.Call(0, uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(wide(appName))),
		0x00CA0000, // caption, system menu, minimize button; fixed size
		0x80000000, 0x80000000, 440, 330, 0, 0, instance, 0)
	if window == 0 {
		return fmt.Errorf("create window: %v", err)
	}
	defer destroyWindow.Call(window)
	defer stop()
	if err := buildControls(instance); err != nil {
		return err
	}
	for _, key := range []struct{ id, vk uintptr }{
		{hotkeyToggle, 0x75}, // F6
		{hotkeyStop, 0x76},   // F7
	} {
		if ok, _, err := registerHotKey.Call(window, key.id, 0x4000, key.vk); ok == 0 {
			return fmt.Errorf("cannot register F%d: %v\nClose another autoclicker or app using this shortcut and try again", key.vk-0x6F, err)
		}
		defer unregisterHotKey.Call(window, key.id)
	}
	showWindow.Call(window, 5) // SW_SHOW
	setFocus.Call(buttonBox)
	var msg message
	for {
		result, _, err := getMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(result) == -1 {
			return fmt.Errorf("read window message: %v", err)
		}
		if result == 0 {
			return nil
		}
		if handled, _, _ := isDialogMessage.Call(window, uintptr(unsafe.Pointer(&msg))); handled == 0 {
			translateMessage.Call(uintptr(unsafe.Pointer(&msg)))
			dispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
		}
	}
}

func buildControls(instance uintptr) error {
	font, _, _ := getStockObject.Call(17) // DEFAULT_GUI_FONT
	for _, c := range []struct {
		class, label string
		style        uintptr
		x, y, w, h   uintptr
		id           uintptr
		handle       *uintptr
	}{
		{"STATIC", "Mouse button", 0, 20, 18, 180, 20, 0, nil},
		{"COMBOBOX", "", wsTabStop | 3 | 0x00200000, 20, 42, 180, 120, 0, &buttonBox},
		{"STATIC", "Action", 0, 224, 18, 180, 20, 0, nil},
		{"COMBOBOX", "", wsTabStop | 3, 224, 42, 180, 100, idMode, &modeBox},
		{"STATIC", "Click interval (milliseconds)", 0, 20, 84, 180, 20, 0, nil},
		{"EDIT", "100", wsTabStop | wsBorder | 0x2000 | 0x80, 20, 108, 180, 25, 0, &intervalEdit},
		{"STATIC", "Start delay (milliseconds)", 0, 224, 84, 180, 20, 0, nil},
		{"EDIT", "0", wsTabStop | wsBorder | 0x2000 | 0x80, 224, 108, 180, 25, 0, &delayEdit},
		{"STATIC", "Hold keeps the button down until stopped.\r\nActions follow your cursor. Use F6 to start in place.", 0, 20, 150, 384, 36, 0, nil},
		{"BUTTON", "Start (F6)", wsTabStop, 20, 202, 180, 32, idStart, &startButton},
		{"BUTTON", "Stop (F7)", wsTabStop, 224, 202, 180, 32, idStop, &stopButton},
		{"STATIC", "Stopped. F6 toggles; F7 always stops.", 0, 20, 250, 384, 32, 0, &statusLabel},
	} {
		handle, _, err := createWindow.Call(0, uintptr(unsafe.Pointer(wide(c.class))),
			uintptr(unsafe.Pointer(wide(c.label))), wsChild|wsVisible|c.style,
			c.x, c.y, c.w, c.h, window, c.id, instance, 0)
		if handle == 0 {
			return fmt.Errorf("create %s control: %v", c.class, err)
		}
		if c.handle != nil {
			*c.handle = handle
		}
		sendMessage.Call(handle, wmSetFont, font, 1)
	}
	for _, box := range []struct {
		handle uintptr
		items  []string
	}{
		{buttonBox, []string{"Left", "Right", "Middle"}},
		{modeBox, []string{"Click repeatedly", "Hold down"}},
	} {
		for _, item := range box.items {
			sendMessage.Call(box.handle, cbAddString, 0, uintptr(unsafe.Pointer(wide(item))))
		}
		sendMessage.Call(box.handle, cbSetCurSel, 0, 0)
	}
	sendMessage.Call(intervalEdit, 0x00C5, 8, 0) // EM_SETLIMITTEXT
	sendMessage.Call(delayEdit, 0x00C5, 8, 0)
	updateControls()
	return nil
}

func windowProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case wmCommand:
		switch wParam & 0xFFFF {
		case idStart:
			if wParam>>16 == 0 { // BN_CLICKED
				start()
			}
		case idStop:
			if wParam>>16 == 0 {
				stop()
			}
		case idMode:
			if wParam>>16 == 1 { // CBN_SELCHANGE
				updateControls()
			}
		}
		return 0
	case wmHotKey:
		if wParam == hotkeyStop || (wParam == hotkeyToggle && (active || heldUp != 0)) {
			stop()
		} else if wParam == hotkeyToggle {
			start()
		}
		return 0
	case wmTimer:
		if wParam == timerAction && active {
			tick()
		}
		return 0
	case wmClose:
		if stop() {
			destroyWindow.Call(hwnd)
		}
		return 0
	case wmDestroy:
		postQuitMessage.Call(0)
		return 0
	}
	result, _, _ := defWindowProc.Call(hwnd, uintptr(msg), wParam, lParam)
	return result
}

func start() {
	if active || heldUp != 0 {
		return
	}
	mode, _, _ := sendMessage.Call(modeBox, cbGetCurSel, 0, 0)
	holdMode = mode == 1
	if !holdMode {
		ms, err := milliseconds(intervalEdit)
		if err != nil || ms < 10 || ms > 86400000 {
			text(statusLabel, "Enter an interval from 10 to 86400000 ms.")
			setFocus.Call(intervalEdit)
			return
		}
		interval = uintptr(ms)
	}
	delay, err := milliseconds(delayEdit)
	if err != nil || delay > 86400000 {
		text(statusLabel, "Enter a start delay from 0 to 86400000 ms.")
		setFocus.Call(delayEdit)
		return
	}
	button, _, _ := sendMessage.Call(buttonBox, cbGetCurSel, 0, 0)
	downFlag = []uint32{0x0002, 0x0008, 0x0020}[button]
	upFlag = downFlag << 1
	active, pending = true, true
	if delay > 0 {
		if ok, _, _ := setTimer.Call(window, timerAction, uintptr(delay), 0); ok == 0 {
			active, pending = false, false
			text(statusLabel, "Could not start the timer.")
			return
		}
	}
	updateControls()
	setFocus.Call(stopButton)
	if delay == 0 {
		tick()
		return
	}
	text(statusLabel, fmt.Sprintf("Starting in %d ms. Move to your target.", delay))
}

func milliseconds(handle uintptr) (uint64, error) {
	var buf [32]uint16
	getWindowText.Call(handle, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return strconv.ParseUint(strings.TrimSpace(syscall.UTF16ToString(buf[:])), 10, 32)
}

func tick() {
	if pending {
		pending = false
		killTimer.Call(window, timerAction)
		if holdMode {
			heldUp = upFlag
			if inject(downFlag) != 1 {
				inputFailed()
				return
			}
			text(statusLabel, "Holding. F6 or F7 releases the button.")
			return
		}
		if ok, _, _ := setTimer.Call(window, timerAction, interval, 0); ok == 0 {
			stop()
			text(statusLabel, "Could not start the click timer.")
			return
		}
		text(statusLabel, "Clicking. F6 or F7 stops.")
	}
	if holdMode {
		return // Ignore any queued timer message after entering hold mode.
	}
	// A single SendInput call keeps down/up together in the input stream.
	if n := inject(downFlag, upFlag); n != 2 {
		if n == 1 {
			heldUp = upFlag // Down succeeded; release it during stop.
		}
		inputFailed()
	}
}

func inputFailed() {
	if stop() {
		text(statusLabel, "Input blocked. Check the target's permissions.")
	}
}

func stop() bool {
	killTimer.Call(window, timerAction)
	active, pending = false, false
	if heldUp != 0 {
		if inject(heldUp) != 1 {
			text(statusLabel, "Release blocked. Return to desktop; press F7 again.")
			updateControls()
			return false
		}
		heldUp = 0
	}
	updateControls()
	text(statusLabel, "Stopped. F6 toggles; F7 always stops.")
	return true
}

func updateControls() {
	busy := active || heldUp != 0
	for _, handle := range []uintptr{buttonBox, modeBox, delayEdit, startButton} {
		enableWindow.Call(handle, boolValue(!busy))
	}
	mode, _, _ := sendMessage.Call(modeBox, cbGetCurSel, 0, 0)
	enableWindow.Call(intervalEdit, boolValue(!busy && mode != 1))
	enableWindow.Call(stopButton, boolValue(busy))
}

func boolValue(b bool) uintptr {
	if b {
		return 1
	}
	return 0
}
