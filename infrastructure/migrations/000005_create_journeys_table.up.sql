-- Create journeys table for Journey aggregate
-- Stores journey state including recommendations (JSONB)

CREATE TABLE journeys (
    journey_id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    
    -- Origin location (embedded value object)
    origin_lat DOUBLE PRECISION NOT NULL,
    origin_lon DOUBLE PRECISION NOT NULL,
    origin_stop_id TEXT NOT NULL,
    destination_stop_id TEXT NOT NULL,
    
    -- Recommendations (JSONB - complex nested structure)
    recommended_buses JSONB NOT NULL DEFAULT '[]',
    active_bus_id TEXT,
    
    -- Journey state
    status INT NOT NULL DEFAULT 0,
    last_switch_reason INT NOT NULL DEFAULT 0,
    last_proximity_level INT NOT NULL DEFAULT 0,
    decline_count INT NOT NULL DEFAULT 0,
    required_direction INT NOT NULL DEFAULT 0,
    
    -- Timing
    estimated_duration_ns BIGINT NOT NULL DEFAULT 0,
    expiration_time TIMESTAMPTZ NOT NULL,
    boarding_window_started_at TIMESTAMPTZ,
    boarded_at TIMESTAMPTZ,
    
    -- Aggregate metadata
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for finding active journeys by user (most common query)
CREATE INDEX idx_journeys_user_status ON journeys (user_id, status);

-- Index for status-based queries (e.g., find all active journeys)
CREATE INDEX idx_journeys_status ON journeys (status);

-- Index for expiration time (for cleanup jobs)
CREATE INDEX idx_journeys_expiration ON journeys (expiration_time);

-- Add comments for documentation
COMMENT ON TABLE journeys IS 'Journey aggregate - user journey tracking state';
COMMENT ON COLUMN journeys.status IS 'JourneyStatus enum: 0=Searching, 1=Tracking, 2=BoardingPrompt, 3=Boarded, 4=Completed, 5=Cancelled, 6=Expired';
COMMENT ON COLUMN journeys.last_switch_reason IS 'JourneySwitchReason enum: 0=Unknown, 1=User, 2=Overtaken, 3=Location, 4=Offline, 5=TerminalDelay, 6=UserDeclined';
COMMENT ON COLUMN journeys.last_proximity_level IS 'ProximityLevel enum: 0=None, 1=Approaching, 2=Nearby, 3=Arrived';
COMMENT ON COLUMN journeys.required_direction IS 'Direction enum: 0=Outbound, 1=Inbound';
COMMENT ON COLUMN journeys.estimated_duration_ns IS 'Duration in nanoseconds (Go time.Duration)';
COMMENT ON COLUMN journeys.recommended_buses IS 'JSONB array of EnhancedBusRecommendation objects';
