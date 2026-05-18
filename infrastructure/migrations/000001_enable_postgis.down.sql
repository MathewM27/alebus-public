-- Disable PostGIS extension
-- WARNING: This will fail if any tables use PostGIS types

DROP EXTENSION IF EXISTS postgis CASCADE;
