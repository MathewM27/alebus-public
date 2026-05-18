-- Rollback: Remove PostGIS stops sync columns

-- Remove indexes
DROP INDEX IF EXISTS idx_stops_route_ids;

-- Remove columns
ALTER TABLE stops DROP COLUMN IF EXISTS route_ids;
ALTER TABLE stops DROP COLUMN IF EXISTS cumulative_distance_meters;

-- Clear the stops table (will be repopulated if re-migrated)
TRUNCATE stops;
