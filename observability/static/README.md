# Observability Pipeline Debugger UI

## Overview

A real-time visualization dashboard for debugging the Alebus GPS processing pipeline. This UI shows the complete data flow from raw MQTT GPS events through enrichment, Redis storage, and recommendation ranking.

## Features

- **4-Stage Pipeline View:**
  1. 📡 Raw GPS (MQTT) - Shows raw coordinates and metadata from MQTT
  2. ⚙️ Enrichment - Shows route/stop/direction calculation results
  3. 💾 Redis State - Shows final enriched state stored in Redis
  4. 🎯 Recommendations - Shows journey bus ranking with scores

- **Interactive Controls:**
  - Click "Step All Buses" to trigger GPS simulation
  - Auto-refresh toggle (5-second intervals)
  - Clear logs button
  - Configurable interpolation points

- **Real-time Status:**
  - Service health indicators (Simulator, Ingestor, Redis)
  - Last update timestamp
  - Debug panel with raw API responses

## Prerequisites

1. **Services must be running:**
   ```bash
   docker compose -f compose/dev.yml up -d
   ```

2. **Sample data loaded:**
   ```bash
   # Load routes
   curl -X POST -H "Authorization: Bearer devtoken" \
     "http://127.0.0.1:9090/api/v1/routes/load-sample"
   
   # Load buses
   curl -X POST -H "Authorization: Bearer devtoken" \
     "http://127.0.0.1:9090/api/v1/buses/load-sample-raw"
   
   # Load journeys
   curl -X POST -H "Authorization: Bearer devtoken" \
     "http://127.0.0.1:9090/api/v1/journeys/load-sample"
   ```

## Usage

**Access the UI:**

```bash
# The UI is served directly from the simulator
http://127.0.0.1:9090/observability/

# No need for Python server or opening files directly!
```

**Authentication:**
The UI uses the same credentials you have in your browser:
- **Bearer Token:** `devtoken` (already configured in app.js)
- **Dev Secret:** `dev`

**Quick Test:**

### Scenario: Journey recommendation mismatch

1. **Click "Step All Buses"** - Triggers GPS simulation for all buses
2. **Wait 2-3 seconds** - Allow data to propagate through pipeline
3. **Click "Refresh Data"** - Load current state from all endpoints

4. **Compare stages:**
   - Check **Raw GPS** column: Are MQTT events being received?
   - Check **Enrichment** column: Is `enriched: true`? Are terminals detected?
   - Check **Redis** column: Are route_id, stop_index, direction correct?
   - Check **Recommendations** column: Do rankings match expected behavior?

5. **Identify root cause:**
   - **Raw GPS missing** → MQTT connection issue
   - **Enrichment failed** → Check enrichment logic in `cmd/emqx_ingestor`
   - **Redis mismatch** → Redis TTL expired or write failed
   - **Wrong ranking** → Check scoring logic in `application/journey/resolver`

### Example: Bus GPS-FPL-01-B not appearing in recommendations

1. Click "Step All Buses"
2. Open **Raw GPS** column → Find GPS-FPL-01-B
   - ✅ If found: MQTT is working
   - ❌ If missing: Check MQTT connection or bus device provisioning

3. Check **Enrichment** column → Find GPS-FPL-01-B
   - ✅ If `enriched: true`: Enrichment succeeded
   - ❌ If `enriched: false`: Check route matching logic

4. Check **Redis** column → Find GPS-FPL-01-B
   - ✅ If exists with TTL > 0: Redis state is fresh
   - ❌ If missing or TTL = 0: Redis write failed or expired

5. Check **Recommendations** column → Look for GPS-FPL-01-B in journey recommendations
   - ✅ If present: Check ranking score vs other buses
   - ❌ If missing: Check if bus matches journey criteria (route, direction, stops)

## Configuration

Edit `app.js` to customize endpoints:

```javascript
const CONFIG = {
    simulatorBaseUrl: 'http://127.0.0.1:9090',  // Simulator API
    ingestorBaseUrl: 'http://127.0.0.1:9100',   // Ingestor API
    bearerToken: 'devtoken',                     // Auth token
    devSecret: 'dev',                            // Dev secret
    autoRefreshInterval: 5000,                   // Auto-refresh interval (ms)
};
```

## API Endpoints Used

1. **POST** `/api/v1/buses/simulate-all-gps?interpolate=N`
   - Triggers GPS simulation with N interpolated points
   - Auth: Bearer token

2. **GET** `/dev/obs/bus?busId=<id>`
   - Returns last MQTT event for bus
   - Port: 9100, No auth required

3. **GET** `/api/v1/dev/obs/redis/bus?busId=<id>`
   - Returns Redis state for bus
   - Auth: Bearer token + X-Dev-Secret header

4. **GET** `/api/v1/dev/obs/journey-explain?journeyId=<id>`
   - Returns journey recommendations with scores
   - Auth: Bearer token + X-Dev-Secret header

