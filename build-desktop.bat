@echo off
cd /d "%~dp0desktop"
set "BUNDLED_NODE=%USERPROFILE%\.cache\codex-runtimes\codex-primary-runtime\dependencies\node\bin\node.exe"
if not exist "%BUNDLED_NODE%" goto standard
pushd frontend
for %%I in (npm.cmd) do set "NPM_CMD=%%~$PATH:I"
for %%I in ("%NPM_CMD%") do set "NPM_CLI=%%~dpInode_modules\npm\bin\npm-cli.js"
"%BUNDLED_NODE%" "%NPM_CLI%" install --include=optional --no-audit --no-fund
if errorlevel 1 goto failed
"%BUNDLED_NODE%" node_modules\vite\bin\vite.js build
if errorlevel 1 goto failed
popd
wails build -s -o chat-junshi.exe
goto done
:standard
wails build -o chat-junshi.exe
:done
if errorlevel 1 pause
exit /b
:failed
popd
pause
