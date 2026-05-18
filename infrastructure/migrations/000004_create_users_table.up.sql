-- Create users table for User aggregate
-- Stores user profile, subscription, and saved locations

CREATE TABLE users (
    user_id TEXT PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    
    -- Subscription (embedded value object)
    subscription_status INT NOT NULL DEFAULT 0,
    subscription_plan INT NOT NULL DEFAULT 0,
    subscription_start_date TIMESTAMPTZ NOT NULL,
    subscription_expiry_date TIMESTAMPTZ NOT NULL,
    
    -- Saved locations (JSONB array)
    saved_locations JSONB NOT NULL DEFAULT '[]',
    
    -- Aggregate metadata
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for email lookups
CREATE INDEX idx_users_email ON users (email);

-- Add comments for documentation
COMMENT ON TABLE users IS 'User aggregate - user profile and subscription';
COMMENT ON COLUMN users.subscription_status IS 'SubscriptionStatus enum: 0=Active, 1=Inactive, 2=Suspended, 3=Cancelled, 4=Expired';
COMMENT ON COLUMN users.subscription_plan IS 'SubscriptionPlan enum: 0=Free, 1=Basic, 2=Premium';
COMMENT ON COLUMN users.saved_locations IS 'JSONB array: [{"name": "Home", "location": {"latitude": 0.0, "longitude": 0.0}, "stopId": "stop1"}]';
