# xbox360-tray

Minimalist Windows tray app showing the battery status of an Xbox 360 controller.

## Build

Requirement: Go 1.27+

The project has zero third-party dependencies, everything is raw
Windows API calls via `syscall`.

Build the .exe with:

```
go build -ldflags="-H=windowsgui -s -w" -o xbox360-tray.exe .
```

- `-H=windowsgui` suppresses the console window.
- `-s -w` strips debug info, shrinking the binary.

## Features

- Tray icon shows the battery status (green/yellow/red) of the first
  XInput-detected controller
- Context menu: autostart, low-battery notification toggle, hide-on-disconnect
  toggle, turn off controller, exit
- Single instance via named mutex + named pipe: launching the .exe a
  second time makes the already-running instance show its icon instead of
  starting a second one
- Settings (notify on low battery, hide on disconnect) persist across
  restarts under `HKCU\Software\Xbox360TrayBattery`
- Autostart via `HKCU\...\Run`
- Balloon notification on low battery via `Shell_NotifyIcon`/`NIF_INFO`
- Custom tray icon implementation (`internal/tray`) built directly on
  `Shell_NotifyIcon`, supporting `NIM_ADD`/`NIM_DELETE` so "hide on
  disconnect" actually removes and re-adds the icon