5. **GET** `/api/v1/buses`
   - Lists all buses (to get bus IDs)
   - Auth: Bearer token

6. **GET** `/api/v1/journeys`
   - Lists all journeys (to get journey IDs)
   - Auth: Bearer token

## Troubleshooting

### "No buses found" error

**Solution:** Load sample data first:
```bash
curl -X POST -H "Authorization: Bearer devtoken" \
  "http://127.0.0.1:9090/api/v1/buses/load-sample-raw"
```

### Services showing "Down" status

**Solution:** Check Docker containers:
```bash
docker compose -f compose/dev.yml ps
docker compose -f compose/dev.yml logs simulator
docker compose -f compose/dev.yml logs emqx_ingestor
```

### CORS errors in browser console

**Solution:** Either:
- Open HTML file directly (not via file://)
- Use a local http server (see "Usage" section above)
- Disable CORS in browser (dev only, not recommended)

### Empty logs after clicking "Step All Buses"

**Possible causes:**
1. Wait 2-3 seconds for data propagation
2. Click "Refresh Data" manually
3. Check browser console for API errors
4. Verify services are healthy in status bar

### Auto-refresh not working

**Solution:** Enable it via checkbox:
- Check the "Auto-refresh (5s)" checkbox
- Data will refresh every 5 seconds automatically

## Features in Detail

### Raw GPS (MQTT) Stage

Shows the last MQTT message received by the ingestor:
- **Bus ID:** Identifier from MQTT topic
- **Lat/Lon:** Raw GPS coordinates
- **Speed:** Speed in km/h
- **Device Time:** Timestamp from GPS device
- **MQTT Topic:** The actual MQTT topic path

### Enrichment Stage

Shows enrichment computation results:
- **Enriched flag:** Whether route/stop/direction were calculated
- **Interpolated:** Whether this is an interpolated point
- **At Terminal:** Whether bus is at route terminal
- **Distance from Stop:** Distance to nearest stop in meters

### Redis State Stage

Shows the final enriched state stored in Redis:
- **Route ID:** Matched route identifier
- **Direction:** North (0) or South (1)
- **Stop Index:** Current stop position on route
- **Stop ID:** Current stop identifier
- **TTL:** Time-to-live remaining (default 120s)

### Recommendations Stage

Shows journey recommendations with ranking:
- **Journey ID:** Unique journey identifier
- **Status:** Tracking, Completed, Cancelled, etc.
- **Active Bus:** Currently selected bus for journey
- **Top 3 recommendations:** Ranked buses with:
  - **ETA:** Estimated arrival time
  - **Distance:** Distance to origin stop
  - **Confidence:** Confidence level (0-1)
  - **Display Text:** Human-readable status

## Dark Mode Design

The UI uses a dark color scheme optimized for:
- Long debugging sessions (reduced eye strain)
- Clear visual hierarchy
- Color-coded stages (MQTT=purple, Enrichment=cyan, Redis=orange, Recommendations=green)
- High contrast for readability

## Browser Compatibility

- ✅ Chrome/Edge (Chromium)
- ✅ Firefox
- ✅ Safari
- ⚠️ IE11 not supported (uses modern ES6+ features)

## Performance

- Fetches data for up to 100 buses simultaneously
- Limits journey recommendations to first 10 journeys (configurable)
- Auto-refresh can be toggled on/off
- Logs are cleared on demand to prevent memory bloat

## Related Tools

### Simulation Preview
The observability UI works in conjunction with **simulation_preview** (port 8080):
- **simulation_preview:** Interactive UI for stepping buses and watching journeys
- **observability UI:** Shows the complete pipeline breakdown (MQTT → Enrichment → Redis → Recommendations)

**Workflow:**
1. Use [simulation_preview](http://127.0.0.1:8080) to "Step" specific buses
2. Switch to [observability UI](http://127.0.0.1:9090/observability/)
3. Click "Refresh Data" to see the pipeline breakdown
4. Both UIs fetch from the same backend, so data is consistent

**Example:**
```bash
# In simulation_preview: Click "Step" on bus GPS-FPL-01-B
# Then in observability UI: See the raw GPS → enrichment → Redis → recommendations flow
```

## Related Documentation

- [OBSERVABILITY_DEBUG_WORKFLOW.md](../docs/OBSERVABILITY_DEBUG_WORKFLOW.md) - Complete debugging guide
- [REALTIME_TRANSPORT.md](../docs/REALTIME_TRANSPORT.md) - Real-time architecture overview
- [RUNTIME_DATA_FLOW.md](../docs/RUNTIME_DATA_FLOW.md) - Data flow documentation

## Future Enhancements

- [ ] WebSocket support for real-time updates
- [ ] Export logs to JSON/CSV
- [ ] Filter by specific bus/journey
- [ ] Compare before/after GPS simulation states
- [ ] Performance metrics (latency between stages)
- [ ] Alert system for pipeline failures
