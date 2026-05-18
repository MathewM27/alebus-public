-- Create bus_devices table for provisioning physical GPS trackers (phones/devices)
-- This table is security/infrastructure-oriented: it maps a bus to a provisioned device identity.

CREATE TABLE bus_devices (
    device_id UUID PRIMARY KEY,
    bus_id TEXT NOT NULL REFERENCES buses(bus_id) ON DELETE CASCADE,

    -- MQTT credentials for the device
    mqtt_username TEXT NOT NULL UNIQUE,
    mqtt_password_hash TEXT NOT NULL,

    -- Lifecycle
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMPTZ,
    last_seen_at TIMESTAMPTZ
);

-- Enforce: only one active (not revoked) device per bus.
CREATE UNIQUE INDEX ux_bus_devices_active_per_bus
    ON bus_devices (bus_id)
    WHERE revoked_at IS NULL;

CREATE INDEX idx_bus_devices_bus_id ON bus_devices (bus_id);
CREATE INDEX idx_bus_devices_last_seen ON bus_devices (last_seen_at);

COMMENT ON TABLE bus_devices IS 'Provisioned GPS tracker devices (phones/devices) authorized to publish telemetry for a bus.';
COMMENT ON COLUMN bus_devices.mqtt_password_hash IS 'BCrypt hash of the MQTT password; plaintext is only returned once at provisioning time.';
