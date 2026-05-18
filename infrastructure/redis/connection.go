// Package redis provides Redis connection and client management for live bus state.
// This is infrastructure code - it must NOT contain business logic.
package redis

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

// ─────────────────────────────────────────────────────────────────────────────
// Key Patterns (Phase 5D Keyspace)
// ─────────────────────────────────────────────────────────────────────────────

const (
	// BusStateKeyPrefix is the prefix for individual bus state hash keys.
	// Full pattern: bus:{busID}:state
	BusStateKeyPrefix = "bus:"
	BusStateKeySuffix = ":state"

	// RouteIndexKeyPrefix is the prefix for route bus index sets.
	// Full pattern: route:{routeID}:dir:{direction}:buses
	RouteIndexKeyPrefix = "route:"
	RouteIndexKeyInfix  = ":dir:"
	RouteIndexKeySuffix = ":buses"

	// BusGeoKey is the global geo index for all bus positions.
	BusGeoKey = "buses:geo"
)

// TTL values for Redis keys.
// Phase 2: TTL enforced on state hash to prevent ghost buses and memory bloat.
const (
	// BusStateTTL is the TTL for bus state hash keys (90 seconds).
	// If a bus stops sending updates, its state expires automatically.
	// This prevents ghost buses and bounds Redis memory growth.
	BusStateTTL = 90 * time.Second // PHASE 2: TTL enforced atomically by Lua

	// RouteIndexTTL is the TTL for route bus index sets (90 seconds).
	// Route indexes are derived data and can safely expire.
	RouteIndexTTL = 90 * time.Second

	// OfflineThreshold is no longer used (replaced by TTL-based expiry).
	// Kept for backward compatibility during transition.
	OfflineThreshold = 120 * time.Second
)

// ─────────────────────────────────────────────────────────────────────────────
// Key Builders
// ─────────────────────────────────────────────────────────────────────────────

// BusStateKey returns the Redis key for a bus's live state hash.
// Format: bus:{busID}:state
func BusStateKey(busID string) string {
	return BusStateKeyPrefix + busID + BusStateKeySuffix
}

// RouteIndexKey returns the Redis key for a route's bus index set.
// Format: route:{routeID}:dir:{direction}:buses
func RouteIndexKey(routeID string, direction int) string {
	return fmt.Sprintf("%s%s%s%d%s", RouteIndexKeyPrefix, routeID, RouteIndexKeyInfix, direction, RouteIndexKeySuffix)
}

// ─────────────────────────────────────────────────────────────────────────────
// Configuration
// ─────────────────────────────────────────────────────────────────────────────

// Config holds Redis connection configuration.
type Config struct {
	// Addr is the Redis server address (host:port).
	Addr string

	// Password is the Redis password (empty for no auth).
	Password string

	// DB is the Redis database number (0-15).
	DB int

	// PoolSize is the maximum number of connections in the pool.
	PoolSize int

	// MinIdleConns is the minimum number of idle connections.
	MinIdleConns int

	// DialTimeout is the timeout for establishing connections.
	DialTimeout time.Duration

	// ReadTimeout is the timeout for read operations.
	ReadTimeout time.Duration

	// WriteTimeout is the timeout for write operations.
	WriteTimeout time.Duration
}

// DefaultConfig returns a Config with sensible defaults.
// REDIS_URL is read from environment variable.
func DefaultConfig() Config {
	addr := os.Getenv("REDIS_URL")
	if addr == "" {
		// Use explicit IPv4 to avoid Windows IPv6 resolution issues
		addr = "127.0.0.1:6379"
	}

	return Config{
		Addr:         addr,
		Password:     os.Getenv("REDIS_PASSWORD"),
		DB:           0,
		PoolSize:     10,
		MinIdleConns: 2,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Client
// ─────────────────────────────────────────────────────────────────────────────

// Client wraps the go-redis client to provide a clean interface.
type Client struct {
	client *redis.Client
}

// NewClientFromGoRedis wraps an existing go-redis client.
// Useful when the client was created via Sentinel failover or other factories.
func NewClientFromGoRedis(client *redis.Client) *Client {
	return &Client{client: client}
}

// NewClient creates a new Redis client with the given configuration.
func NewClient(ctx context.Context, cfg Config) (*Client, error) {
	opts := &redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	}

	client := redis.NewClient(opts)

	// Verify connection
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to ping Redis: %w", err)
	}

	return &Client{client: client}, nil
}

// NewClientFromAddr creates a new Redis client from an address string.
// This is a convenience function for simple use cases.
func NewClientFromAddr(ctx context.Context, addr string) (*Client, error) {
	cfg := DefaultConfig()
	cfg.Addr = addr
	return NewClient(ctx, cfg)
}

// Close closes the Redis client connection.
func (c *Client) Close() error {
	return c.client.Close()
}

// Ping verifies the Redis connection is alive.
func (c *Client) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

// Underlying returns the underlying go-redis client.
// Use this only when you need direct access to Redis commands.
func (c *Client) Underlying() *redis.Client {
	return c.client
}
