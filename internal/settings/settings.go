package settings

import (
	"syscall"
	"unsafe"
)

const (
	keyPath = `Software\Xbox360TrayBattery`
	hkeyCU  = 0x80000001 // HKEY_CURRENT_USER

	regDword = 4

	keySetValue   = 0x0002
	keyQueryValue = 0x0001
	keyWow64_64   = 0x0100

	errorSuccess = 0
)

var (
	advapi32             = syscall.NewLazyDLL("advapi32.dll")
	procRegCreateKeyExW  = advapi32.NewProc("RegCreateKeyExW")
	procRegOpenKeyExW    = advapi32.NewProc("RegOpenKeyExW")
	procRegSetValueExW   = advapi32.NewProc("RegSetValueExW")
	procRegQueryValueExW = advapi32.NewProc("RegQueryValueExW")
	procRegCloseKey      = advapi32.NewProc("RegCloseKey")
)

// LoadBool reads a boolean setting, returning def if the key/value doesnt exist yet
func LoadBool(name string, def bool) bool {
	pathPtr, _ := syscall.UTF16PtrFromString(keyPath)
	var hKey syscall.Handle
	r, _, _ := procRegOpenKeyExW.Call(
		uintptr(hkeyCU),
		uintptr(unsafe.Pointer(pathPtr)),
		0,
		uintptr(keyQueryValue|keyWow64_64),
		uintptr(unsafe.Pointer(&hKey)),
	)
	if r != errorSuccess {
		return def
	}
	defer procRegCloseKey.Call(uintptr(hKey))

	namePtr, _ := syscall.UTF16PtrFromString(name)
	var dataType uint32
	var data uint32
	dataSize := uint32(unsafe.Sizeof(data))
	r, _, _ = procRegQueryValueExW.Call(
		uintptr(hKey),
		uintptr(unsafe.Pointer(namePtr)),
		0,
		uintptr(unsafe.Pointer(&dataType)),
		uintptr(unsafe.Pointer(&data)),
		uintptr(unsafe.Pointer(&dataSize)),
	)
	if r != errorSuccess || dataType != regDword {
		return def
	}
	return data != 0
}

// SaveBool writes a boolean setting, creating the key if necessary
func SaveBool(name string, value bool) error {
	pathPtr, _ := syscall.UTF16PtrFromString(keyPath)
	var hKey syscall.Handle
	r, _, callErr := procRegCreateKeyExW.Call(
		uintptr(hkeyCU),
		uintptr(unsafe.Pointer(pathPtr)),
		0, 0, 0,
		uintptr(keySetValue|keyWow64_64),
		0,
		uintptr(unsafe.Pointer(&hKey)),
		0,
	)
	if r != errorSuccess {
		return callErr
	}
	defer procRegCloseKey.Call(uintptr(hKey))

	namePtr, _ := syscall.UTF16PtrFromString(name)
	var data uint32
	if value {
		data = 1
	}
	r, _, callErr = procRegSetValueExW.Call(
		uintptr(hKey),
		uintptr(unsafe.Pointer(namePtr)),
		0,
		uintptr(regDword),
		uintptr(unsafe.Pointer(&data)),
		uintptr(unsafe.Sizeof(data)),
	)
	if r != errorSuccess {
		return callErr
	}
	return nil
}
