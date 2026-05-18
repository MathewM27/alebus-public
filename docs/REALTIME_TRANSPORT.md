# Realtime Transport

**Alebus uses WebSockets as the canonical realtime transport mechanism.** This document explains the connection lifecycle, message formats, available streams, and provides guidance for implementing WebSocket clients.

---

## Why WebSockets?

**Requirements:**
- **Mobile support:** React Native Expo does not support Server-Sent Events (EventSource)
- **Bidirectional communication:** Clients can send subscription commands dynamically
- **Efficiency:** One TCP connection can multiplex multiple subscriptions
- **Universal compatibility:** WebSockets work on web, iOS, Android, and native apps

**SSE is NOT used for user-facing realtime updates.** Legacy SSE endpoints exist but are deprecated.

---

## WebSocket Endpoints

### 1. `/api/v1/ws/mux` — Multiplexed WebSocket (Recommended)

**Use this endpoint for production applications.**

**Features:**
- Single WebSocket connection
- Dynamic subscription management (subscribe/unsubscribe at runtime)
- Multiple concurrent subscriptions (max 32)
- Supports both `journeys` and `live-buses` streams

**Connection URL:**
```
ws://127.0.0.1:9090/api/v1/ws/mux
```

**Client → Server Messages:**

```json
{
  "type": "subscribe",
  "data": {
    "stream": "journeys",
    "journeyId": "journey-123",
    "busIds": ["BUS-001", "BUS-002"]
  }
}
```

```json
{
  "type": "subscribe",
  "data": {
    "stream": "live-buses",
    "busIds": ["BUS-001", "BUS-002", "BUS-003"]
  }
}
```

```json
{
  "type": "unsubscribe",
  "data": {
    "subId": "journeys:1"
  }
}
```

**Server → Client Messages:**

```json
{
  "type": "ready",
  "data": {
    "serverTs": "2026-02-04T12:00:00Z",
    "stream": "mux"
  }
}
```

```json
{
  "type": "subscribed",
  "data": {
    "serverTs": "2026-02-04T12:00:01Z",
    "subId": "journeys:1",
    "stream": "journeys"
  }
}
```

```json
{
  "type": "bus.update",
  "data": {
    "subId": "journeys:1",
    "serverTs": "2026-02-04T12:00:05Z",
    "seq": 42,
    "bus": {
      "BusID": "BUS-001",
      "RouteID": "ROUTE-A",
      "Position": {
        "Lat": 40.7128,
        "Lon": -74.0060,
        "SpeedKmh": 45.5,
        "Accuracy": 10.0,
        "Timestamp": "2026-02-04T12:00:04Z"
      },
      "Direction": 0,
      "StopIndex": 12,
      "IsAtTerminal": false
    }
  }
}
```

```json
{
  "type": "journey.update",
  "data": {
    "subId": "journeys:1",
    "serverTs": "2026-02-04T12:00:06Z",
    "seq": 43,
    "journey": {
      "journeyId": "journey-123",
      "status": "tracking",
      "activeBusId": "BUS-001",
      "estimatedArrival": "2026-02-04T12:15:00Z"
    }
  }
}
```

```json
{
  "type": "error",
  "data": {
    "serverTs": "2026-02-04T12:00:02Z",
    "code": "not_found",
    "message": "journey not found",
    "subId": "journeys:1"
  }
}
```

---

### 2. `/api/v1/ws/journeys` — Journey-Specific WebSocket

**Simpler endpoint for tracking a single journey.**

**Connection URL:**
```
ws://127.0.0.1:9090/api/v1/ws/journeys?journeyId=journey-123&busIds=BUS-001,BUS-002
```

**Query Parameters:**
- `journeyId` (required): Journey ID to track
- `busIds` (optional): Comma-separated bus IDs to filter updates

**Server → Client Messages:**
- `ready`: Connection established
- `bus.update`: Bus position/state update
- `journey.update`: Journey state update (active bus, ETA, etc.)

**Client → Server:** None (read-only stream)

---

### 3. `/api/v1/ws/live-buses` — Live Bus Position Stream

**Stream live bus positions filtered by bus IDs.**

**Connection URL:**
```
ws://127.0.0.1:9090/api/v1/ws/live-buses?busIds=BUS-001,BUS-002,BUS-003
```

