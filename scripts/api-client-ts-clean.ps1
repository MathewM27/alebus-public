Param(
  [string]$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
)

$ErrorActionPreference = 'Stop'

$clientRoot = Join-Path $RepoRoot 'api\clients\ts'

$paths = @(
  (Join-Path $clientRoot 'dist'),
  (Join-Path $clientRoot 'node_modules'),
  (Join-Path $clientRoot 'package-lock.json')
)

foreach ($p in $paths) {
  if (Test-Path $p) {
    Remove-Item -LiteralPath $p -Force -Recurse
  }
}

Write-Host "Cleaned TS client artifacts." -ForegroundColor Green
