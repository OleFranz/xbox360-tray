package tray

import (
	"sync"
	"syscall"
	"unsafe"

	"xbox360tray/internal/icon"
)

const (
	className   = "Xbox360TrayBatteryWndClass"
	callbackMsg = 0x8001 // WM_APP + 1
	wmDestroy   = 0x0002
	wmCommand   = 0x0111
	wmClose     = 0x0010
	wmLButtonUp = 0x0202
	wmRButtonUp = 0x0205
	wmUser      = 0x0400

	nimAdd    = 0x00000000
	nimModify = 0x00000001
	nimDelete = 0x00000002

	nifMessage = 0x00000001
	nifIcon    = 0x00000002
	nifTip     = 0x00000004
	nifInfo    = 0x00000010

	niifWarning = 0x00000002

	tpmRightAlign  = 0x0008
	tpmBottomAlign = 0x0020
	tpmReturnCmd   = 0x0100

	mfString    = 0x00000000
	mfSeparator = 0x00000800
	mfChecked   = 0x00000008
	mfUnchecked = 0x00000000
	mfDisabled  = 0x00000002
	mfEnabled   = 0x00000000
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")

	procRegisterClassExW    = user32.NewProc("RegisterClassExW")
	procCreateWindowExW     = user32.NewProc("CreateWindowExW")
	procDefWindowProcW      = user32.NewProc("DefWindowProcW")
	procDestroyWindow       = user32.NewProc("DestroyWindow")
	procPostQuitMessage     = user32.NewProc("PostQuitMessage")
	procGetMessageW         = user32.NewProc("GetMessageW")
	procTranslateMessage    = user32.NewProc("TranslateMessage")
	procDispatchMessageW    = user32.NewProc("DispatchMessageW")
	procCreatePopupMenu     = user32.NewProc("CreatePopupMenu")
	procDestroyMenu         = user32.NewProc("DestroyMenu")
	procAppendMenuW         = user32.NewProc("AppendMenuW")
	procTrackPopupMenuEx    = user32.NewProc("TrackPopupMenuEx")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procGetCursorPos        = user32.NewProc("GetCursorPos")
	procPostMessageW        = user32.NewProc("PostMessageW")
	procDestroyIcon         = user32.NewProc("DestroyIcon")

	procGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")

	procShellNotifyIconW = shell32.NewProc("Shell_NotifyIconW")
)

type point struct{ X, Y int32 }

type wndClassExW struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     uintptr
	HIcon         uintptr
	HCursor       uintptr
	HbrBackground uintptr
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       uintptr
}

type notifyIconDataW struct {
	CbSize            uint32
	HWnd              uintptr
	UID               uint32
	Flags             uint32
	CallbackMessage   uint32
	HIcon             uintptr
	SzTip             [128]uint16
	DwState           uint32
	DwStateMask       uint32
	SzInfo            [256]uint16
	UVersionOrTimeout uint32
	SzInfoTitle       [64]uint16
	DwInfoFlags       uint32
	GuidItem          [16]byte
	HBalloonIcon      uintptr
}

// MenuItem describes one entry in the trays context menu
type MenuItem struct {
	ID        uint32
	Label     string
	Checked   bool
	Separator bool
}

var (
	mu          sync.Mutex
	hwnd        uintptr
	currentIcon uintptr // HICON of whatever is currently set
	tooltip     string
	visible     bool
	menuItems   []MenuItem
	onCommand   func(id uint32)
)

// Init creates the hidden message window backing the tray icon, must be
// called once, from the goroutine that will later call Run()
func Init(initialTooltip string, cmdHandler func(id uint32)) error {
	tooltip = initialTooltip
	onCommand = cmdHandler

	classNameW, _ := syscall.UTF16PtrFromString(className)
	hInstance, _, _ := procGetModuleHandleW.Call(0)

	wc := wndClassExW{
		LpfnWndProc:   syscall.NewCallback(wndProc),
		HInstance:     hInstance,
		LpszClassName: classNameW,
	}
	wc.CbSize = uint32(unsafe.Sizeof(wc))
	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	h, _, err := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(classNameW)),
		0,
		0, 0, 0, 0, 0,
		0, // no parent, a plain hidden top-level window is enough
		0,
		hInstance,
		0,
	)
	if h == 0 {
		return err
	}
	hwnd = h
	return nil
}

// SetMenu replaces the context menu shown on click
func SetMenu(items []MenuItem) {
	mu.Lock()
	defer mu.Unlock()
	menuItems = items
}

