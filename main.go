package main

import (
	"time"

	"xbox360tray/internal/autostart"
	"xbox360tray/internal/icon"
	"xbox360tray/internal/ipc"
	"xbox360tray/internal/settings"
	"xbox360tray/internal/tray"
	"xbox360tray/internal/xinput"
)

const pollInterval = 3 * time.Second

const (
	menuAutostart = iota + 1
	menuNotifyLowBattery
	menuHideOnDisconnect
	menuPowerOff
	menuExit
)

const (
	settingNotifyLowBattery = "NotifyOnLowBattery"
	settingHideOnDisconnect = "HideOnDisconnect"
)

type appState struct {
	autostartEnabled bool
	notifyOnLow      bool
	hideOnDisconnect bool

	// forceShow overrides hideOnDisconnect until the next real connect,
	// used when a second launch asks this instance to become visible
	forceShow bool

	lastWasLow   bool
	lastUserIdx  uint32
	wasConnected bool
}

var state = &appState{}

func main() {
	// single instance: if the app is already running, notify it and exit
	if !ipc.AcquireOrNotify() {
		return
	}

	state.autostartEnabled = autostart.IsEnabled()
	state.notifyOnLow = settings.LoadBool(settingNotifyLowBattery, true)
	state.hideOnDisconnect = settings.LoadBool(settingHideOnDisconnect, false)

	if err := tray.Init("Xbox 360 Controller: no device connected", onMenuCommand); err != nil {
		return
	}
	setIcon(icon.StateUnknown)
	rebuildMenu()
	tray.Show()

	// second launch: make the icon visible
	ipc.ServePipe(func() {
		state.forceShow = true
		tray.Show()
	})

	go pollLoop()

	tray.Run()
}

func setIcon(s icon.State) {
	img := icon.Render(s)
	tray.SetIconPNG(icon.EncodePNG(img))
}

func rebuildMenu() {
	tray.SetMenu([]tray.MenuItem{
		{ID: menuAutostart, Label: "Start with Windows", Checked: state.autostartEnabled},
		{ID: menuNotifyLowBattery, Label: "Notify on low battery", Checked: state.notifyOnLow},
		{ID: menuHideOnDisconnect, Label: "Hide on disconnect", Checked: state.hideOnDisconnect},
		{Separator: true},
		{ID: menuPowerOff, Label: "Turn off controller"},
		{Separator: true},
		{ID: menuExit, Label: "Exit"},
	})
}

func onMenuCommand(id uint32) {
	switch id {
	case menuAutostart:
		if state.autostartEnabled {
			if autostart.Disable() == nil {
				state.autostartEnabled = false
			}
		} else {
			if autostart.Enable() == nil {
				state.autostartEnabled = true
			}
		}
		rebuildMenu()

	case menuNotifyLowBattery:
		state.notifyOnLow = !state.notifyOnLow
		settings.SaveBool(settingNotifyLowBattery, state.notifyOnLow)
		rebuildMenu()

	case menuHideOnDisconnect:
		state.hideOnDisconnect = !state.hideOnDisconnect
		settings.SaveBool(settingHideOnDisconnect, state.hideOnDisconnect)
		rebuildMenu()
		// reevaluate immediately so toggling the option takes effect
		// without waiting for the next poll tick
		applyVisibility(state.wasConnected)

	case menuPowerOff:
		if state.wasConnected {
			xinput.PowerOffController(state.lastUserIdx)
		}

	case menuExit:
		tray.Quit()
	}
}

func pollLoop() {
	for {
		status := xinput.FindFirstConnected()
		updateFromStatus(status)
		time.Sleep(pollInterval)
	}
}

func updateFromStatus(status xinput.DeviceStatus) {
	if !status.Connected {
		if state.wasConnected {
			setIcon(icon.StateUnknown)
			tray.SetTooltip("Xbox 360 Controller: no device connected")
		}
		state.wasConnected = false
		state.lastWasLow = false
		applyVisibility(false)
		return
	}

	state.wasConnected = true
	state.lastUserIdx = status.UserIndex
	state.forceShow = false // a real connect/disconnect cycle takes over from here
	applyVisibility(true)

	var st icon.State
	var label string
	switch status.Level {
	case xinput.BatteryFull:
		st, label = icon.StateFull, "full"
	case xinput.BatteryMedium:
		st, label = icon.StateMedium, "medium"
	case xinput.BatteryLow, xinput.BatteryEmpty:
		st, label = icon.StateLow, "low"
	default:
		st, label = icon.StateMedium, "unknown"
	}

	setIcon(st)
	tray.SetTooltip("Xbox 360 Controller: battery " + label)

	isLowNow := status.Level == xinput.BatteryLow || status.Level == xinput.BatteryEmpty
	if isLowNow && !state.lastWasLow && state.notifyOnLow {
		tray.Notify("Controller battery low", "Your Xbox 360 controller's battery is almost empty.")
	}
	state.lastWasLow = isLowNow
}

// applyVisibility shows or hides the icon based on hideOnDisconnect,
// connection state, and forceShow
func applyVisibility(connected bool) {
	if !state.hideOnDisconnect || connected || state.forceShow {
		tray.Show()
		return
	}
	tray.Hide()
}
