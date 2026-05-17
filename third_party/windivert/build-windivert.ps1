param(
    [string]$SourceDir = "",
    [string]$VsInstallPath = "C:\Program Files\Microsoft Visual Studio\18\Community",
    [string]$MsvcVersion = "14.51.36231",
    [string]$KitVersion = "10.0.26100.0",
    [string]$KmdfVersion = "1.35",
    [string]$VisualStudioVersion = "17.0",
    [switch]$SkipDriver,
    [switch]$SkipDll
)

$ErrorActionPreference = "Stop"

function Resolve-ExistingPath {
    param(
        [Parameter(Mandatory = $true)][string]$Path,
        [Parameter(Mandatory = $true)][string]$Description
    )
    if (-not (Test-Path -LiteralPath $Path)) {
        throw "$Description not found: $Path"
    }
    return (Resolve-Path -LiteralPath $Path).Path
}

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$ProjectRoot = Resolve-Path -LiteralPath (Join-Path $ScriptDir "..\..")

if ([string]::IsNullOrWhiteSpace($SourceDir)) {
    $SourceDir = Join-Path $ProjectRoot "..\WinDivert"
}

$SourceDir = Resolve-ExistingPath $SourceDir "WinDivert source directory"
$msbuild = Resolve-ExistingPath (Join-Path $VsInstallPath "MSBuild\Current\Bin\MSBuild.exe") "MSBuild"
$clDir = Resolve-ExistingPath (Join-Path $VsInstallPath "VC\Tools\MSVC\$MsvcVersion\bin\Hostx64\x64") "MSVC x64 compiler directory"
$kitRoot = Resolve-ExistingPath "C:\Program Files (x86)\Windows Kits\10" "Windows Kits root"
$kitBin = Resolve-ExistingPath (Join-Path $kitRoot "bin\$KitVersion\x64") "Windows Kit x64 tools"
$kitInclude = Resolve-ExistingPath (Join-Path $kitRoot "Include\$KitVersion") "Windows Kit include directory"
$kmdfInclude = Resolve-ExistingPath (Join-Path $kitRoot "Include\wdf\kmdf\$KmdfVersion") "KMDF include directory"

$requiredFiles = @(
    (Join-Path $kitInclude "um\winsock2.h"),
    (Join-Path $kitInclude "shared\ndis\types.h"),
    (Join-Path $kitInclude "km\ntifs.h"),
    (Join-Path $kmdfInclude "wdf.h"),
    (Join-Path $kitBin "mc.exe")
)
foreach ($file in $requiredFiles) {
    Resolve-ExistingPath $file "Required WinDivert build dependency" | Out-Null
}

$oldPath = $env:PATH
$env:PATH = "$clDir;$msbuild;$kitBin;$oldPath"

$outDir = Join-Path $SourceDir "install\MSVC\amd64"
$vendoredDir = Join-Path $ScriptDir "bin\amd64"
New-Item -ItemType Directory -Force -Path $outDir, $vendoredDir | Out-Null

try {
    if (-not $SkipDll) {
        Write-Host "Building WinDivert.dll..."
        & $msbuild (Join-Path $SourceDir "dll\windivert.vcxproj") `
            /p:Configuration=Release `
            /p:Platform=x64 `
            /p:PlatformToolset=v145 `
            /p:VCToolsVersion=$MsvcVersion `
            /p:WindowsTargetPlatformVersion=$KitVersion `
            /p:OutDir="..\install\MSVC\amd64\"
        if ($LASTEXITCODE -ne 0) {
            throw "WinDivert.dll build failed with exit code $LASTEXITCODE"
        }
    }

    if (-not $SkipDriver) {
        Write-Host "Generating WinDivert message resources..."
        Push-Location (Join-Path $SourceDir "sys")
        try {
            & (Join-Path $kitBin "mc.exe") -h . -r . windivert_log.mc
            if ($LASTEXITCODE -ne 0) {
                throw "mc.exe failed with exit code $LASTEXITCODE"
            }
        }
        finally {
            Pop-Location
        }

        Write-Host "Building WinDivert64.sys..."
        & $msbuild (Join-Path $SourceDir "sys\windivert.vcxproj") `
            /p:VisualStudioVersion=$VisualStudioVersion `
            /p:Configuration=Release `
            /p:Platform=x64 `
            /p:PlatformToolset=WindowsKernelModeDriver10.0 `
            /p:VCToolsVersion=$MsvcVersion `
            /p:WindowsTargetPlatformVersion=$KitVersion `
            /p:TargetVersion=Windows10 `
            /p:KmdfVersion=$KmdfVersion `
            /p:SpectreMitigation=false `
            /p:BasicRuntimeChecks=Default `
            /p:UseDebugLibraries=false `
            /p:TreatWarningAsError=false `
            /p:SignMode=Off `
            /p:OutDir="..\install\MSVC\amd64\" `
            /p:AssemblyName=WinDivert64
        if ($LASTEXITCODE -ne 0) {
            throw "WinDivert64.sys build failed with exit code $LASTEXITCODE"
        }
    }

    $dll = Resolve-ExistingPath (Join-Path $outDir "WinDivert.dll") "Built WinDivert.dll"
    $sys = Resolve-ExistingPath (Join-Path $outDir "WinDivert64.sys") "Built WinDivert64.sys"

    Copy-Item -LiteralPath $dll -Destination (Join-Path $vendoredDir "WinDivert.dll") -Force
    Copy-Item -LiteralPath $sys -Destination (Join-Path $vendoredDir "WinDivert64.sys") -Force

    Write-Host "Copied runtime files to $vendoredDir"
    Get-ChildItem -LiteralPath $vendoredDir -Filter "WinDivert*" | Format-Table Name, Length, LastWriteTime -AutoSize
}
finally {
    $env:PATH = $oldPath
}
