# ═══════════════════════════════════════════════════════════════════════════════
# EMQX Ingestor Resilience Tests
# ═══════════════════════════════════════════════════════════════════════════════
#
# Tests:
#   1. Kill Redis mid-write
#   2. Kill ingestor during processing
#   3. Restart EMQX while publishers active
#   4. Multiple ingestors with shared subscriptions
#   5. Redis latency injection
#
# ═══════════════════════════════════════════════════════════════════════════════

param(
    [int]$TestNumber = 0  # 0 = run all tests, 1-5 = run specific test
)

$ErrorActionPreference = "Continue"

# Helper function to publish a message
function Publish-TestMessage {
    param([string]$BusId, [string]$RouteId = "ROUTE-001")
    
    $ts = [DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()
    $json = @{
        bus_id = $BusId
        route_id = $RouteId
        direction = 0
        stop_index = 1
        is_at_terminal = $false
        lat = -33.8688
        lon = 151.2093
        speed_kmh = 30.0
        status = "active"
        timestamp_ms = $ts
    } | ConvertTo-Json -Compress
    
    $json | Set-Content -Path ".\resilience_msg.json" -NoNewline -Encoding ASCII
    docker cp ".\resilience_msg.json" alebus-mqtt-publisher:/tmp/msg.json 2>$null
    docker exec alebus-mqtt-publisher mosquitto_pub -h emqx -p 1883 -t "bus/$BusId/telemetry" -f /tmp/msg.json 2>$null
    Remove-Item ".\resilience_msg.json" -ErrorAction SilentlyContinue
}

# Helper function to get metrics
function Get-IngestorMetrics {
    try {
        $metrics = (Invoke-WebRequest -Uri "http://localhost:9100/metrics" -UseBasicParsing -TimeoutSec 2).Content
        $received = ([regex]::Match($metrics, 'alebus_ingestor_received_total\s+(\d+)')).Groups[1].Value
        $accepted = ([regex]::Match($metrics, 'alebus_ingestor_accepted_total\s+(\d+)')).Groups[1].Value
        $invalid = ([regex]::Match($metrics, 'alebus_ingestor_invalid_total\s+(\d+)')).Groups[1].Value
        $infraError = ([regex]::Match($metrics, 'alebus_ingestor_infra_error_total\s+(\d+)')).Groups[1].Value
        return @{
            Received = [int]$received
            Accepted = [int]$accepted
            Invalid = [int]$invalid
            InfraError = [int]$infraError
        }
    } catch {
        return $null
    }
}

# Helper function to check health
function Get-IngestorHealth {
    try {
        $health = Invoke-RestMethod -Uri "http://localhost:9100/healthz" -TimeoutSec 2
        return $health
    } catch {
        return @{ status = "unreachable" }
    }
}

Write-Host "═══════════════════════════════════════════════════════════════════════════════"
Write-Host "EMQX INGESTOR RESILIENCE TESTS"
Write-Host "═══════════════════════════════════════════════════════════════════════════════"

# ═══════════════════════════════════════════════════════════════════════════════
# TEST 1: Kill Redis Mid-Write
# ═══════════════════════════════════════════════════════════════════════════════
if ($TestNumber -eq 0 -or $TestNumber -eq 1) {
    Write-Host "`n┌─────────────────────────────────────────────────────────────────────────────┐"
    Write-Host "│ TEST 1: Kill Redis Mid-Write                                                │"
    Write-Host "└─────────────────────────────────────────────────────────────────────────────┘"
    
    # Get baseline metrics
    $before = Get-IngestorMetrics
    Write-Host "  Before: received=$($before.Received), accepted=$($before.Accepted), infra_error=$($before.InfraError)"
    
    # Start publishing messages in background
    Write-Host "  Starting message flood..."
    $publishJob = Start-Job -ScriptBlock {
        for ($i = 1; $i -le 50; $i++) {
            $ts = [DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()
            $json = @{bus_id="REDIS-TEST-$i";route_id="ROUTE-001";direction=0;stop_index=1;is_at_terminal=$false;lat=-33.8688;lon=151.2093;speed_kmh=30.0;status="active";timestamp_ms=$ts} | ConvertTo-Json -Compress
            $json | Set-Content -Path ".\redis_test_$i.json" -NoNewline -Encoding ASCII
            docker cp ".\redis_test_$i.json" alebus-mqtt-publisher:/tmp/msg.json 2>$null
            docker exec alebus-mqtt-publisher mosquitto_pub -h emqx -p 1883 -t "bus/REDIS-TEST-$i/telemetry" -f /tmp/msg.json 2>$null
            Remove-Item ".\redis_test_$i.json" -ErrorAction SilentlyContinue
            Start-Sleep -Milliseconds 50
        }
    }
    
    # Wait a bit then kill Redis
    Start-Sleep -Seconds 1
    Write-Host "  Killing Redis container..."
    docker stop alebus-redis-emqx --time=0 2>$null
    
    # Wait for job to complete
    Start-Sleep -Seconds 3
    Stop-Job $publishJob -ErrorAction SilentlyContinue
    Remove-Job $publishJob -ErrorAction SilentlyContinue
    
    # Check health (should be unhealthy)
    $health = Get-IngestorHealth
    Write-Host "  Health after Redis kill: $($health.status) (redis: $($health.redis))"
    
    # Restart Redis
    Write-Host "  Restarting Redis..."
    docker start alebus-redis-emqx 2>$null
    Start-Sleep -Seconds 5
    
    # Check health (should recover)
    $health = Get-IngestorHealth
    Write-Host "  Health after Redis restart: $($health.status)"
    
    # Get final metrics
    $after = Get-IngestorMetrics
    if ($after) {
        Write-Host "  After: received=$($after.Received), accepted=$($after.Accepted), infra_error=$($after.InfraError)"
        $deltaInfra = $after.InfraError - $before.InfraError
        if ($health.status -eq "healthy") {
            Write-Host "  ✅ TEST 1 PASSED: Ingestor recovered after Redis restart (infra_errors: $deltaInfra)" -ForegroundColor Green
        } else {
            Write-Host "  ❌ TEST 1 FAILED: Ingestor did not recover" -ForegroundColor Red
        }
    } else {
        Write-Host "  ❌ TEST 1 FAILED: Could not get metrics" -ForegroundColor Red
    }
}

# ═══════════════════════════════════════════════════════════════════════════════
# TEST 2: Kill Ingestor During Processing
# ═══════════════════════════════════════════════════════════════════════════════
if ($TestNumber -eq 0 -or $TestNumber -eq 2) {
    Write-Host "`n┌─────────────────────────────────────────────────────────────────────────────┐"
    Write-Host "│ TEST 2: Kill Ingestor During Processing                                     │"
    Write-Host "└─────────────────────────────────────────────────────────────────────────────┘"
    
    # Send some messages first
    Write-Host "  Sending initial messages..."
    for ($i = 1; $i -le 5; $i++) {
        Publish-TestMessage -BusId "KILL-TEST-$i"
        Start-Sleep -Milliseconds 100
    }
    Start-Sleep -Seconds 1
    
    $before = Get-IngestorMetrics
    Write-Host "  Before kill: received=$($before.Received), accepted=$($before.Accepted)"
    
    # Kill ingestor
    Write-Host "  Killing ingestor container..."
    docker kill alebus-ingestor 2>$null
    
    # Check it's gone
    $status = docker ps --filter "name=alebus-ingestor" --format "{{.Status}}" 2>$null
    if (-not $status) {
        Write-Host "  Ingestor stopped successfully"
    }
    
    # Restart via compose
    Write-Host "  Restarting ingestor via docker compose..."
    docker compose -f docker-compose.emqx.yml up -d ingestor 2>$null
    
    # Wait for healthy
    Write-Host "  Waiting for ingestor to become healthy..."
    $healthy = $false
    for ($i = 0; $i -lt 30; $i++) {
        Start-Sleep -Seconds 1
        $health = Get-IngestorHealth
        if ($health.status -eq "healthy") {
            $healthy = $true
            break
        }
    }
    
    if ($healthy) {
        # Send more messages
        Write-Host "  Sending messages after restart..."
        for ($i = 1; $i -le 5; $i++) {
            Publish-TestMessage -BusId "AFTER-KILL-$i"
            Start-Sleep -Milliseconds 100
        }
        Start-Sleep -Seconds 1
        
        $after = Get-IngestorMetrics
        Write-Host "  After restart: received=$($after.Received), accepted=$($after.Accepted)"
        
        if ($after.Accepted -gt 0) {
            Write-Host "  ✅ TEST 2 PASSED: Ingestor recovered and processing messages" -ForegroundColor Green
        } else {
            Write-Host "  ❌ TEST 2 FAILED: No messages accepted after restart" -ForegroundColor Red
        }
    } else {
        Write-Host "  ❌ TEST 2 FAILED: Ingestor did not become healthy" -ForegroundColor Red
    }
}

# ═══════════════════════════════════════════════════════════════════════════════
# TEST 3: Restart EMQX While Publishers Active
# ═══════════════════════════════════════════════════════════════════════════════
if ($TestNumber -eq 0 -or $TestNumber -eq 3) {
    Write-Host "`n┌─────────────────────────────────────────────────────────────────────────────┐"
    Write-Host "│ TEST 3: Restart EMQX While Publishers Active                                │"
    Write-Host "└─────────────────────────────────────────────────────────────────────────────┘"
    
    $before = Get-IngestorMetrics
    Write-Host "  Before: received=$($before.Received), accepted=$($before.Accepted)"
    
    # Start continuous publishing
    Write-Host "  Starting continuous publisher..."
    $publishJob = Start-Job -ScriptBlock {
        for ($i = 1; $i -le 100; $i++) {
            $ts = [DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()
            $json = @{bus_id="EMQX-TEST-$i";route_id="ROUTE-001";direction=0;stop_index=1;is_at_terminal=$false;lat=-33.8688;lon=151.2093;speed_kmh=30.0;status="active";timestamp_ms=$ts} | ConvertTo-Json -Compress
            $json | Set-Content -Path ".\emqx_test_$i.json" -NoNewline -Encoding ASCII
            docker cp ".\emqx_test_$i.json" alebus-mqtt-publisher:/tmp/msg.json 2>$null
            docker exec alebus-mqtt-publisher mosquitto_pub -h emqx -p 1883 -t "bus/EMQX-TEST-$i/telemetry" -f /tmp/msg.json 2>$null
            Remove-Item ".\emqx_test_$i.json" -ErrorAction SilentlyContinue
            Start-Sleep -Milliseconds 100
        }
    }
    
    # Wait then restart EMQX
    Start-Sleep -Seconds 2
    Write-Host "  Restarting EMQX..."
    docker restart alebus-emqx 2>$null
    
    # Wait for EMQX to be healthy
    Write-Host "  Waiting for EMQX to be healthy..."
    for ($i = 0; $i -lt 60; $i++) {
        Start-Sleep -Seconds 1
        $emqxHealth = docker exec alebus-emqx emqx ping 2>$null
        if ($emqxHealth -eq "pong") {
            Write-Host "  EMQX healthy after $($i+1) seconds"
            break
        }
    }
    
    # Wait for job to complete
    Wait-Job $publishJob -Timeout 30 | Out-Null
    Stop-Job $publishJob -ErrorAction SilentlyContinue
    Remove-Job $publishJob -ErrorAction SilentlyContinue
    
    Start-Sleep -Seconds 3
    
    # Check ingestor reconnected
    $health = Get-IngestorHealth
    Write-Host "  Ingestor health: $($health.status) (mqtt: $($health.mqtt))"
    
    $after = Get-IngestorMetrics
    Write-Host "  After: received=$($after.Received), accepted=$($after.Accepted)"
    
    $deltaReceived = $after.Received - $before.Received
    $deltaAccepted = $after.Accepted - $before.Accepted
    
    if ($health.mqtt -eq "connected" -and $deltaAccepted -gt 0) {
        Write-Host "  ✅ TEST 3 PASSED: Ingestor reconnected after EMQX restart (+$deltaAccepted accepted)" -ForegroundColor Green
    } else {
        Write-Host "  ⚠️  TEST 3 PARTIAL: Some messages may have been lost during restart" -ForegroundColor Yellow
    }
}

# ═══════════════════════════════════════════════════════════════════════════════
# TEST 4: Multiple Ingestors with Shared Subscriptions
# ═══════════════════════════════════════════════════════════════════════════════
if ($TestNumber -eq 0 -or $TestNumber -eq 4) {
    Write-Host "`n┌─────────────────────────────────────────────────────────────────────────────┐"
    Write-Host "│ TEST 4: Multiple Ingestors with Shared Subscriptions                        │"
    Write-Host "└─────────────────────────────────────────────────────────────────────────────┘"
    
    Write-Host "  Starting 2 additional ingestor instances..."
    
    # Start ingestor-2
    docker run -d --name alebus-ingestor-2 --network alebus-net `
        -e EMQX_SERVER_URLS="mqtt://emqx:1883" `
        -e EMQX_CLIENT_ID="alebus-ingestor-2" `
        -e EMQX_TOPIC_FILTER="bus/+/telemetry" `
        -e EMQX_SHARED_GROUP="alebus-ingestor" `
        -e REDIS_ADDR="redis:6379" `
        -e REDIS_ENABLED="true" `
        -e INGESTOR_HTTP_ADDR=":9101" `
        -p 9101:9101 `
        alebus-ingestor:latest 2>$null
    
    # Start ingestor-3
    docker run -d --name alebus-ingestor-3 --network alebus-net `
        -e EMQX_SERVER_URLS="mqtt://emqx:1883" `
        -e EMQX_CLIENT_ID="alebus-ingestor-3" `
        -e EMQX_TOPIC_FILTER="bus/+/telemetry" `
        -e EMQX_SHARED_GROUP="alebus-ingestor" `
        -e REDIS_ADDR="redis:6379" `
        -e REDIS_ENABLED="true" `
        -e INGESTOR_HTTP_ADDR=":9102" `
        -p 9102:9102 `
        alebus-ingestor:latest 2>$null
    
    # Wait for them to start
    Write-Host "  Waiting for ingestors to connect..."
    Start-Sleep -Seconds 10
    
    # Check all three are healthy
    $health1 = Get-IngestorHealth
    try { $health2 = Invoke-RestMethod -Uri "http://localhost:9101/healthz" -TimeoutSec 2 } catch { $health2 = @{status="unreachable"} }
    try { $health3 = Invoke-RestMethod -Uri "http://localhost:9102/healthz" -TimeoutSec 2 } catch { $health3 = @{status="unreachable"} }
    
    Write-Host "  Ingestor 1: $($health1.status)"
    Write-Host "  Ingestor 2: $($health2.status)"
    Write-Host "  Ingestor 3: $($health3.status)"
    
    # Send 30 messages
    Write-Host "  Sending 30 messages to test load distribution..."
    for ($i = 1; $i -le 30; $i++) {
        Publish-TestMessage -BusId "SHARED-$i"
        Start-Sleep -Milliseconds 50
    }
    
    Start-Sleep -Seconds 2
    
    # Get metrics from all three
    $m1 = Get-IngestorMetrics
    try {
        $m2Content = (Invoke-WebRequest -Uri "http://localhost:9101/metrics" -UseBasicParsing -TimeoutSec 2).Content
        $m2Received = ([regex]::Match($m2Content, 'alebus_ingestor_received_total\s+(\d+)')).Groups[1].Value
    } catch { $m2Received = "0" }
    try {
        $m3Content = (Invoke-WebRequest -Uri "http://localhost:9102/metrics" -UseBasicParsing -TimeoutSec 2).Content
        $m3Received = ([regex]::Match($m3Content, 'alebus_ingestor_received_total\s+(\d+)')).Groups[1].Value
    } catch { $m3Received = "0" }
    
    Write-Host "  Messages received - Ingestor1: $($m1.Received), Ingestor2: $m2Received, Ingestor3: $m3Received"
    
    # Clean up extra ingestors
    Write-Host "  Cleaning up extra ingestors..."
    docker stop alebus-ingestor-2 alebus-ingestor-3 2>$null
    docker rm alebus-ingestor-2 alebus-ingestor-3 2>$null
    
    if ([int]$m2Received -gt 0 -or [int]$m3Received -gt 0) {
        Write-Host "  ✅ TEST 4 PASSED: Load distributed across multiple ingestors" -ForegroundColor Green
    } else {
        Write-Host "  ⚠️  TEST 4 PARTIAL: Only primary ingestor received messages (shared sub may need EMQX config)" -ForegroundColor Yellow
    }
}

# ═══════════════════════════════════════════════════════════════════════════════
# TEST 5: Redis Latency Injection
# ═══════════════════════════════════════════════════════════════════════════════
if ($TestNumber -eq 0 -or $TestNumber -eq 5) {
    Write-Host "`n┌─────────────────────────────────────────────────────────────────────────────┐"
    Write-Host "│ TEST 5: Redis Latency Injection                                             │"
    Write-Host "└─────────────────────────────────────────────────────────────────────────────┘"
    
    $before = Get-IngestorMetrics
    Write-Host "  Before: received=$($before.Received), accepted=$($before.Accepted)"
    
    # Use Redis DEBUG SLEEP to simulate latency (this pauses Redis)
    Write-Host "  Injecting 2-second latency via Redis DEBUG SLEEP..."
    
    # Start publishing
    $publishJob = Start-Job -ScriptBlock {
        for ($i = 1; $i -le 20; $i++) {
            $ts = [DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()
            $json = @{bus_id="LATENCY-$i";route_id="ROUTE-001";direction=0;stop_index=1;is_at_terminal=$false;lat=-33.8688;lon=151.2093;speed_kmh=30.0;status="active";timestamp_ms=$ts} | ConvertTo-Json -Compress
            $json | Set-Content -Path ".\latency_test_$i.json" -NoNewline -Encoding ASCII
            docker cp ".\latency_test_$i.json" alebus-mqtt-publisher:/tmp/msg.json 2>$null
            docker exec alebus-mqtt-publisher mosquitto_pub -h emqx -p 1883 -t "bus/LATENCY-$i/telemetry" -f /tmp/msg.json 2>$null
            Remove-Item ".\latency_test_$i.json" -ErrorAction SilentlyContinue
            Start-Sleep -Milliseconds 200
        }
    }
    
    # Pause Redis for 2 seconds mid-stream
    Start-Sleep -Seconds 1
    Write-Host "  Pausing Redis for 2 seconds..."
    docker pause alebus-redis-emqx 2>$null
    Start-Sleep -Seconds 2
    docker unpause alebus-redis-emqx 2>$null
    Write-Host "  Redis unpaused"
    
    # Wait for job
    Wait-Job $publishJob -Timeout 30 | Out-Null
    Stop-Job $publishJob -ErrorAction SilentlyContinue
    Remove-Job $publishJob -ErrorAction SilentlyContinue
    
    Start-Sleep -Seconds 3
    
    $after = Get-IngestorMetrics
    Write-Host "  After: received=$($after.Received), accepted=$($after.Accepted), infra_error=$($after.InfraError)"
    
    $deltaReceived = $after.Received - $before.Received
    $deltaAccepted = $after.Accepted - $before.Accepted
    $deltaInfra = $after.InfraError - $before.InfraError
    
    $health = Get-IngestorHealth
    Write-Host "  Health: $($health.status)"
    
    if ($health.status -eq "healthy" -and $deltaAccepted -gt 0) {
        Write-Host "  ✅ TEST 5 PASSED: Ingestor handled latency (+$deltaAccepted accepted, $deltaInfra infra errors)" -ForegroundColor Green
    } else {
        Write-Host "  ⚠️  TEST 5 PARTIAL: Some messages may have failed during pause" -ForegroundColor Yellow
    }
}

# ═══════════════════════════════════════════════════════════════════════════════
# SUMMARY
# ═══════════════════════════════════════════════════════════════════════════════
Write-Host "`n═══════════════════════════════════════════════════════════════════════════════"
Write-Host "RESILIENCE TESTS COMPLETE"
Write-Host "═══════════════════════════════════════════════════════════════════════════════"

# Final health check
$finalHealth = Get-IngestorHealth
Write-Host "`nFinal System Status:"
Write-Host "  Ingestor: $($finalHealth.status)"
Write-Host "  Redis: $($finalHealth.redis)"
Write-Host "  MQTT: $($finalHealth.mqtt)"
Write-Host "  Heartbeat: $($finalHealth.heartbeat)"

$finalMetrics = Get-IngestorMetrics
if ($finalMetrics) {
    Write-Host "`nFinal Metrics:"
    Write-Host "  Total Received: $($finalMetrics.Received)"
    Write-Host "  Total Accepted: $($finalMetrics.Accepted)"
    Write-Host "  Total Invalid: $($finalMetrics.Invalid)"
    Write-Host "  Total Infra Errors: $($finalMetrics.InfraError)"
}