// SetIconPNG sets the tray icon from PNG bytes, if the icon is currently
// shown, it is updated live
func SetIconPNG(pngBytes []byte) error {
	h, err := icon.ToHICON(pngBytes)
	if err != nil {
		return err
	}
	mu.Lock()
	old := currentIcon
	currentIcon = uintptr(h)
	shouldModify := visible
	mu.Unlock()

	if old != 0 {
		procDestroyIcon.Call(old)
	}
	if shouldModify {
		notify(nimModify, nifIcon|nifTip)
	}
	return nil
}

// SetTooltip updates the tooltip text shown on hover
func SetTooltip(text string) {
	mu.Lock()
	tooltip = text
	shouldModify := visible
	mu.Unlock()
	if shouldModify {
		notify(nimModify, nifTip)
	}
}

// Show adds the icon to the notification area if it isnt already there
func Show() {
	mu.Lock()
	already := visible
	visible = true
	mu.Unlock()
	if !already {
		notify(nimAdd, nifMessage|nifIcon|nifTip)
	}
}

// Hide removes the icon from the notification area, Show() brings it back
func Hide() {
	mu.Lock()
	already := visible
	visible = false
	mu.Unlock()
	if already {
		notify(nimDelete, 0)
	}
}

// Notify shows a balloon notification
func Notify(title, message string) {
	mu.Lock()
	wasVisible := visible
	mu.Unlock()

	if !wasVisible {
		Show()
	}

	var data notifyIconDataW
	fillBase(&data)
	data.Flags = nifInfo
	data.DwInfoFlags = niifWarning
	copyString(data.SzInfoTitle[:], title)
	copyString(data.SzInfo[:], message)
	procShellNotifyIconW.Call(uintptr(nimModify), uintptr(unsafe.Pointer(&data)))

	if !wasVisible {
		Hide()
	}
}

func notify(action uintptr, flags uint32) {
	var data notifyIconDataW
	fillBase(&data)
	data.Flags = flags
	procShellNotifyIconW.Call(action, uintptr(unsafe.Pointer(&data)))
}

func fillBase(data *notifyIconDataW) {
	mu.Lock()
	defer mu.Unlock()
	data.CbSize = uint32(unsafe.Sizeof(*data))
	data.HWnd = hwnd
	data.UID = 1
	data.CallbackMessage = callbackMsg
	data.HIcon = currentIcon
	copyString(data.SzTip[:], tooltip)
}

// Run enters the Win32 message loop, blocks until Quit() is called
func Run() {
	var msg struct {
		Hwnd    uintptr
		Message uint32
		WParam  uintptr
		LParam  uintptr
		Time    uint32
		Pt      point
	}
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

// Quit tears down the icon and stops the message loop
func Quit() {
	Hide()
	procDestroyWindow.Call(hwnd)
}

func wndProc(hWnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case callbackMsg:
		switch uint32(lParam) {
		case wmLButtonUp, wmRButtonUp:
			showMenu(hWnd)
		}
		return 0
	case wmCommand:
		id := uint32(wParam & 0xFFFF)
		if onCommand != nil {
			onCommand(id)
		}
		return 0
	case wmClose, wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(hWnd, uintptr(msg), wParam, lParam)
	return r
}

func showMenu(hWnd uintptr) {
	mu.Lock()
	items := make([]MenuItem, len(menuItems))
	copy(items, menuItems)
	mu.Unlock()

	hMenu, _, _ := procCreatePopupMenu.Call()
	if hMenu == 0 {
		return
	}
	defer procDestroyMenu.Call(hMenu)

	for _, it := range items {
		if it.Separator {
			procAppendMenuW.Call(hMenu, uintptr(mfSeparator), 0, 0)
			continue
		}
		flags := uintptr(mfString | mfEnabled)
		if it.Checked {
			flags |= uintptr(mfChecked)
		} else {
			flags |= uintptr(mfUnchecked)
		}
		labelPtr, _ := syscall.UTF16PtrFromString(it.Label)
		procAppendMenuW.Call(hMenu, flags, uintptr(it.ID), uintptr(unsafe.Pointer(labelPtr)))
	}

	var pt point
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))

	// required so the menu closes properly when it loses focus
	procSetForegroundWindow.Call(hWnd)
	procTrackPopupMenuEx.Call(
		hMenu,
		uintptr(tpmRightAlign|tpmBottomAlign),
		uintptr(pt.X),
		uintptr(pt.Y),
		hWnd,
		0,
	)
	procPostMessageW.Call(hWnd, uintptr(wmUser+1), 0, 0)
}

func copyString(dst []uint16, s string) {
	u, err := syscall.UTF16FromString(s)
	if err != nil {
		return
	}
	n := len(u)
	if n > len(dst) {
		n = len(dst)
	}
	copy(dst, u[:n])
}