**Query Parameters:**
- `busIds` (required): Comma-separated bus IDs (max 100)

**Server → Client Messages:**
- `ready`: Connection established
- `bus.update`: Bus position update

**Client → Server:** None (read-only stream)

---

## Connection Lifecycle

### 1. Handshake (HTTP → WebSocket Upgrade)

**Client sends:**
```
GET /api/v1/ws/mux HTTP/1.1
Host: 127.0.0.1:9090
Upgrade: websocket
Connection: Upgrade
Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==
Sec-WebSocket-Version: 13
Origin: http://localhost:8081
```

**Server responds:**
```
HTTP/1.1 101 Switching Protocols
Upgrade: websocket
Connection: Upgrade
Sec-WebSocket-Accept: s3pPLMBiTxaQ9kYGzzhZRbK+xOo=
```

**Possible errors (before upgrade):**
- `403 Forbidden` — Origin not allowed (CORS issue)
- `503 Service Unavailable` — Too many connections (saturated)
- `400 Bad Request` — Missing required query params (journeys/live-buses endpoints)

---

### 2. Connection Established

**Server sends `ready` message:**
```json
{
  "type": "ready",
  "data": {
    "serverTs": "2026-02-04T12:00:00Z",
    "stream": "mux"
  }
}
```

**Client is now ready to send subscriptions (mux endpoint) or receive updates (other endpoints).**

---

### 3. Keep-Alive (Ping/Pong)

**Server sends WebSocket PING frame every 30 seconds.**

**Client must respond with PONG frame** (most WebSocket libraries do this automatically).

**If no PONG received within 90 seconds:** Server disconnects.

---

### 4. Realtime Updates

**Server pushes updates as events occur:**
- **GPS update received** → `bus.update` message
- **Journey state changed** → `journey.update` message
- **Bus assigned to journey** → `journey.update` message with new `activeBusId`

**Message rate:** Variable (depends on GPS update frequency, typically 1-10 messages/second per subscription)

---

### 5. Graceful Close

**Client initiates close:**
```javascript
ws.close(1000, "User closed connection");
```

**Server acknowledges and cleans up subscriptions.**

**Server initiates close (rare):**
- Shutdown
- Connection idle timeout
- Backpressure (client too slow, outgoing queue full)

---

## Message Payload Semantics

### Bus Update Payload

```json
{
  "BusID": "BUS-001",
  "OperatorID": "OP-ABC",
  "RouteID": "ROUTE-A",
  "Direction": 0,               // 0 = outbound, 1 = inbound
  "StopIndex": 12,             // Current stop index on route
  "IsAtTerminal": false,
  "TerminalArrivalTime": null,
  "Position": {
    "Lat": 40.7128,
    "Lon": -74.0060,
    "SpeedKmh": 45.5,
    "Accuracy": 10.0,
    "Timestamp": "2026-02-04T12:00:04Z"
  },
  "Status": 0,                  // 0 = active, 1 = offline, 2 = maintenance
  "UpdatedAt": "2026-02-04T12:00:05Z"
}
```

**Field Notes:**
- `Direction`: Integer enum (0 = outbound, 1 = inbound)
- `StopIndex`: Zero-based index of current/next stop on route
- `IsAtTerminal`: Boolean indicating if bus is dwelling at terminal
- `Position.Timestamp`: When GPS was captured (may lag behind `UpdatedAt`)
- `UpdatedAt`: When server last updated this bus state

---

### Journey Update Payload

```json
{
  "journeyId": "journey-123",
  "userId": "user-456",
  "status": "tracking",         // tracking | boarded | completed | cancelled
  "activeBusId": "BUS-001",     // Current recommended bus
  "originStopId": "STOP-10",
  "destinationStopId": "STOP-25",
  "estimatedArrival": "2026-02-04T12:15:00Z",
  "boardingConfirmed": false,
  "createdAt": "2026-02-04T11:50:00Z",
  "updatedAt": "2026-02-04T12:00:06Z"
}
```

**Field Notes:**
- `activeBusId`: Changes when journey-worker computes a better recommendation
- `estimatedArrival`: ETA to destination stop (null if no active bus)
- `boardingConfirmed`: `true` after user confirms boarding
- `status`: Lifecycle state (tracking → boarded → completed)

