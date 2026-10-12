param([string]$Iscc = "$PSScriptRoot\tools\inno\ISCC.exe")
$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path $PSScriptRoot -Parent
Set-Location $projectRoot
$python = Join-Path $projectRoot '.venv\Scripts\python.exe'
$skill = Join-Path $PSScriptRoot 'vendor\goutoujunshi'
if (-not (Test-Path "$skill\SKILL.md")) { throw 'Run packaging/fetch-skill.py first.' }
if (-not (Test-Path $Iscc)) { throw 'Install Inno Setup 6 and pass -Iscc with the compiler path.' }
& $python -m PyInstaller --noconfirm --onedir --console --name junshi-service --distpath packaging/dist/service --workpath packaging/work --specpath packaging/work --collect-all rapidocr_onnxruntime --collect-all windows_capture --collect-all onnxruntime --hidden-import sqlite3 --hidden-import argparse --add-data "$skill;skill/goutoujunshi" desktop_service.py
if ($LASTEXITCODE -ne 0) { throw 'Python service build failed' }
Push-Location desktop
try {
 & wails build -s -m -nosyncgomod -o chat-junshi-installer.exe
 if ($LASTEXITCODE -ne 0) { throw 'Desktop build failed' }
} finally { Pop-Location }
& $python packaging/assemble.py
if ($LASTEXITCODE -ne 0) { throw 'Assembly failed' }
& $Iscc packaging/installer.iss
if ($LASTEXITCODE -ne 0) { throw 'Installer build failed' }
Get-FileHash release/ChatJunshi-Setup-0.1.0-x64.exe -Algorithm SHA256 | Format-List
