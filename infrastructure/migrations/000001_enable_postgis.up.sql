-- Enable PostGIS extension
-- This must be the first migration to enable geospatial capabilities

CREATE EXTENSION IF NOT EXISTS postgis;

-- Verify PostGIS is working
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'postgis') THEN
        RAISE EXCEPTION 'PostGIS extension failed to install';
    END IF;
END
$$;
