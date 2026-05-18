-- Migration: Enable PostGIS-based stop queries
-- This adds route relationship and cumulative distance to the stops table
-- and populates it from existing routes for efficient geospatial queries.

-- Add route relationship column (which routes pass through this stop)
ALTER TABLE stops ADD COLUMN IF NOT EXISTS route_ids TEXT[] NOT NULL DEFAULT '{}';

-- Add cumulative distance for segment-based distance calculations
ALTER TABLE stops ADD COLUMN IF NOT EXISTS cumulative_distance_meters DOUBLE PRECISION;

-- Index for finding stops by route (GIN for array containment queries)
CREATE INDEX IF NOT EXISTS idx_stops_route_ids ON stops USING GIN (route_ids);

-- Populate stops table from existing routes (one-time sync)
-- This extracts stops from the JSONB column and creates proper PostGIS points
INSERT INTO stops (stop_id, name, location, route_ids, cumulative_distance_meters)
SELECT 
    stop->>'id' as stop_id,
    stop->>'name' as name,
    ST_SetSRID(ST_MakePoint(
        (stop->'location'->>'longitude')::double precision,
        (stop->'location'->>'latitude')::double precision
    ), 4326)::geography as location,
    ARRAY[route_id] as route_ids,
    (stop->>'cumulativeDistanceMeters')::double precision as cumulative_distance_meters
FROM routes, jsonb_array_elements(stops) as stop
WHERE status = 0
ON CONFLICT (stop_id) DO UPDATE SET
    name = EXCLUDED.name,
    location = EXCLUDED.location,
    route_ids = (
        SELECT ARRAY(SELECT DISTINCT unnest(array_cat(stops.route_ids, EXCLUDED.route_ids)) ORDER BY 1)
    ),
    cumulative_distance_meters = COALESCE(EXCLUDED.cumulative_distance_meters, stops.cumulative_distance_meters);

-- Add comments for documentation
COMMENT ON COLUMN stops.route_ids IS 'Array of route IDs that pass through this stop';
COMMENT ON COLUMN stops.cumulative_distance_meters IS 'Distance from route origin (per-route, may vary)';
