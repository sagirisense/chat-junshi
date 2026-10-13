param([string]$DesktopBinary)
$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path $PSScriptRoot -Parent
if (-not $DesktopBinary) { $DesktopBinary = Join-Path $projectRoot 'desktop/build/bin/chat-junshi-installer.exe' }
$dist = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot 'dist'))
$target = [IO.Path]::GetFullPath((Join-Path $dist 'ChatJunshi'))
if ((Split-Path $target -Parent) -ne $dist) { throw 'Unsafe assembly path.' }
if (Test-Path -LiteralPath $target) {
 if ((Get-Item -LiteralPath $target).Attributes -band [IO.FileAttributes]::ReparsePoint) { throw 'Assembly target must not be a link.' }
 Remove-Item -LiteralPath $target -Recurse -Force
}
New-Item -ItemType Directory -Force -Path "$target/runtime/models", "$target/runtime/skill/goutoujunshi", "$target/licenses" | Out-Null
Copy-Item -LiteralPath $DesktopBinary -Destination "$target/chat-junshi.exe"
$runtime = Join-Path $projectRoot '.go-cache/runtime'
foreach ($name in @('onnxruntime.dll','junshi_wgc.dll','ocr-source.json','msvcp140.dll','msvcp140_1.dll','vcruntime140.dll','vcruntime140_1.dll')) { Copy-Item -LiteralPath "$runtime/$name" -Destination "$target/runtime/$name" }
foreach ($name in @('ch_PP-OCRv4_det_infer.onnx','ch_PP-OCRv4_rec_infer.onnx','ppocr_keys_v5.txt')) { Copy-Item -LiteralPath "$runtime/models/$name" -Destination "$target/runtime/models/$name" }
$skill = Join-Path $PSScriptRoot 'vendor/goutoujunshi'
foreach ($name in @('SKILL.md','LICENSE')) { Copy-Item -LiteralPath "$skill/$name" -Destination "$target/runtime/skill/goutoujunshi/$name" }
Get-ChildItem -LiteralPath "$skill/references" -Recurse -File -Filter '*.md' | ForEach-Object {
 $relative = $_.FullName.Substring($skill.Length + 1)
 $destination = Join-Path "$target/runtime/skill/goutoujunshi" $relative
 New-Item -ItemType Directory -Force -Path (Split-Path $destination -Parent) | Out-Null
 Copy-Item -LiteralPath $_.FullName -Destination $destination
}
foreach ($name in @('LICENSE','NOTICE')) { Copy-Item -LiteralPath "$projectRoot/$name" -Destination "$target/$name" }
Get-ChildItem -LiteralPath "$projectRoot/licenses" -File | ForEach-Object { Copy-Item -LiteralPath $_.FullName -Destination "$target/licenses/$($_.Name)" }
Copy-Item -LiteralPath "$skill/LICENSE" -Destination "$target/licenses/goutoujunshi-LICENSE"
Copy-Item -LiteralPath "$PSScriptRoot/skill-source.json" -Destination "$target/skill-source.json"