---

## Reconnection Strategy

**WebSocket connections may disconnect due to:**
- Network issues
- Server restarts
- Load balancer timeouts
- Client device sleep/wake cycles

**Recommended reconnection logic:**

```javascript
let reconnectAttempts = 0;
const maxReconnectDelay = 30000; // 30 seconds

function connect() {
  const ws = new WebSocket('ws://127.0.0.1:9090/api/v1/ws/mux');
  
  ws.onopen = () => {
    console.log('WebSocket connected');
    reconnectAttempts = 0;
    // Re-subscribe to all active subscriptions
    resubscribeAll(ws);
  };
  
  ws.onclose = (event) => {
    console.log('WebSocket closed:', event.code, event.reason);
    reconnect();
  };
  
  ws.onerror = (error) => {
    console.error('WebSocket error:', error);
    // onclose will fire next, triggering reconnect
  };
  
  ws.onmessage = (event) => {
    const msg = JSON.parse(event.data);
    handleMessage(msg);
  };
  
  return ws;
}

function reconnect() {
  reconnectAttempts++;
  const delay = Math.min(1000 * Math.pow(2, reconnectAttempts), maxReconnectDelay);
  console.log(`Reconnecting in ${delay}ms (attempt ${reconnectAttempts})...`);
  setTimeout(connect, delay);
}
```

**Key principles:**
- **Exponential backoff:** 1s, 2s, 4s, 8s, 16s, 30s (max)
- **Resubscribe after reconnect:** Track active subscriptions and replay them
- **Idempotent subscriptions:** Safe to subscribe multiple times to same stream

---

## Error Handling

### Connection Errors (Before Upgrade)

**HTTP 403 Forbidden:**
- **Cause:** Origin header not in allowed list
- **Fix:** Add your origin to `WS_ALLOWED_ORIGINS` environment variable
- **Example:** `WS_ALLOWED_ORIGINS=http://localhost:8081,https://app.example.com`

**HTTP 503 Service Unavailable:**
- **Cause:** Too many concurrent WebSocket connections
- **Fix:** Implement connection pooling on client side, or increase `STREAM_MAX_CONNECTIONS` on server

**HTTP 400 Bad Request:**
- **Cause:** Missing required query parameter (`journeyId` or `busIds`)
- **Fix:** Ensure query params are included in connection URL

---

### Runtime Errors (After Upgrade)

**`error` message from server:**
```json
{
  "type": "error",
  "data": {
    "code": "not_found",
    "message": "journey not found",
    "subId": "journeys:1"
  }
}
```

**Common error codes:**
- `not_found`: Journey/bus does not exist
- `forbidden`: User not authorized to subscribe to this stream
- `invalid_request`: Malformed subscription request
- `too_many_subscriptions`: Exceeded max subscriptions per connection (32)
- `misconfigured`: Server-side configuration error (rare)

**Client action:**
- Log error
- Optionally unsubscribe from failed subscription
- Do NOT close connection (other subscriptions may be healthy)

---

## Mobile Client Guidance (React Native Expo)

### Install Dependencies

```bash
npm install --save react-native-websocket
```

### Example Client

```javascript
import React, { useEffect, useState } from 'react';

function useAlebusWebSocket(journeyId) {
  const [ws, setWs] = useState(null);
  const [busUpdates, setBusUpdates] = useState([]);
  
  useEffect(() => {
    const socket = new WebSocket('ws://192.168.1.100:9090/api/v1/ws/mux');
    
    socket.onopen = () => {
      console.log('Connected to Alebus');
      // Subscribe to journey stream
      socket.send(JSON.stringify({
        type: 'subscribe',
        data: {
          stream: 'journeys',
          journeyId: journeyId,
        },
      }));
    };
    
    socket.onmessage = (event) => {
      const msg = JSON.parse(event.data);
      
      if (msg.type === 'bus.update') {
        setBusUpdates((prev) => [...prev, msg.data.bus]);
      }
      
      if (msg.type === 'journey.update') {
        console.log('Journey updated:', msg.data.journey);
      }
    };
    
    socket.onerror = (error) => {
      console.error('WebSocket error:', error);
    };
    
    socket.onclose = () => {
      console.log('Disconnected from Alebus');
      // Implement reconnection logic here
    };
    
    setWs(socket);
    
    return () => {
      socket.close();
    };
  }, [journeyId]);
  
  return { ws, busUpdates };
}

// Usage in component:
function JourneyTracker({ journeyId }) {
  const { busUpdates } = useAlebusWebSocket(journeyId);
  
  return (
    <View>
      {busUpdates.map((bus) => (
        <Text key={bus.BusID}>
          {bus.BusID}: {bus.Position.Lat}, {bus.Position.Lon}
        </Text>
      ))}
    </View>
  );
}
```

