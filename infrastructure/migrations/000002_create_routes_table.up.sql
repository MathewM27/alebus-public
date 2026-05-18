-- Create routes table for Route aggregate
-- Stores route configuration, stops (JSONB), and operator IDs (JSONB)

CREATE TABLE routes (
    route_id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    operator_ids JSONB NOT NULL DEFAULT '[]',
    stops JSONB NOT NULL DEFAULT '[]',
    direction INT NOT NULL,
    route_type INT NOT NULL,
    avg_detour_rate DOUBLE PRECISION NOT NULL DEFAULT 1.3,
    status INT NOT NULL DEFAULT 0,
    active_from TIMESTAMPTZ NOT NULL,
    active_until TIMESTAMPTZ NOT NULL,
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for finding active routes by time range
CREATE INDEX idx_routes_active_period ON routes (active_from, active_until);

-- Index for filtering by status
CREATE INDEX idx_routes_status ON routes (status);

-- Add comments for documentation
COMMENT ON TABLE routes IS 'Route aggregate - bus routes with stops and operator assignments';
COMMENT ON COLUMN routes.operator_ids IS 'JSONB array of operator IDs: ["op1", "op2"]';
COMMENT ON COLUMN routes.stops IS 'JSONB array of stops: [{"id": "stop1", "name": "Stop 1", "location": {"latitude": 0.0, "longitude": 0.0}}]';
COMMENT ON COLUMN routes.direction IS 'RouteDirection enum: 0=Outbound, 1=Inbound, 2=Bidirectional, 3=Unidirectional';
COMMENT ON COLUMN routes.route_type IS 'RouteType enum: 0=Urban, 1=Highway, 2=Mixed';
COMMENT ON COLUMN routes.status IS 'RouteStatus enum: 0=Active, 1=Inactive, 2=Suspended';
