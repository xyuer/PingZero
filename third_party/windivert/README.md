# WinDivert Dependency

PingZero uses the open-source WinDivert project for Windows packet capture and reinjection.

WinDivert is not a Go module dependency. The Go code in `internal/windivert` dynamically loads the native `WinDivert.dll` at runtime and calls the exported C API directly.

Expected runtime files:

- `WinDivert.dll`
- `WinDivert64.sys`

The x64 runtime files are vendored at:

- `third_party/windivert/bin/amd64/WinDivert.dll`
- `third_party/windivert/bin/amd64/WinDivert64.sys`

For runtime use, copy both files next to the built PingZero client executable, or otherwise make `WinDivert.dll` available on the DLL search path. The client must run as administrator so WinDivert can load its driver.

Source project:

- https://github.com/basil00/WinDivert

The vendored files in `bin/amd64` were built locally from the WinDivert source tree.

## Build

Use the helper script from the repository root:

```powershell
third_party\windivert\build-windivert.cmd
```

By default, the script expects the WinDivert source tree at `..\WinDivert` beside this repository. Override it when needed:

```powershell
third_party\windivert\build-windivert.cmd -SourceDir E:\workspace\WinDivert
```

The script builds x64 `WinDivert.dll` and `WinDivert64.sys`, then copies them into `third_party\windivert\bin\amd64`.

Current defaults:

- Visual Studio: `C:\Program Files\Microsoft Visual Studio\18\Community`
- MSVC: `14.51.36231`
- Windows Kit: `10.0.26100.0`
- KMDF: `1.35`
- Driver signing: off

For local test-signing and Windows Test Mode instructions, see `TEST_SIGNING.md`.
