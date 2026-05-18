-- Create buses table for Bus aggregate
-- Stores bus state including position snapshot

CREATE TABLE buses (
    bus_id TEXT PRIMARY KEY,
    operator_id TEXT NOT NULL,
    route_id TEXT NOT NULL,
    
    -- Position snapshot (embedded value object)
    position_lat DOUBLE PRECISION NOT NULL,
    position_lon DOUBLE PRECISION NOT NULL,
    position_timestamp TIMESTAMPTZ NOT NULL,
    position_accuracy DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    position_speed_kmh DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    
    -- Bus state
    stop_index INT NOT NULL DEFAULT 0,
    direction INT NOT NULL,
    current_speed DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    status INT NOT NULL DEFAULT 0,
    is_at_terminal BOOLEAN NOT NULL DEFAULT FALSE,
    terminal_arrival_time TIMESTAMPTZ,
    
    -- Aggregate metadata
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for listing buses by route and status (most common query)
CREATE INDEX idx_buses_route_status ON buses (route_id, status);

-- Index for operator queries
CREATE INDEX idx_buses_operator ON buses (operator_id);

-- Add comments for documentation
COMMENT ON TABLE buses IS 'Bus aggregate - real-time bus tracking state';
COMMENT ON COLUMN buses.direction IS 'Direction enum: 0=Outbound, 1=Inbound';
COMMENT ON COLUMN buses.status IS 'BusStatus enum: 0=Active, 1=Offline, 2=Maintenance';
COMMENT ON COLUMN buses.terminal_arrival_time IS 'Nullable - set when bus arrives at terminal';
