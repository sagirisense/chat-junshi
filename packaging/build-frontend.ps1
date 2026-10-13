param([string]$Node)
$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path $PSScriptRoot -Parent
if (-not $Node) {
 $bundled = Join-Path $env:USERPROFILE '.cache/codex-runtimes/codex-primary-runtime/dependencies/node/bin/node.exe'
 if (Test-Path -LiteralPath $bundled) { $Node=$bundled } else { $Node=(Get-Command node -ErrorAction Stop).Source }
}
$version = (& $Node --version).TrimStart('v')
if ([version]$version -lt [version]'20.19.0') { throw 'Frontend needs Node 20.19+ or 22.12+.' }
Push-Location "$projectRoot/desktop/frontend"
try {
 if (-not (Test-Path -LiteralPath 'node_modules/vite/bin/vite.js')) {
  $npm = (Get-Command npm.cmd -ErrorAction Stop).Source
  $npmCLI = Join-Path (Split-Path $npm -Parent) 'node_modules/npm/bin/npm-cli.js'
  & $Node $npmCLI ci
  if ($LASTEXITCODE -ne 0) { throw 'Frontend dependency installation failed.' }
 }
 & $Node node_modules/vite/bin/vite.js build
 if ($LASTEXITCODE -ne 0) { throw 'Frontend build failed.' }
} finally { Pop-Location }
