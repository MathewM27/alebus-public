Param(
  [string]$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
)

$ErrorActionPreference = 'Stop'

Push-Location (Join-Path $RepoRoot 'api\clients\ts')
try {
  npm install
}
finally {
  Pop-Location
}
