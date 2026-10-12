@echo off
cd /d "%~dp0"
if not exist "desktop\build\bin\chat-junshi.exe" (
  echo Please run build-desktop.bat first.
  pause
  exit /b 1
)
start "" "desktop\build\bin\chat-junshi.exe"
