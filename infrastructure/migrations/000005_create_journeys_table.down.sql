-- Drop journeys table and indexes
DROP INDEX IF EXISTS idx_journeys_expiration;
DROP INDEX IF EXISTS idx_journeys_status;
DROP INDEX IF EXISTS idx_journeys_user_status;
DROP TABLE IF EXISTS journeys;
