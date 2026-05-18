Param(
  [string]$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
)

$ErrorActionPreference = 'Stop'

Push-Location (Join-Path $RepoRoot 'api\clients\ts')
try {
  npm run typecheck
}
finally {
  Pop-Location
}
