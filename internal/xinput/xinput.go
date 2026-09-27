package xinput

import (
	"fmt"
	"syscall"
	"unsafe"
)

type BatteryLevel int

const (
	BatteryEmpty BatteryLevel = iota
	BatteryLow
	BatteryMedium
	BatteryFull
	BatteryUnknown
)

type BatteryType int

const (
	BatteryTypeDisconnected BatteryType = iota
	BatteryTypeWired
	BatteryTypeAlkaline
	BatteryTypeNiMH
	BatteryTypeUnknown
)

const (
	batDevTypeGamepad     = 0x00
	batTypeDisconnectRaw  = 0x00
	batTypeWiredRaw       = 0x01
	xUserMaxCount         = 4
	powerOffControllerOrd = 103 // undocumented ordinal export
)

var (
	xinputDLL          *syscall.LazyDLL
	procGetBatteryInfo *syscall.LazyProc
	procGetState       *syscall.LazyProc
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	procGetProcAddress = kernel32.NewProc("GetProcAddress")
	powerOffProc       uintptr
)

func init() {
	// xinput9_1_0 is the fallback for pre win8 systems
	xinputDLL = syscall.NewLazyDLL("xinput1_4.dll")
	if err := xinputDLL.Load(); err != nil {
		xinputDLL = syscall.NewLazyDLL("xinput9_1_0.dll")
		xinputDLL.Load()
	}
	procGetBatteryInfo = xinputDLL.NewProc("XInputGetBatteryInformation")
	procGetState = xinputDLL.NewProc("XInputGetState")

	// ordinal only export, no name to resolve
	h := xinputDLL.Handle()
	addr, _, _ := procGetProcAddress.Call(h, uintptr(powerOffControllerOrd))
	powerOffProc = addr
}

type xinputBatteryInformation struct {
	BatteryType  byte
	BatteryLevel byte
}

type xinputState struct {
	PacketNumber uint32
	Gamepad      [12]byte // we only care whether the call succeeds
}

// DeviceStatus summarizes the state of one XInput slot
type DeviceStatus struct {
	UserIndex   uint32
	Connected   bool
	BatteryType BatteryType
	Level       BatteryLevel
}

// FindFirstConnected returns the status of the first connected controller,
// or Connected=false if none is connected
func FindFirstConnected() DeviceStatus {
	for i := uint32(0); i < xUserMaxCount; i++ {
		if _, ok := queryState(i); ok {
			bt, lvl := queryBattery(i)
			return DeviceStatus{UserIndex: i, Connected: true, BatteryType: bt, Level: lvl}
		}
	}
	return DeviceStatus{Connected: false}
}

func queryState(userIndex uint32) (xinputState, bool) {
	var state xinputState
	r, _, _ := procGetState.Call(uintptr(userIndex), uintptr(unsafe.Pointer(&state)))
	return state, r == 0
}

func queryBattery(userIndex uint32) (BatteryType, BatteryLevel) {
	var info xinputBatteryInformation
	r, _, _ := procGetBatteryInfo.Call(
		uintptr(userIndex),
		uintptr(batDevTypeGamepad),
		uintptr(unsafe.Pointer(&info)),
	)
	if r != 0 {
		return BatteryTypeUnknown, BatteryUnknown
	}
	var bt BatteryType
	switch info.BatteryType {
	case batTypeDisconnectRaw:
		bt = BatteryTypeDisconnected
	case batTypeWiredRaw:
		bt = BatteryTypeWired
	case 0x02:
		bt = BatteryTypeAlkaline
	case 0x03:
		bt = BatteryTypeNiMH
	default:
		bt = BatteryTypeUnknown
	}
	return bt, BatteryLevel(info.BatteryLevel)
}

// PowerOffController turns off the wireless controller at the given user index
func PowerOffController(userIndex uint32) error {
	if powerOffProc == 0 {
		return fmt.Errorf("XInputPowerOffController not found (ordinal %d)", powerOffControllerOrd)
	}
	r, _, callErr := syscall.SyscallN(powerOffProc, uintptr(userIndex))
	if r != 0 {
		return fmt.Errorf("XInputPowerOffController failed: %v (code %d)", callErr, r)
	}
	return nil
}
