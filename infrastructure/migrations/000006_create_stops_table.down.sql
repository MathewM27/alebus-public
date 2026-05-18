-- Drop stops table, index, and helper function
DROP FUNCTION IF EXISTS insert_stop(TEXT, TEXT, DOUBLE PRECISION, DOUBLE PRECISION);
DROP INDEX IF EXISTS idx_stops_location;
DROP TABLE IF EXISTS stops;
