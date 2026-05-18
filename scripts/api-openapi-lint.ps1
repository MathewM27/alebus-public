Param(
  [string]$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
)

$ErrorActionPreference = 'Stop'

Push-Location $RepoRoot
try {
  npx -y @redocly/cli@latest lint --config .redocly.yaml api/openapi.yaml
}
finally {
  Pop-Location
}
