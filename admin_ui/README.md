# admin_ui

A tiny static admin page for provisioning/revoking MQTT device credentials per bus.

It calls these Simulation Preview endpoints:
- `POST /api/v1/buses/device/provision`
- `POST /api/v1/buses/device/revoke`

## Run

Any static server works. Two easy options:

### Option A: VS Code Live Server
- Open `admin_ui/index.html`
- Click **Go Live**

### Option B: Python
From repo root:

- `python -m http.server 5175 --directory admin_ui`
- Open `http://localhost:5175`

## Use

1. Ensure Simulation Preview server is running (default in UI: `http://localhost:8081`).
2. Enter a `Bus ID` (e.g. `ANDROID-001`).
3. Click **Provision**.
4. Copy the returned `mqttUsername` / `mqttPassword` into your Android app/device.
5. Publish GPS to the returned topic, usually `bus/{busId}/gps`.

### Send test GPS (no phone MQTT app required)

admin_ui also includes a **Send Test GPS** panel that publishes a raw GPS point via Simulation Preview:

- `POST /api/v1/buses/simulate-gps`

This requires Simulation Preview to be started with MQTT publishing enabled:

- `MQTT_BROKER_URL`
- `SIM_PREVIEW_MQTT_CLIENT_ID`

## Notes

- The password is one-time: it’s only returned at provisioning time.
- This UI does **not** store the password in `localStorage`.
- If you use **Download config JSON**, don’t commit it to git.
