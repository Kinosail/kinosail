//go:build windows

package main

import (
	"errors"
	"path/filepath"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	wmDestroy     = 0x0002
	wmClose       = 0x0010
	wmLButtonUp   = 0x0202
	wmRButtonUp   = 0x0205
	wmTray        = 0x8001
	nimAdd        = 0
	nimModify     = 1
	nimDelete     = 2
	nifMessage    = 0x1
	nifIcon       = 0x2
	nifTip        = 0x4
	mfString      = 0x0
	mfGrayed      = 0x1
	mfChecked     = 0x8
	mfSeparator   = 0x800
	tpmReturnCmd  = 0x100
	tpmRightAlign = 0x8
	imageIcon     = 1
	lrLoadFile    = 0x10
	lrDefaultSize = 0x40
	menuOpen      = 1
	menuRestart   = 2
	menuLogin     = 3
	menuQuit      = 4
)

var (
	user32           = windows.NewLazySystemDLL("user32.dll")
	shell32          = windows.NewLazySystemDLL("shell32.dll")
	registerClass    = user32.NewProc("RegisterClassExW")
	createWindow     = user32.NewProc("CreateWindowExW")
	defaultProcedure = user32.NewProc("DefWindowProcW")
	getMessage       = user32.NewProc("GetMessageW")
	translateMessage = user32.NewProc("TranslateMessage")
	dispatchMessage  = user32.NewProc("DispatchMessageW")
	postQuit         = user32.NewProc("PostQuitMessage")
	destroyWindow    = user32.NewProc("DestroyWindow")
	createMenu       = user32.NewProc("CreatePopupMenu")
	appendMenu       = user32.NewProc("AppendMenuW")
	trackMenu        = user32.NewProc("TrackPopupMenu")
	destroyMenu      = user32.NewProc("DestroyMenu")
	foreground       = user32.NewProc("SetForegroundWindow")
	cursorPosition   = user32.NewProc("GetCursorPos")
	loadImage        = user32.NewProc("LoadImageW")
	registerMessage  = user32.NewProc("RegisterWindowMessageW")
	notifyIcon       = shell32.NewProc("Shell_NotifyIconW")
)

type point struct{ X, Y int32 }

type message struct {
	Window  uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Point   point
	Private uint32
}

type windowClass struct {
	Size       uint32
	Style      uint32
	Procedure  uintptr
	ClassExtra int32
	WindowData int32
	Instance   windows.Handle
	Icon       windows.Handle
	Cursor     windows.Handle
	Background windows.Handle
	MenuName   *uint16
	ClassName  *uint16
	SmallIcon  windows.Handle
}

type notifyIconData struct {
	Size            uint32
	Window          uintptr
	ID              uint32
	Flags           uint32
	CallbackMessage uint32
	Icon            windows.Handle
	Tip             [128]uint16
	State           uint32
	StateMask       uint32
	Info            [256]uint16
	Version         uint32
	InfoTitle       [64]uint16
	InfoFlags       uint32
	GUID            windows.GUID
	BalloonIcon     windows.Handle
}

type trayIcon struct {
	server     *supervisor
	window     uintptr
	icon       windows.Handle
	recreate   uintptr
	mu         sync.Mutex
	statusText string
}

func (tray *trayIcon) create(install string) error {
	var instance windows.Handle
	if err := windows.GetModuleHandleEx(0, nil, &instance); err != nil {
		return err
	}
	className, _ := windows.UTF16PtrFromString("Kinosail.Launcher")
	class := windowClass{Procedure: windows.NewCallback(tray.procedure), Instance: instance, ClassName: className}
	class.Size = uint32(unsafe.Sizeof(class))
	if atom, _, _ := registerClass.Call(uintptr(unsafe.Pointer(&class))); atom == 0 {
		return errors.New("tray window class was not registered")
	}
	title, _ := windows.UTF16PtrFromString("Kinosail")
	tray.window, _, _ = createWindow.Call(0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(title)), 0, 0, 0, 0, 0, 0, 0, uintptr(instance), 0)
	if tray.window == 0 {
		return errors.New("tray window was not created")
	}
	iconPath, _ := windows.UTF16PtrFromString(filepath.Join(install, "Kinosail.ico"))
	icon, _, _ := loadImage.Call(0, uintptr(unsafe.Pointer(iconPath)), imageIcon, 0, 0, lrLoadFile|lrDefaultSize)
	tray.icon = windows.Handle(icon)
	created, _ := windows.UTF16PtrFromString("TaskbarCreated")
	tray.recreate, _, _ = registerMessage.Call(uintptr(unsafe.Pointer(created)))
	tray.notify(nimAdd)
	return nil
}

