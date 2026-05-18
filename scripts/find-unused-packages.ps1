param(
  [string]$Root = (Get-Location).Path,
  [switch]$IncludeSimulationPreview
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

Push-Location $Root
try {
  # 1) Discover all main packages (binaries)
  $mainPkgs = @(go list -f '{{if eq .Name "main"}}{{.ImportPath}}{{end}}' ./... | Where-Object { $_ -and $_.Trim().Length -gt 0 })

  if (-not $IncludeSimulationPreview) {
    # simulation_preview is a dev-only adapter; exclude unless explicitly requested
    $mainPkgs = $mainPkgs | Where-Object { $_ -notmatch '/simulation_preview$' }
  }

  if ($mainPkgs.Count -eq 0) {
    Write-Error 'No main packages found under ./...'
  }

  # 2) Collect dependency closure of all binaries
  $used = New-Object 'System.Collections.Generic.HashSet[string]'
  foreach ($m in $mainPkgs) {
    Write-Host "[deps] $m" -ForegroundColor DarkGray
    $deps = @(go list -deps $m)
    foreach ($d in $deps) { [void]$used.Add($d) }
  }

  # 3) All packages in repo
  $all = @(go list ./...)

  # 4) Unused-in-production packages = repo packages not in dependency closure
  $unused = @()
  foreach ($p in $all) {
    if (-not $used.Contains($p)) {
      $unused += $p
    }
  }

  $report = [PSCustomObject]@{
    Root = $Root
    Binaries = $mainPkgs
    UsedPackageCount = $used.Count
    RepoPackageCount = $all.Count
    UnusedPackages = $unused
  }

  $unused | Sort-Object | ForEach-Object { $_ }

  $outPath = Join-Path $Root 'analysis-unused-packages.json'
  $report | ConvertTo-Json -Depth 5 | Out-File -FilePath $outPath -Encoding utf8
  Write-Host "\nWrote report: $outPath" -ForegroundColor Green

  Write-Host "\nNotes:" -ForegroundColor Yellow
  Write-Host "- This flags packages not reachable from ANY main binary (production-dead candidates)." -ForegroundColor Yellow
  Write-Host "- Test-only helpers will show up as unused (expected)." -ForegroundColor Yellow
  Write-Host "- Run with -IncludeSimulationPreview to treat simulation_preview as a binary." -ForegroundColor Yellow
}
finally {
  Pop-Location
}
