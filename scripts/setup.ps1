#Requires -Version 5.1
<#
  Plug-and-play setup of FocusGuard for Windows.
  - Auto-elevates to Administrator if necessary.
  - Compiles the binary (requires Go).
  - Installs it into Program Files.
  - Registers a Scheduled Task that launches it, elevated, at every logon.

  Usage:
    .\scripts\setup.ps1              installs and starts
    .\scripts\setup.ps1 -Uninstall   uninstalls everything and cleans up the hosts file
#>

param(
  [switch]$Uninstall
)

$ErrorActionPreference = "Stop"
$TaskName = "FocusGuard"
$InstallDir = Join-Path $Env:ProgramFiles "FocusGuard"
$ExePath = Join-Path $InstallDir "focusguard.exe"
$Port = 7878

function Write-Step($msg) { Write-Host "  -> $msg" -ForegroundColor Cyan }
function Write-Bold($msg) { Write-Host $msg -ForegroundColor White -BackgroundColor DarkBlue }
function Write-ErrLine($msg) { Write-Host "  !! $msg" -ForegroundColor Red }

# --- auto-elevation ---
$isAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
  Write-Step "Administrator privileges required. Reopening elevated..."
  $argList = @("-NoProfile", "-ExecutionPolicy", "Bypass", "-File", "`"$PSCommandPath`"")
  if ($Uninstall) { $argList += "-Uninstall" }
  Start-Process powershell -Verb RunAs -ArgumentList $argList
  exit
}

function Remove-FocusGuard {
  Write-Bold "Uninstalling FocusGuard..."
  schtasks /end /tn $TaskName 2>$null | Out-Null
  schtasks /delete /tn $TaskName /f 2>$null | Out-Null
  Get-Process focusguard -ErrorAction SilentlyContinue | Stop-Process -Force

  if (Test-Path $InstallDir) {
    Remove-Item -Recurse -Force $InstallDir
  }

  # Cleans up the managed block in the hosts file.
  $hostsPath = "$Env:WINDIR\System32\drivers\etc\hosts"
  if (Test-Path $hostsPath) {
    $content = Get-Content $hostsPath -Raw
    $pattern = "(?s)# === FocusGuard START.*?# === FocusGuard END ===\r?\n?"
    $clean = [regex]::Replace($content, $pattern, "")
    Set-Content -Path $hostsPath -Value $clean -NoNewline
    Write-Step "hosts file cleaned up"
  }

  Write-Step "Done."
  exit 0
}

if ($Uninstall) { Remove-FocusGuard }

Write-Bold "FocusGuard - setup for Windows"

# 1. Go
$go = Get-Command go -ErrorAction SilentlyContinue
if (-not $go) {
  Write-Step "Go is not installed. Attempting to install it with winget..."
  try {
    winget install -e --id GoLang.Go --accept-source-agreements --accept-package-agreements
    $Env:Path += ";$Env:ProgramFiles\Go\bin"
  } catch {
    Write-ErrLine "Could not install Go automatically."
    Write-ErrLine "Please install it manually from https://go.dev/dl/ and run this script again."
    exit 1
  }
}
Write-Step "Go found: $(go version)"

# 2. Build
Write-Bold "Compiling..."
$RepoRoot = Split-Path -Parent $PSScriptRoot
Push-Location $RepoRoot
New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
go build -o $ExePath ./cmd/focusguard
Pop-Location
Write-Step "Build OK -> $ExePath"

# 3. Scheduled task that runs elevated at every logon
schtasks /create /tn $TaskName /tr "`"$ExePath`" -no-browser" /sc onlogon /rl highest /f | Out-Null
Write-Step "Scheduled task '$TaskName' registered (runs elevated at every logon)"

# 4. Start it right now
Start-Process -FilePath $ExePath
Start-Sleep -Seconds 1

Write-Bold "Done. FocusGuard is running."
Write-Host ""
Write-Host "  Web UI:      http://localhost:$Port"
Write-Host "  Uninstall:   .\scripts\setup.ps1 -Uninstall"
Write-Host ""
Write-Host "  Note: Windows SmartScreen may warn you that the .exe is unsigned" -ForegroundColor DarkYellow
Write-Host "  (this is normal for locally compiled binaries). Click 'More info' -> 'Run anyway'." -ForegroundColor DarkYellow
