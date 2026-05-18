-- Create stops table for StopGeoRepository (read model)
-- Uses PostGIS GEOGRAPHY type for geospatial queries

CREATE TABLE stops (
    stop_id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    location GEOGRAPHY(POINT, 4326) NOT NULL
);

-- GIST index for geospatial queries (ST_DWithin, etc.)
CREATE INDEX idx_stops_location ON stops USING GIST (location);

-- Add helper function to insert stops with lat/lon
-- Usage: SELECT insert_stop('stop1', 'Central Station', -33.8688, 151.2093);
CREATE OR REPLACE FUNCTION insert_stop(
    p_stop_id TEXT,
    p_name TEXT,
    p_latitude DOUBLE PRECISION,
    p_longitude DOUBLE PRECISION
) RETURNS VOID AS $$
BEGIN
    INSERT INTO stops (stop_id, name, location)
    VALUES (p_stop_id, p_name, ST_SetSRID(ST_MakePoint(p_longitude, p_latitude), 4326)::geography)
    ON CONFLICT (stop_id) DO UPDATE SET
        name = EXCLUDED.name,
        location = EXCLUDED.location;
END;
$$ LANGUAGE plpgsql;

-- Add comments for documentation
COMMENT ON TABLE stops IS 'Stops read model - deduplicated stop reference with geospatial index';
COMMENT ON COLUMN stops.location IS 'PostGIS GEOGRAPHY point (SRID 4326 = WGS84)';
COMMENT ON FUNCTION insert_stop IS 'Helper to insert/update stops using lat/lon instead of WKT';
