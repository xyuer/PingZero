# Testing WinDivert64.sys Locally

Windows 10/11 x64 cannot normally load a completely unsigned kernel driver. For local development, use a test-signed driver and enable Windows Test Mode.

References:

- https://learn.microsoft.com/en-us/windows-hardware/drivers/install/the-testsigning-boot-configuration-option
- https://learn.microsoft.com/en-us/windows-hardware/drivers/install/test-signing

## Enable Test Mode

Run PowerShell or Command Prompt as Administrator:

```powershell
bcdedit /set testsigning on
```

Restart Windows. After reboot, the desktop should show a `Test Mode` watermark.

If Windows reports that the value is protected by Secure Boot policy, disable Secure Boot in BIOS/UEFI first. If BitLocker is enabled, suspend or prepare your recovery key before changing boot settings.

## Create a Test Certificate

Run PowerShell as Administrator:

```powershell
$cert = New-SelfSignedCertificate `
  -Type CodeSigningCert `
  -Subject "CN=PingZero Test Driver" `
  -CertStoreLocation "Cert:\LocalMachine\My"

$cer = "E:\workspace\PingZero\third_party\windivert\PingZeroTestDriver.cer"
Export-Certificate -Cert $cert -FilePath $cer

Import-Certificate -FilePath $cer -CertStoreLocation "Cert:\LocalMachine\Root"
Import-Certificate -FilePath $cer -CertStoreLocation "Cert:\LocalMachine\TrustedPublisher"
```

## Sign the Driver

Find `signtool.exe`:

```powershell
Get-ChildItem "C:\Program Files (x86)\Windows Kits\10\bin" -Recurse -Filter signtool.exe
```

Then sign `WinDivert64.sys`:

```powershell
& "C:\Program Files (x86)\Windows Kits\10\bin\10.0.26100.0\x64\signtool.exe" sign `
  /v /fd SHA256 /s My /n "PingZero Test Driver" `
  "E:\workspace\PingZero\third_party\windivert\bin\amd64\WinDivert64.sys"
```

Verify the signature:

```powershell
& "C:\Program Files (x86)\Windows Kits\10\bin\10.0.26100.0\x64\signtool.exe" verify `
  /v /kp `
  "E:\workspace\PingZero\third_party\windivert\bin\amd64\WinDivert64.sys"
```

## Runtime Test

Copy both files next to the PingZero client executable:

```powershell
Copy-Item third_party\windivert\bin\amd64\WinDivert.dll  .\build\bin\
Copy-Item third_party\windivert\bin\amd64\WinDivert64.sys .\build\bin\
```

Run the PingZero client as Administrator. The first call to `WinDivertOpen` should load the driver.

If loading fails, check:

- `WinDivert.dll` and `WinDivert64.sys` are in the same directory as the client `.exe`.
- The client is running as Administrator.
- Test Mode is enabled and Windows was restarted after enabling it.
- `WinDivert64.sys` has a test signature.
- Secure Boot or Memory Integrity/HVCI is not blocking local test drivers.
- Windows Event Viewer has no driver load errors.

## Disable Test Mode

After testing, run as Administrator:

```powershell
bcdedit /set testsigning off
```

Restart Windows.

## Notes

This workflow is for development machines only. Production releases need proper driver signing.
