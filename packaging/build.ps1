param([string]$Iscc = "$PSScriptRoot/tools/inno/ISCC.exe", [string]$ModelDirectory, [string]$OnnxDll, [string]$VCRedistDirectory)
$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path $PSScriptRoot -Parent
Set-Location $projectRoot
if (-not (Test-Path -LiteralPath "$PSScriptRoot/vendor/goutoujunshi/SKILL.md")) { & "$PSScriptRoot/fetch-skill.ps1" }
if (-not (Test-Path -LiteralPath $Iscc)) { throw 'Install Inno Setup 6 and pass -Iscc.' }
& "$PSScriptRoot/prepare-runtime.ps1" -ModelDirectory $ModelDirectory -OnnxDll $OnnxDll -VCRedistDirectory $VCRedistDirectory
& "$PSScriptRoot/build-frontend.ps1"
Push-Location desktop
try {
 & wails build -s -m -nosyncgomod -o chat-junshi-installer.exe
 if ($LASTEXITCODE -ne 0) { throw 'Go desktop build failed.' }
} finally { Pop-Location }
& "$PSScriptRoot/assemble.ps1"
& $Iscc "$PSScriptRoot/installer.iss"
if ($LASTEXITCODE -ne 0) { throw 'Installer build failed.' }
Get-FileHash -LiteralPath "$projectRoot/release/ChatJunshi-Setup-0.1.0-x64.exe" -Algorithm SHA256 | Format-List
