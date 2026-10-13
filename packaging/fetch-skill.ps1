$ErrorActionPreference = 'Stop'
$source = Get-Content -LiteralPath "$PSScriptRoot/skill-source.json" -Raw | ConvertFrom-Json
if ($source.repository -ne 'https://github.com/shengjidaguai-china/goutoujunshi' -or $source.commit -notmatch '^[a-f0-9]{40}$') { throw 'Invalid public Skill source.' }
$projectRoot = Split-Path $PSScriptRoot -Parent
$cache = Join-Path $projectRoot '.go-cache'
New-Item -ItemType Directory -Force -Path $cache | Out-Null
$archivePath = Join-Path $cache 'public-skill.zip'
Invoke-WebRequest -Uri "https://codeload.github.com/shengjidaguai-china/goutoujunshi/zip/$($source.commit)" -OutFile $archivePath
Add-Type -AssemblyName System.IO.Compression.FileSystem
$bundle = [IO.Compression.ZipFile]::OpenRead($archivePath)
$destination = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot 'vendor/goutoujunshi'))
try {
 foreach ($entry in $bundle.Entries) {
  $parts = $entry.FullName.Split('/')
  if ($parts.Length -lt 2) { continue }
  $relative = ($parts | Select-Object -Skip 1) -join '/'
  $allowed = $relative -in @('SKILL.md','LICENSE','LICENSE.zh-CN.md') -or ($relative.StartsWith('references/') -and $relative.EndsWith('.md'))
  if (-not $allowed -or $parts -contains '..') { continue }
  $target = [IO.Path]::GetFullPath((Join-Path $destination $relative))
  if (-not $target.StartsWith($destination + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)) { throw 'Unsafe Skill archive path.' }
  New-Item -ItemType Directory -Force -Path (Split-Path $target -Parent) | Out-Null
  [IO.Compression.ZipFileExtensions]::ExtractToFile($entry,$target,$true)
 }
} finally { $bundle.Dispose() }
$source | ConvertTo-Json | Set-Content -LiteralPath "$PSScriptRoot/vendor/skill-version.json" -Encoding UTF8
Write-Output 'Pinned public Skill fetched; no personal data or Python scripts copied.'