func (tray *trayIcon) notify(action uintptr) {
	data := notifyIconData{Window: tray.window, ID: 1, Flags: nifMessage | nifIcon | nifTip, CallbackMessage: wmTray, Icon: tray.icon}
	data.Size = uint32(unsafe.Sizeof(data))
	tray.mu.Lock()
	tip := "Kinosail"
	if tray.statusText != "" {
		tip = "Kinosail: " + tray.statusText
	}
	tray.mu.Unlock()
	text, _ := windows.UTF16FromString(tip)
	copy(data.Tip[:len(data.Tip)-1], text)
	_, _, _ = notifyIcon.Call(action, uintptr(unsafe.Pointer(&data)))
}

func (tray *trayIcon) setStatus(text string) {
	tray.mu.Lock()
	tray.statusText = text
	tray.mu.Unlock()
	if tray.window != 0 {
		tray.notify(nimModify)
	}
}

func (tray *trayIcon) loop() {
	var current message
	for {
		result, _, _ := getMessage.Call(uintptr(unsafe.Pointer(&current)), 0, 0, 0)
		if result == 0 || result == ^uintptr(0) {
			return
		}
		_, _, _ = translateMessage.Call(uintptr(unsafe.Pointer(&current)))
		_, _, _ = dispatchMessage.Call(uintptr(unsafe.Pointer(&current)))
	}
}

func (tray *trayIcon) procedure(window, kind, wParam, lParam uintptr) uintptr {
	switch {
	case kind == wmTray && lParam == wmLButtonUp:
		openKinosail()
	case kind == wmTray && lParam == wmRButtonUp:
		tray.showMenu()
	case kind == wmClose:
		tray.quit()
	case kind == wmDestroy:
		_, _, _ = postQuit.Call(0)
	case tray.recreate != 0 && kind == tray.recreate:
		tray.notify(nimAdd)
	default:
		result, _, _ := defaultProcedure.Call(window, kind, wParam, lParam)
		return result
	}
	return 0
}

func (tray *trayIcon) showMenu() {
	menu, _, _ := createMenu.Call()
	if menu == 0 {
		return
	}
	defer func() { _, _, _ = destroyMenu.Call(menu) }()
	tray.mu.Lock()
	status := tray.statusText
	tray.mu.Unlock()
	login := uintptr(mfString)
	if startsAtLogin() {
		login |= mfChecked
	}
	for _, entry := range []struct {
		flags uintptr
		id    uintptr
		label string
	}{
		{mfString | mfGrayed, 0, status},
		{mfSeparator, 0, ""},
		{mfString, menuOpen, "Open Kinosail"},
		{mfString, menuRestart, "Restart Kinosail"},
		{login, menuLogin, "Start Kinosail when I sign in"},
		{mfSeparator, 0, ""},
		{mfString, menuQuit, "Quit Kinosail"},
	} {
		if entry.flags == mfString|mfGrayed && entry.label == "" {
			continue
		}
		label, _ := windows.UTF16PtrFromString(entry.label)
		_, _, _ = appendMenu.Call(menu, entry.flags, entry.id, uintptr(unsafe.Pointer(label)))
	}
	var cursor point
	_, _, _ = cursorPosition.Call(uintptr(unsafe.Pointer(&cursor)))
	_, _, _ = foreground.Call(tray.window)
	choice, _, _ := trackMenu.Call(menu, tpmReturnCmd|tpmRightAlign, uintptr(cursor.X), uintptr(cursor.Y), 0, tray.window, 0) //nolint:gosec // Win32 reads the signed screen coordinates from the low 32 bits.
	switch choice {
	case menuOpen:
		openKinosail()
	case menuRestart:
		go tray.server.restart()
	case menuLogin:
		setStartsAtLogin(!startsAtLogin())
	case menuQuit:
		tray.quit()
	}
}

func (tray *trayIcon) quit() {
	tray.setStatus("Stopping Kinosail…")
	tray.server.stop()
	tray.notify(nimDelete)
	_, _, _ = destroyWindow.Call(tray.window)
}
