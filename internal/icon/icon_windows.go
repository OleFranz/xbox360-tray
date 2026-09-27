package icon

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	user32                       = syscall.NewLazyDLL("user32.dll")
	procCreateIconFromResourceEx = user32.NewProc("CreateIconFromResourceEx")
)

const (
	imageIcon      = 1
	lrDefaultColor = 0x0000
	iconVersion3   = 0x00030000
)

// ToHICON converts a PNG buffer directly into a Windows HICON handle
func ToHICON(pngBytes []byte) (syscall.Handle, error) {
	r, _, err := procCreateIconFromResourceEx.Call(
		uintptr(unsafe.Pointer(&pngBytes[0])),
		uintptr(len(pngBytes)),
		1, // fIcon = TRUE
		uintptr(iconVersion3),
		uintptr(Size),
		uintptr(Size),
		uintptr(lrDefaultColor),
	)
	if r == 0 {
		return 0, fmt.Errorf("CreateIconFromResourceEx failed: %v", err)
	}
	return syscall.Handle(r), nil
}
