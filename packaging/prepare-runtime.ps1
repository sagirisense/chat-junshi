param([string]$ModelDirectory, [string]$OnnxDll, [string]$RustToolchain = 'stable-x86_64-pc-windows-gnu', [string]$VCRedistDirectory)
$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path $PSScriptRoot -Parent
function Copy-RuntimeAsset([string]$Source, [string]$Destination) {
 if ((Test-Path -LiteralPath $Destination -PathType Leaf) -and (Get-FileHash -LiteralPath $Source -Algorithm SHA256).Hash -eq (Get-FileHash -LiteralPath $Destination -Algorithm SHA256).Hash) { return }
 Copy-Item -LiteralPath $Source -Destination $Destination -Force
}
if (-not $ModelDirectory) { $ModelDirectory = Join-Path $projectRoot '.venv/Lib/site-packages/rapidocr_onnxruntime/models' }
if (-not $OnnxDll) { $OnnxDll = Join-Path $projectRoot '.venv/Lib/site-packages/onnxruntime/capi/onnxruntime.dll' }
$source = Get-Content -LiteralPath "$PSScriptRoot/ocr-source.json" -Raw | ConvertFrom-Json
$inputs = @{
 'ch_PP-OCRv4_det_infer.onnx' = Join-Path $ModelDirectory 'ch_PP-OCRv4_det_infer.onnx'
 'ch_PP-OCRv4_rec_infer.onnx' = Join-Path $ModelDirectory 'ch_PP-OCRv4_rec_infer.onnx'
 'onnxruntime.dll' = $OnnxDll
}
foreach ($name in $inputs.Keys) {
 if (-not (Test-Path -LiteralPath $inputs[$name] -PathType Leaf)) { throw "Provide pinned OCR asset: $name via -ModelDirectory / -OnnxDll." }
 if ((Get-FileHash -LiteralPath $inputs[$name] -Algorithm SHA256).Hash -ne $source.sha256.$name) { throw "OCR asset hash mismatch: $name" }
}
$runtime = Join-Path $projectRoot '.go-cache/runtime'
New-Item -ItemType Directory -Force -Path "$runtime/models" | Out-Null
foreach ($name in $inputs.Keys) {
 $target = if ($name.EndsWith('.onnx')) { Join-Path "$runtime/models" $name } else { Join-Path $runtime $name }
 Copy-RuntimeAsset $inputs[$name] $target
}
if (-not $VCRedistDirectory) {
 $redistRoot = 'C:/Program Files/Microsoft Visual Studio/2022/Community/VC/Redist/MSVC'
 $VCRedistDirectory = Get-ChildItem -LiteralPath $redistRoot -Directory | Where-Object { $_.Name -match '^\d' } | Sort-Object Name -Descending | ForEach-Object { Join-Path $_.FullName 'x64/Microsoft.VC143.CRT' } | Select-Object -First 1
}
foreach ($name in @('msvcp140.dll','msvcp140_1.dll','vcruntime140.dll','vcruntime140_1.dll')) {
 if (-not (Test-Path -LiteralPath "$VCRedistDirectory/$name")) { throw "Provide VC++ redistributable directory for $name." }
 Copy-RuntimeAsset "$VCRedistDirectory/$name" "$runtime/$name"
}
& cargo "+$RustToolchain" build --release --locked --manifest-path "$projectRoot/desktop/native/wgc/Cargo.toml"
if ($LASTEXITCODE -ne 0) { throw 'Native WGC build failed.' }
Copy-RuntimeAsset "$projectRoot/desktop/native/wgc/target/release/junshi_wgc.dll" "$runtime/junshi_wgc.dll"
Push-Location "$projectRoot/desktop"
try {
 & go build -o "$projectRoot/.go-cache/prepare-ocr.exe" ./cmd/prepare-ocr
 if ($LASTEXITCODE -ne 0) { throw 'OCR asset tool build failed.' }
} finally { Pop-Location }
& "$projectRoot/.go-cache/prepare-ocr.exe" "$runtime/models"
if ($LASTEXITCODE -ne 0) { throw 'Model dictionary extraction failed.' }
Copy-Item -LiteralPath "$PSScriptRoot/ocr-source.json" -Destination "$runtime/ocr-source.json" -Force
Write-Output "Runtime assets prepared: $runtime"
