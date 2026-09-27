package ipc

import (
	"syscall"
	"time"
	"unsafe"
)

const (
	mutexName = `Global\Xbox360TrayBattery_SingleInstance`
	pipeName  = `\\.\pipe\Xbox360TrayBattery_IPC`

	ShowCommand = "SHOW"

	errorAlreadyExists = 183
)

var (
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	procCreateMutexW = kernel32.NewProc("CreateMutexW")
)

// AcquireOrNotify claims the global mutex, returns true if this is the
// only running instance, otherwise sends ShowCommand to the running
// instances pipe and returns false
func AcquireOrNotify() bool {
	namePtr, err := syscall.UTF16PtrFromString(mutexName)
	if err != nil {
		return true // fail open rather than refusing to start
	}

	h, _, callErr := procCreateMutexW.Call(0, 0, uintptr(unsafe.Pointer(namePtr)))
	if h == 0 {
		return true // CreateMutex itself failed, dont block startup on this
	}

	if callErr != syscall.Errno(errorAlreadyExists) {
		return true // we are the first/only instance
	}

	// another instance is already running -> notify it via the pipe
	dialPipe()
	return false
}

func dialPipe() {
	pathPtr, err := syscall.UTF16PtrFromString(pipeName)
	if err != nil {
		return
	}
	for i := 0; i < 5; i++ {
		h, err := syscall.CreateFile(
			pathPtr,
			syscall.GENERIC_WRITE,
			0, nil,
			syscall.OPEN_EXISTING,
			0, 0,
		)
		if err == nil {
			msg := []byte(ShowCommand)
			var written uint32
			syscall.WriteFile(h, msg, &written, nil)
			syscall.CloseHandle(h)
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// ServePipe listens on the named pipe indefinitely and calls onShow()
// whenever a second instance sends a SHOW command
func ServePipe(onShow func()) {
	go func() {
		for {
			listenOnce(onShow)
		}
	}()
}

func listenOnce(onShow func()) {
	pathPtr, err := syscall.UTF16PtrFromString(pipeName)
	if err != nil {
		time.Sleep(time.Second)
		return
	}

	h, err := createNamedPipe(pathPtr)
	if err != nil {
		time.Sleep(time.Second)
		return
	}
	defer syscall.CloseHandle(h)

	connectNamedPipe(h) // blocks until a client connects

	buf := make([]byte, 32)
	var read uint32
	if err := syscall.ReadFile(h, buf, &read, nil); err == nil && read > 0 {
		if string(buf[:read]) == ShowCommand {
			onShow()
		}
	}
}

var (
	procCreateNamedPipeW = kernel32.NewProc("CreateNamedPipeW")
	procConnectNamedPipe = kernel32.NewProc("ConnectNamedPipe")
)

const (
	pipeAccessDuplex   = 0x00000003
	pipeTypeMessage    = 0x00000004
	pipeReadModeByte   = 0x00000000
	pipeWait           = 0x00000000
	pipeUnlimitedInsts = 255
)

func createNamedPipe(name *uint16) (syscall.Handle, error) {
	h, _, err := procCreateNamedPipeW.Call(
		uintptr(unsafe.Pointer(name)),
		uintptr(pipeAccessDuplex),
		uintptr(pipeTypeMessage|pipeReadModeByte|pipeWait),
		uintptr(pipeUnlimitedInsts),
		512, 512, 0, 0,
	)
	if h == 0 || h == ^uintptr(0) {
		return 0, err
	}
	return syscall.Handle(h), nil
}

func connectNamedPipe(h syscall.Handle) {
	procConnectNamedPipe.Call(uintptr(h), 0)
}