# Include module license files, never module sources or runtime/user data.
Push-Location "$projectRoot/desktop"
try {
 $modules = & go list -deps -f '{{if .Module}}{{.Module.Path}}|{{.Module.Version}}|{{.Module.Dir}}{{end}}' . | Sort-Object -Unique
 if ($LASTEXITCODE -ne 0) { throw 'Go dependency enumeration failed.' }
} finally { Pop-Location }
foreach ($module in $modules) {
 $parts = $module.Split('|')
 if ($parts.Length -ne 3 -or -not $parts[1] -or -not $parts[2]) { continue }
 $folder = Join-Path "$target/licenses/go" ($parts[0].Replace('/','__') + '@' + $parts[1])
 Get-ChildItem -LiteralPath $parts[2] -File | Where-Object { $_.Name -match '^(LICENSE|LICENCE|NOTICE|COPYING)' } | ForEach-Object {
  New-Item -ItemType Directory -Force -Path $folder | Out-Null
  Copy-Item -LiteralPath $_.FullName -Destination (Join-Path $folder $_.Name)
 }
}
# Rust crates linked into the WGC bridge retain their own license texts.
$crateMetadata = & cargo metadata --offline --locked --format-version 1 --manifest-path "$projectRoot/desktop/native/wgc/Cargo.toml" | ConvertFrom-Json
if ($LASTEXITCODE -ne 0) { throw 'Rust dependency enumeration failed.' }
foreach ($crate in $crateMetadata.packages) {
 if (-not $crate.source) { continue }
 $folder = Join-Path "$target/licenses/rust" "$($crate.name)@$($crate.version)"
 Get-ChildItem -LiteralPath (Split-Path $crate.manifest_path -Parent) -File | Where-Object { $_.Name -match '^(LICENSE|LICENCE|NOTICE|COPYING)' } | ForEach-Object {
  New-Item -ItemType Directory -Force -Path $folder | Out-Null
  Copy-Item -LiteralPath $_.FullName -Destination (Join-Path $folder $_.Name)
 }
}
Get-ChildItem -LiteralPath "$projectRoot/desktop/frontend/node_modules" -Recurse -File | Where-Object { $_.Name -match '^(LICENSE|LICENCE|NOTICE|COPYING)(\..*)?$' } | ForEach-Object {
 $relative = $_.FullName.Substring((Join-Path $projectRoot 'desktop/frontend/node_modules').Length + 1)
 $destination = Join-Path "$target/licenses/frontend" $relative
 New-Item -ItemType Directory -Force -Path (Split-Path $destination -Parent) | Out-Null
 Copy-Item -LiteralPath $_.FullName -Destination $destination
}
foreach ($folder in @('onnxruntime','rapidocr_onnxruntime')) {
 $dependency = Join-Path $projectRoot ".venv/Lib/site-packages/$folder"
 if (Test-Path -LiteralPath $dependency) {
  Get-ChildItem -LiteralPath $dependency -File | Where-Object { $_.Name -match 'LICENSE|NOTICE|COPYING' } | ForEach-Object {
   New-Item -ItemType Directory -Force -Path "$target/licenses/ocr/$folder" | Out-Null
   Copy-Item -LiteralPath $_.FullName -Destination "$target/licenses/ocr/$folder/$($_.Name)"
  }
 }
}
Get-ChildItem -LiteralPath "$projectRoot/licenses" -Directory | ForEach-Object { Copy-Item -LiteralPath $_.FullName -Destination "$target/licenses" -Recurse -Force }
@'
聊天军师 0.1.0 · Go 迁移版
内置原生WGC、OCR和公开MIT狗头军师规则；无需Python、Go、Rust或Node运行时。
用户数据：%LOCALAPPDATA%\ChatJunshi\data，升级首次打开旧数据前自动备份。
卸载默认保留数据；Codex模式需另行安装并登录CLI，自定义API需自己的密钥。
需要Windows 10 1903+ x64 / Windows 11与WebView2 Runtime。
来源及许可证见NOTICE、licenses；源码：https://github.com/sagirisense/chat-junshi
'@ | Set-Content -LiteralPath "$target/INSTALL-README.txt" -Encoding UTF8
$manifest = @()
foreach ($file in Get-ChildItem -LiteralPath $target -Recurse -File | Sort-Object FullName) {
 $relative = $file.FullName.Substring($target.Length + 1).Replace('\','/')
 if ($file.Extension -in @('.py','.pyc','.pyd','.db','.sqlite','.sqlite3','.log') -or $file.Name -match '^python.*\.dll$' -or $file.Name -in @('auth.json','.env','selected-session.json') -or $relative -match '(^|/)data/') { throw "Unexpected runtime or private file: $relative" }
 $manifest += @{ path=$relative; size=$file.Length; sha256=(Get-FileHash -LiteralPath $file.FullName -Algorithm SHA256).Hash.ToLowerInvariant() }
}
$manifest | ConvertTo-Json -Depth 6 | Set-Content -LiteralPath "$target/payload-manifest.json" -Encoding UTF8
Write-Output "Go installer payload files: $($manifest.Count)"
