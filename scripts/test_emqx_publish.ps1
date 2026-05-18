# ═══════════════════════════════════════════════════════════════════════════════
# EMQX Telemetry Test Publisher
# ═══════════════════════════════════════════════════════════════════════════════
#
# This script sends properly formatted JSON messages to the EMQX broker for testing
# the ingestor pipeline. It handles PowerShell/Docker quote escaping correctly.
#
# USAGE:
#   .\scripts\test_emqx_publish.ps1
#
# PREREQUISITES:
#   - docker compose -f docker-compose.emqx.yml up -d
#   - Stack healthy: http://localhost:9100/healthz
#
# ═══════════════════════════════════════════════════════════════════════════════

# ─────────────────────────────────────────────────────────────────────────────
# REQUIRED JSON FIELDS (per types.go and live_bus_write_ports.go)
# ─────────────────────────────────────────────────────────────────────────────
#
# RawTelemetryMessage validation (infrastructure/mqtt/types.go):
#   - bus_id:       required, non-empty string
#   - route_id:     required, non-empty string
#   - direction:    must be 0 or 1
#   - timestamp_ms: required, must be > 0 (Unix milliseconds)
#
# LiveBusUpdate validation (application/journey/ports/live_bus_write_ports.go):
#   - StopIndex:    must be >= 0
#   - Lat:          must be in range [-90, 90]
#   - Lon:          must be in range [-180, 180]
#   - SpeedKmh:     must be >= 0
#   - Status:       MUST BE "active", "inactive", OR "delayed" (not empty!)
#   - Timestamp:    must not be in the future (5s tolerance)
#
# ─────────────────────────────────────────────────────────────────────────────

param(
    [string]$BusID = "BUS-TEST-001",
    [string]$RouteID = "ROUTE-TEST-001",
    [int]$Direction = 0,
    [int]$StopIndex = 1,
    [bool]$IsAtTerminal = $false,
    [double]$Lat = -33.8688,
    [double]$Lon = 151.2093,
    [double]$SpeedKmh = 25.5,
    [string]$Status = "active",  # REQUIRED: must be "active", "inactive", or "delayed"
    [int]$Count = 1
)

Write-Host "═══════════════════════════════════════════════════════════════════════════════"
Write-Host "EMQX Telemetry Test Publisher"
Write-Host "═══════════════════════════════════════════════════════════════════════════════"

# Check if ingestor is healthy
Write-Host "`nChecking ingestor health..."
try {
    $health = Invoke-RestMethod -Uri "http://localhost:9100/healthz" -TimeoutSec 5
    Write-Host "  Status: $($health.status)"
    Write-Host "  Redis:  $($health.redis)"
    Write-Host "  MQTT:   $($health.mqtt)"
    if ($health.status -eq "unhealthy") {
        Write-Host "`n❌ Ingestor is unhealthy! Check docker compose logs." -ForegroundColor Red
        exit 1
    }
} catch {
    Write-Host "❌ Could not connect to ingestor at localhost:9100" -ForegroundColor Red
    Write-Host "   Make sure the stack is running: docker compose -f docker-compose.emqx.yml up -d"
    exit 1
}

# Get current metrics
Write-Host "`nCurrent metrics:"
$metricsBefore = Invoke-WebRequest -Uri "http://localhost:9100/metrics" -UseBasicParsing
$received = ($metricsBefore.Content | Select-String 'alebus_ingestor_received_total\s+(\d+)').Matches[0].Groups[1].Value
$accepted = ($metricsBefore.Content | Select-String 'alebus_ingestor_accepted_total\s+(\d+)').Matches[0].Groups[1].Value
$invalid = ($metricsBefore.Content | Select-String 'alebus_ingestor_invalid_total\s+(\d+)').Matches[0].Groups[1].Value
Write-Host "  Received: $received"
Write-Host "  Accepted: $accepted"
Write-Host "  Invalid:  $invalid"

Write-Host "`n───────────────────────────────────────────────────────────────────────────────"
Write-Host "Publishing $Count message(s)..."
Write-Host "───────────────────────────────────────────────────────────────────────────────"

