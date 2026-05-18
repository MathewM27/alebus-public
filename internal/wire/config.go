// Package wire provides dependency injection wiring for the application.
// This package assembles infrastructure and application components without
// modifying handler constructors or leaking infrastructure concerns.
//
// Usage:
//
//	cfg := wire.LoadFromEnv()
//	pgInfra, err := wire.NewPostgresInfra(ctx, cfg)
//	redisInfra, err := wire.NewRedisInfra(ctx, cfg)
//	if err != nil { /* service cannot start without Redis */ }
//	handlers := wire.NewJourneyHandlers(pgInfra, redisInfra)
package wire

import (
	"os"
	"strconv"
	"time"
)

// Config holds all configuration for infrastructure wiring.
// RedisEnabled is used ONLY at composition time, never inside handlers.
type Config struct {
	// Postgres
	DatabaseURL string

	// Redis
	RedisURL              string
	RedisEnabled          bool // Feature flag — only used at composition time
	RedisPassword         string
	RedisSentinelMaster   string
	RedisSentinelAddrs    string
	RedisSentinelPassword string

	// Timeouts
	DBTimeout    time.Duration
	RedisTimeout time.Duration
}

// LoadFromEnv loads configuration from environment variables.
// Defaults are provided for development.
func LoadFromEnv() Config {
	cfg := Config{
		DatabaseURL:           getEnv("DATABASE_URL", "postgres://alebus:alebus@127.0.0.1:5432/alebus?sslmode=disable"),
		RedisURL:              getEnv("REDIS_ADDR", "127.0.0.1:6379"),
		RedisEnabled:          getEnvBool("REDIS_ENABLED", true),
		RedisPassword:         getEnv("REDIS_PASSWORD", ""),
		RedisSentinelMaster:   getEnv("REDIS_SENTINEL_MASTER", "mymaster"),
		RedisSentinelAddrs:    getEnv("REDIS_SENTINEL_ADDRS", "localhost:26379"),
		RedisSentinelPassword: getEnv("REDIS_SENTINEL_PASSWORD", ""),
		DBTimeout:             getEnvDuration("DB_TIMEOUT", 10*time.Second),
		RedisTimeout:          getEnvDuration("REDIS_TIMEOUT", 5*time.Second),
	}
	return cfg
}

// getEnv returns the environment variable value or the default.
func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

// getEnvBool parses a boolean environment variable.
func getEnvBool(key string, defaultValue bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return defaultValue
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return defaultValue
	}
	return b
}

// getEnvDuration parses a duration environment variable.
func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return defaultValue
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return defaultValue
	}
	return d
}