**Production Recommendations:**
- Use `wss://` (secure WebSocket) in production
- Implement exponential backoff reconnection
- Store subscription state to re-subscribe after reconnect
- Handle auth tokens via query params or custom headers
- Use Redux/Context for global WebSocket management

---

## Performance Characteristics

### Connection Overhead

- **Initial handshake:** <100ms (HTTP upgrade)
- **Memory per connection:** ~4KB (read buffer) + 4KB (write buffer) + subscription state
- **Max connections per server:** 10,000 (configurable via `STREAM_MAX_CONNECTIONS`)

### Message Throughput

- **Bus updates:** 1-10 messages/second per bus (depends on GPS update frequency)
- **Journey updates:** 0.5 messages/second per journey (journey-worker batch interval = 2s)
- **Ping frames:** 1 every 30 seconds

### Backpressure Protection

**Outgoing message queue:** 256 messages per connection

**If queue fills (client processing too slow):**
- Server disconnects client immediately
- Client receives WebSocket close code 1006 (abnormal closure)
- Client should reconnect with exponential backoff

**Mitigation:**
- Process messages quickly (don't block onmessage handler)
- Offload heavy processing to background threads/workers
- Limit subscriptions to only what's actively displayed

---

## Debugging

### Enable Debug Logging (Server)

Set environment variable:
```bash
STREAM_DEBUG=true
```

**Logs include:**
- Connection opened/closed
- Subscription added/removed
- Message send success/failure
- Backpressure queue status

### Inspect WebSocket Traffic (Browser)

**Chrome DevTools:**
1. Open DevTools (F12)
2. Go to **Network** tab
3. Filter by **WS** (WebSocket)
4. Click on connection to see frames

**Message types:**
- ⬆️ **Sent:** Client → Server messages (green)
- ⬇️ **Received:** Server → Client messages (white)
- 🏓 **Ping/Pong:** Keep-alive frames (gray)

---

## Comparison: WebSocket vs SSE

| Feature | WebSocket | SSE | Winner |
|---------|-----------|-----|--------|
| Mobile Support | ✅ Yes (React Native) | ❌ No (EventSource unsupported) | **WebSocket** |
| Bidirectional | ✅ Yes | ❌ No (server → client only) | **WebSocket** |
| Multiplexing | ✅ Yes (`/ws/mux`) | ❌ No (one stream per connection) | **WebSocket** |
| Automatic Reconnect | ❌ No (manual) | ✅ Yes (browser retries) | SSE |
| Browser Support | ✅ Universal | ✅ Universal (desktop) | Tie |
| Firewall Friendly | ⚠️ Some corporate proxies block | ✅ HTTP-based (rarely blocked) | SSE |
| Protocol Overhead | Low (binary framing) | Medium (text-based) | **WebSocket** |

**Verdict:** WebSockets are superior for Alebus due to mobile requirement.

---

## Next Steps

1. **For Web Developers:** Migrate from SSE to WebSocket (`/ws/mux` recommended)
2. **For Mobile Developers:** Use `/ws/mux` endpoint for all realtime subscriptions
3. **For DevOps:** Monitor WebSocket connection count and message throughput
4. **For Backend Engineers:** SSE endpoints remain for backward compatibility but should not be used for new features

---

## References

- **WebSocket Specification:** RFC 6455
- **gorilla/websocket Docs:** https://pkg.go.dev/github.com/gorilla/websocket
- **Alebus API Handlers:** `api/http/ws.go`, `api/http/ws_mux.go`, `api/http/ws_live_buses.go`
- **Test Suite:** `api/http/ws_test.go`, `api/http/ws_mux_test.go`