for ($i = 1; $i -le $Count; $i++) {
    # Generate current timestamp in milliseconds
    $timestampMs = [DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()
    
    # Build bus ID with counter if sending multiple
    $currentBusId = if ($Count -gt 1) { "$BusID-$i" } else { $BusID }
    
    # Build the JSON message - use ConvertTo-Json for proper escaping
    $message = @{
        bus_id        = $currentBusId
        route_id      = $RouteID
        direction     = $Direction
        stop_index    = $StopIndex
        is_at_terminal = $IsAtTerminal
        lat           = $Lat
        lon           = $Lon
        speed_kmh     = $SpeedKmh
        status        = $Status
        timestamp_ms  = $timestampMs
    } | ConvertTo-Json -Compress

    Write-Host "`n[$i/$Count] Publishing to bus/$currentBusId/telemetry"
    Write-Host "  Message: $message"
    
    # Write message to a temp file WITHOUT BOM (critical!)
    # PowerShell's Out-File -Encoding utf8 adds a BOM which breaks JSON parsing in Go
    # Use Set-Content -Encoding ASCII or OEM to avoid BOM
    $localTempFile = ".\temp_msg_$i.json"
    $containerTempFile = "/tmp/msg_$i.json"
    
    # Write without BOM
    $message | Set-Content -Path $localTempFile -NoNewline -Encoding ASCII
    
    # Copy to container (avoids Docker exec quote escaping issues)
    docker cp $localTempFile "alebus-mqtt-publisher:$containerTempFile" 2>&1 | Out-Null
    
    # Publish using the file
    $result = docker exec alebus-mqtt-publisher mosquitto_pub -h emqx -p 1883 -t "bus/$currentBusId/telemetry" -f $containerTempFile 2>&1
    
    if ($LASTEXITCODE -ne 0) {
        Write-Host "  ❌ Failed to publish: $result" -ForegroundColor Red
    } else {
        Write-Host "  ✅ Published successfully" -ForegroundColor Green
    }
    
    # Clean up temp files
    Remove-Item -Path $localTempFile -Force -ErrorAction SilentlyContinue
    docker exec alebus-mqtt-publisher rm -f $containerTempFile 2>&1 | Out-Null
    
    # Small delay between messages
    if ($i -lt $Count) {
        Start-Sleep -Milliseconds 100
    }
}

# Wait a moment for processing
Write-Host "`nWaiting for processing..."
Start-Sleep -Seconds 1

# Check Redis for the bus state
Write-Host "`n───────────────────────────────────────────────────────────────────────────────"
Write-Host "Checking Redis state..."
Write-Host "───────────────────────────────────────────────────────────────────────────────"

$keys = docker exec alebus-redis-emqx redis-cli KEYS "bus:*:state" 2>&1
if ($keys) {
    Write-Host "Bus state keys found:"
    $keys -split "`n" | ForEach-Object { 
        if ($_ -match "bus:(.+):state") {
            $busKey = $_
            Write-Host "`n  Key: $busKey"
            $state = docker exec alebus-redis-emqx redis-cli HGETALL $busKey 2>&1
            Write-Host "  Data: $state"
        }
    }
} else {
    Write-Host "  No bus state keys found in Redis"
}

# Check updated metrics
Write-Host "`n───────────────────────────────────────────────────────────────────────────────"
Write-Host "Updated metrics:"
Write-Host "───────────────────────────────────────────────────────────────────────────────"
$metricsAfter = Invoke-WebRequest -Uri "http://localhost:9100/metrics" -UseBasicParsing
$receivedAfter = ($metricsAfter.Content | Select-String 'alebus_ingestor_received_total\s+(\d+)').Matches[0].Groups[1].Value
$acceptedAfter = ($metricsAfter.Content | Select-String 'alebus_ingestor_accepted_total\s+(\d+)').Matches[0].Groups[1].Value
$invalidAfter = ($metricsAfter.Content | Select-String 'alebus_ingestor_invalid_total\s+(\d+)').Matches[0].Groups[1].Value
$staleAfter = ($metricsAfter.Content | Select-String 'alebus_ingestor_stale_total\s+(\d+)').Matches[0].Groups[1].Value

$deltaReceived = [int]$receivedAfter - [int]$received
$deltaAccepted = [int]$acceptedAfter - [int]$accepted
$deltaInvalid = [int]$invalidAfter - [int]$invalid

Write-Host "  Received: $receivedAfter (+$deltaReceived)"
Write-Host "  Accepted: $acceptedAfter (+$deltaAccepted)"
Write-Host "  Invalid:  $invalidAfter (+$deltaInvalid)"
Write-Host "  Stale:    $staleAfter"

Write-Host "`n───────────────────────────────────────────────────────────────────────────────"
if ($deltaAccepted -gt 0) {
    Write-Host "✅ SUCCESS: $deltaAccepted message(s) accepted and written to Redis!" -ForegroundColor Green
} elseif ($deltaInvalid -gt 0) {
    Write-Host "❌ VALIDATION FAILED: $deltaInvalid message(s) were invalid" -ForegroundColor Red
    Write-Host "   Check that all required fields are present and valid:"
    Write-Host "   - status must be 'active', 'inactive', or 'delayed'"
    Write-Host "   - lat must be in range [-90, 90]"
    Write-Host "   - lon must be in range [-180, 180]"
    Write-Host "   - timestamp_ms must be current (not future)"
} elseif ($deltaReceived -eq 0) {
    Write-Host "⚠️  WARNING: No messages received. Check MQTT connection." -ForegroundColor Yellow
} else {
    Write-Host "⚠️  WARNING: Messages received but not accepted. Check ingestor logs." -ForegroundColor Yellow
    docker logs alebus-ingestor --tail 20
}
Write-Host "───────────────────────────────────────────────────────────────────────────────"
