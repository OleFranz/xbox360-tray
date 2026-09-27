package autostart

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

const (
	keyPath  = `Software\Microsoft\Windows\CurrentVersion\Run`
	valName  = "Xbox360TrayBattery"
	hkeyCU   = 0x80000001 // HKEY_CURRENT_USER
	regSzTyp = 1          // REG_SZ

	keySetValue   = 0x0002
	keyQueryValue = 0x0001

	errorSuccess   = 0
	errorFileNotFo = 2
)

var (
	advapi32             = syscall.NewLazyDLL("advapi32.dll")
	procRegCreateKeyExW  = advapi32.NewProc("RegCreateKeyExW")
	procRegOpenKeyExW    = advapi32.NewProc("RegOpenKeyExW")
	procRegSetValueExW   = advapi32.NewProc("RegSetValueExW")
	procRegDeleteValueW  = advapi32.NewProc("RegDeleteValueW")
	procRegQueryValueExW = advapi32.NewProc("RegQueryValueExW")
	procRegCloseKey      = advapi32.NewProc("RegCloseKey")
)

func Enable() error {
	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	quoted := `"` + exePath + `"`

	pathPtr, _ := syscall.UTF16PtrFromString(keyPath)
	var hKey syscall.Handle
	r, _, callErr := procRegCreateKeyExW.Call(
		uintptr(hkeyCU),
		uintptr(unsafe.Pointer(pathPtr)),
		0, 0, 0,
		uintptr(keySetValue),
		0,
		uintptr(unsafe.Pointer(&hKey)),
		0,
	)
	if r != errorSuccess {
		return fmt.Errorf("RegCreateKeyEx failed: %v", callErr)
	}
	defer procRegCloseKey.Call(uintptr(hKey))

	namePtr, _ := syscall.UTF16PtrFromString(valName)
	dataUTF16, _ := syscall.UTF16FromString(quoted)
	dataBytes := utf16ToBytes(dataUTF16)

	r, _, callErr = procRegSetValueExW.Call(
		uintptr(hKey),
		uintptr(unsafe.Pointer(namePtr)),
		0,
		uintptr(regSzTyp),
		uintptr(unsafe.Pointer(&dataBytes[0])),
		uintptr(len(dataBytes)),
	)
	if r != errorSuccess {
		return fmt.Errorf("RegSetValueEx failed: %v", callErr)
	}
	return nil
}

func Disable() error {
	pathPtr, _ := syscall.UTF16PtrFromString(keyPath)
	var hKey syscall.Handle
	r, _, _ := procRegOpenKeyExW.Call(
		uintptr(hkeyCU),
		uintptr(unsafe.Pointer(pathPtr)),
		0,
		uintptr(keySetValue),
		uintptr(unsafe.Pointer(&hKey)),
	)
	if r == errorFileNotFo {
		return nil // key doesnt exist -> nothing to disable
	}
	if r != errorSuccess {
		return fmt.Errorf("RegOpenKeyEx failed: code %d", r)
	}
	defer procRegCloseKey.Call(uintptr(hKey))

	namePtr, _ := syscall.UTF16PtrFromString(valName)
	r, _, _ = procRegDeleteValueW.Call(uintptr(hKey), uintptr(unsafe.Pointer(namePtr)))
	if r != errorSuccess && r != errorFileNotFo {
		return fmt.Errorf("RegDeleteValue failed: code %d", r)
	}
	return nil
}

func IsEnabled() bool {
	pathPtr, _ := syscall.UTF16PtrFromString(keyPath)
	var hKey syscall.Handle
	r, _, _ := procRegOpenKeyExW.Call(
		uintptr(hkeyCU),
		uintptr(unsafe.Pointer(pathPtr)),
		0,
		uintptr(keyQueryValue),
		uintptr(unsafe.Pointer(&hKey)),
	)
	if r != errorSuccess {
		return false
	}
	defer procRegCloseKey.Call(uintptr(hKey))

	namePtr, _ := syscall.UTF16PtrFromString(valName)
	var dataType uint32
	var dataSize uint32
	r, _, _ = procRegQueryValueExW.Call(
		uintptr(hKey),
		uintptr(unsafe.Pointer(namePtr)),
		0,
		uintptr(unsafe.Pointer(&dataType)),
		0,
		uintptr(unsafe.Pointer(&dataSize)),
	)
	return r == errorSuccess && dataSize > 0
}

func utf16ToBytes(u []uint16) []byte {
	b := make([]byte, len(u)*2)
	for i, v := range u {
		b[i*2] = byte(v)
		b[i*2+1] = byte(v >> 8)
	}
	return b
}
