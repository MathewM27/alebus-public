package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

// ─────────────────────────────────────────────────────────────────────────────
// Structured Logging - Phase 5
// ─────────────────────────────────────────────────────────────────────────────
//
// This module provides structured logging for Redis operations with:
//   - Request IDs for tracing
//   - Bus/Route/Journey tagging
//   - Latency and error categorization
//   - JSON output for log aggregation (ELK, Loki, etc.)
//   - Log level filtering
//   - Context propagation

// LogLevel represents the severity of a log entry.
type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
	LogLevelFatal
)

// String returns the string representation of a log level.
func (l LogLevel) String() string {
	switch l {
	case LogLevelDebug:
		return "DEBUG"
	case LogLevelInfo:
		return "INFO"
	case LogLevelWarn:
		return "WARN"
	case LogLevelError:
		return "ERROR"
	case LogLevelFatal:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Log Entry
// ─────────────────────────────────────────────────────────────────────────────

// LogEntry represents a structured log entry.
type LogEntry struct {
	// Core fields
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	RequestID string    `json:"request_id,omitempty"`

	// Context fields
	BusID     string `json:"bus_id,omitempty"`
	RouteID   string `json:"route_id,omitempty"`
	JourneyID string `json:"journey_id,omitempty"`
	Region    string `json:"region,omitempty"`

	// Operation fields
	Operation     string `json:"operation,omitempty"`
	LatencyUs     int64  `json:"latency_us,omitempty"`
	Success       bool   `json:"success,omitempty"`
	ErrorCategory string `json:"error_category,omitempty"`
	ErrorMessage  string `json:"error_message,omitempty"`

	// Redis fields
	RedisCommand string `json:"redis_command,omitempty"`
	RedisKey     string `json:"redis_key,omitempty"`
	RedisNode    string `json:"redis_node,omitempty"`

	// Performance fields
	QueueDepth   int     `json:"queue_depth,omitempty"`
	PoolSize     int     `json:"pool_size,omitempty"`
	ActiveConns  int     `json:"active_conns,omitempty"`
	MemoryUsedMB float64 `json:"memory_used_mb,omitempty"`

	// Source info
	File     string `json:"file,omitempty"`
	Line     int    `json:"line,omitempty"`
	Function string `json:"function,omitempty"`

	// Custom fields
	Extra map[string]interface{} `json:"extra,omitempty"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Request Context
// ─────────────────────────────────────────────────────────────────────────────

type requestContextKey struct{}

// RequestContext holds request-scoped information.
type RequestContext struct {
	RequestID string
	BusID     string
	RouteID   string
	JourneyID string
	Region    string
	StartTime time.Time
	Extra     map[string]interface{}
}

// NewRequestContext creates a new request context with a generated ID.
func NewRequestContext() *RequestContext {
	return &RequestContext{
		RequestID: uuid.New().String()[:8],
		StartTime: time.Now(),
		Extra:     make(map[string]interface{}),
	}
}

// WithBus adds bus information to the context.
func (rc *RequestContext) WithBus(busID string) *RequestContext {
	rc.BusID = busID
	return rc
}

// WithRoute adds route information to the context.
func (rc *RequestContext) WithRoute(routeID string) *RequestContext {
	rc.RouteID = routeID
	return rc
}

// WithJourney adds journey information to the context.
func (rc *RequestContext) WithJourney(journeyID string) *RequestContext {
	rc.JourneyID = journeyID
	return rc
}

// WithRegion adds region information to the context.
func (rc *RequestContext) WithRegion(region string) *RequestContext {
	rc.Region = region
	return rc
}

// WithExtra adds custom fields to the context.
func (rc *RequestContext) WithExtra(key string, value interface{}) *RequestContext {
	rc.Extra[key] = value
	return rc
}

// ContextWithRequest adds a request context to a Go context.
func ContextWithRequest(ctx context.Context, rc *RequestContext) context.Context {
	return context.WithValue(ctx, requestContextKey{}, rc)
}

// RequestFromContext extracts request context from a Go context.
func RequestFromContext(ctx context.Context) *RequestContext {
	if rc, ok := ctx.Value(requestContextKey{}).(*RequestContext); ok {
		return rc
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Logger
// ─────────────────────────────────────────────────────────────────────────────

// LoggerConfig configures the structured logger.
type LoggerConfig struct {
	// Level is the minimum log level to output.
	Level LogLevel

	// Output is where logs are written.
	Output io.Writer

	// IncludeSource includes file/line/function in logs.
	IncludeSource bool

	// AsyncBuffer enables async logging with buffering.
	AsyncBuffer int

	// JSONFormat outputs logs as JSON (vs human-readable).
	JSONFormat bool

	// ServiceName identifies this service in logs.
	ServiceName string

	// Environment (dev, staging, prod).
	Environment string
}

// DefaultLoggerConfig returns sensible defaults.
func DefaultLoggerConfig() LoggerConfig {
	return LoggerConfig{
		Level:         LogLevelInfo,
		Output:        os.Stdout,
		IncludeSource: false,
		AsyncBuffer:   1000,
		JSONFormat:    true,
		ServiceName:   "alebus-redis",
		Environment:   "dev",
	}
}

// StructuredLogger provides structured logging for Redis operations.
type StructuredLogger struct {
	config LoggerConfig
	mu     sync.Mutex
	buffer chan *LogEntry
	closed int32
	wg     sync.WaitGroup

	// Metrics integration
	logCount map[LogLevel]*int64
}

// NewStructuredLogger creates a new structured logger.
func NewStructuredLogger(cfg LoggerConfig) *StructuredLogger {
	sl := &StructuredLogger{
		config:   cfg,
		logCount: make(map[LogLevel]*int64),
	}

	// Initialize counters
	for level := LogLevelDebug; level <= LogLevelFatal; level++ {
		counter := int64(0)
		sl.logCount[level] = &counter
	}

	// Start async processing if enabled
	if cfg.AsyncBuffer > 0 {
		sl.buffer = make(chan *LogEntry, cfg.AsyncBuffer)
		sl.wg.Add(1)
		go sl.processBuffer()
	}

	return sl
}

// processBuffer handles async log writing.
func (sl *StructuredLogger) processBuffer() {
	defer sl.wg.Done()
	for entry := range sl.buffer {
		sl.writeEntry(entry)
	}
}

// Close shuts down the logger gracefully.
func (sl *StructuredLogger) Close() error {
	if !atomic.CompareAndSwapInt32(&sl.closed, 0, 1) {
		return nil
	}
	if sl.buffer != nil {
		close(sl.buffer)
		sl.wg.Wait()
	}
	return nil
}

// log creates and writes a log entry.
func (sl *StructuredLogger) log(ctx context.Context, level LogLevel, msg string, fields ...func(*LogEntry)) {
	if level < sl.config.Level {
		return
	}

	// Increment counter
	if counter, ok := sl.logCount[level]; ok {
		atomic.AddInt64(counter, 1)
	}

	entry := &LogEntry{
		Timestamp: time.Now().UTC(),
		Level:     level.String(),
		Message:   msg,
	}

	// Apply request context
	if rc := RequestFromContext(ctx); rc != nil {
		entry.RequestID = rc.RequestID
		entry.BusID = rc.BusID
		entry.RouteID = rc.RouteID
		entry.JourneyID = rc.JourneyID
		entry.Region = rc.Region
		entry.Extra = rc.Extra
	}

	// Apply additional fields
	for _, fn := range fields {
		fn(entry)
	}

	// Add source info if enabled
	if sl.config.IncludeSource {
		if pc, file, line, ok := runtime.Caller(2); ok {
			entry.File = file
			entry.Line = line
			if fn := runtime.FuncForPC(pc); fn != nil {
				entry.Function = fn.Name()
			}
		}
	}

	// Write async or sync
	if sl.buffer != nil && atomic.LoadInt32(&sl.closed) == 0 {
		select {
		case sl.buffer <- entry:
		default:
			// Buffer full, write sync
			sl.writeEntry(entry)
		}
	} else {
		sl.writeEntry(entry)
	}
}

// writeEntry writes a single log entry.
func (sl *StructuredLogger) writeEntry(entry *LogEntry) {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	if sl.config.JSONFormat {
		data, _ := json.Marshal(entry)
		fmt.Fprintln(sl.config.Output, string(data))
	} else {
		sl.writeHumanReadable(entry)
	}
}

// writeHumanReadable writes a human-readable log line.
func (sl *StructuredLogger) writeHumanReadable(entry *LogEntry) {
	ts := entry.Timestamp.Format("2006-01-02 15:04:05.000")

	// Base line
	line := fmt.Sprintf("[%s] %s %s", ts, entry.Level, entry.Message)

	// Add context
	if entry.RequestID != "" {
		line += fmt.Sprintf(" req=%s", entry.RequestID)
	}
	if entry.BusID != "" {
		line += fmt.Sprintf(" bus=%s", entry.BusID)
	}
	if entry.RouteID != "" {
		line += fmt.Sprintf(" route=%s", entry.RouteID)
	}
	if entry.Operation != "" {
		line += fmt.Sprintf(" op=%s", entry.Operation)
	}
	if entry.LatencyUs > 0 {
		line += fmt.Sprintf(" latency=%dµs", entry.LatencyUs)
	}
	if entry.ErrorMessage != "" {
		line += fmt.Sprintf(" error=%q", entry.ErrorMessage)
	}

	fmt.Fprintln(sl.config.Output, line)
}

// ─────────────────────────────────────────────────────────────────────────────
// Logging Methods
// ─────────────────────────────────────────────────────────────────────────────

// Debug logs a debug message.
func (sl *StructuredLogger) Debug(ctx context.Context, msg string, fields ...func(*LogEntry)) {
	sl.log(ctx, LogLevelDebug, msg, fields...)
}

// Info logs an info message.
func (sl *StructuredLogger) Info(ctx context.Context, msg string, fields ...func(*LogEntry)) {
	sl.log(ctx, LogLevelInfo, msg, fields...)
}

// Warn logs a warning message.
func (sl *StructuredLogger) Warn(ctx context.Context, msg string, fields ...func(*LogEntry)) {
	sl.log(ctx, LogLevelWarn, msg, fields...)
}

// Error logs an error message.
func (sl *StructuredLogger) Error(ctx context.Context, msg string, fields ...func(*LogEntry)) {
	sl.log(ctx, LogLevelError, msg, fields...)
}

// Fatal logs a fatal message.
func (sl *StructuredLogger) Fatal(ctx context.Context, msg string, fields ...func(*LogEntry)) {
	sl.log(ctx, LogLevelFatal, msg, fields...)
}

// ─────────────────────────────────────────────────────────────────────────────
// Field Functions
// ─────────────────────────────────────────────────────────────────────────────

// WithOperation adds operation information.
func WithOperation(op string) func(*LogEntry) {
	return func(e *LogEntry) {
		e.Operation = op
	}
}

// WithLatency adds latency information.
func WithLatency(d time.Duration) func(*LogEntry) {
	return func(e *LogEntry) {
		e.LatencyUs = d.Microseconds()
	}
}

// WithSuccess marks the operation as successful.
func WithSuccess() func(*LogEntry) {
	return func(e *LogEntry) {
		e.Success = true
	}
}

// WithError adds error information.
func WithError(err error, category string) func(*LogEntry) {
	return func(e *LogEntry) {
		e.Success = false
		e.ErrorCategory = category
		if err != nil {
			e.ErrorMessage = err.Error()
		}
	}
}

// WithRedisCommand adds Redis command information.
func WithRedisCommand(cmd, key string) func(*LogEntry) {
	return func(e *LogEntry) {
		e.RedisCommand = cmd
		e.RedisKey = key
	}
}

// WithRedisNode adds Redis node information.
func WithRedisNode(node string) func(*LogEntry) {
	return func(e *LogEntry) {
		e.RedisNode = node
	}
}

// WithPoolStats adds connection pool statistics.
func WithPoolStats(poolSize, activeConns int) func(*LogEntry) {
	return func(e *LogEntry) {
		e.PoolSize = poolSize
		e.ActiveConns = activeConns
	}
}

// WithMemory adds memory usage information.
func WithMemory(usedMB float64) func(*LogEntry) {
	return func(e *LogEntry) {
		e.MemoryUsedMB = usedMB
	}
}

// WithExtra adds custom fields.
func WithExtra(key string, value interface{}) func(*LogEntry) {
	return func(e *LogEntry) {
		if e.Extra == nil {
			e.Extra = make(map[string]interface{})
		}
		e.Extra[key] = value
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Operation Logger
// ─────────────────────────────────────────────────────────────────────────────

// OperationLogger logs Redis operations with automatic timing.
type OperationLogger struct {
	logger *StructuredLogger
}

// NewOperationLogger creates a new operation logger.
func NewOperationLogger(sl *StructuredLogger) *OperationLogger {
	return &OperationLogger{logger: sl}
}

// StartOperation begins tracking an operation.
func (ol *OperationLogger) StartOperation(ctx context.Context, operation string) *OperationTracker {
	return &OperationTracker{
		ctx:       ctx,
		logger:    ol.logger,
		operation: operation,
		startTime: time.Now(),
	}
}

// OperationTracker tracks a single operation.
type OperationTracker struct {
	ctx       context.Context
	logger    *StructuredLogger
	operation string
	startTime time.Time
	redisCmd  string
	redisKey  string
}

// WithRedis adds Redis command info.
func (ot *OperationTracker) WithRedis(cmd, key string) *OperationTracker {
	ot.redisCmd = cmd
	ot.redisKey = key
	return ot
}

// Success logs a successful operation.
func (ot *OperationTracker) Success(msg string, fields ...func(*LogEntry)) {
	allFields := []func(*LogEntry){
		WithOperation(ot.operation),
		WithLatency(time.Since(ot.startTime)),
		WithSuccess(),
	}
	if ot.redisCmd != "" {
		allFields = append(allFields, WithRedisCommand(ot.redisCmd, ot.redisKey))
	}
	allFields = append(allFields, fields...)
	ot.logger.Debug(ot.ctx, msg, allFields...)
}

// Failure logs a failed operation.
func (ot *OperationTracker) Failure(msg string, err error, category string, fields ...func(*LogEntry)) {
	allFields := []func(*LogEntry){
		WithOperation(ot.operation),
		WithLatency(time.Since(ot.startTime)),
		WithError(err, category),
	}
	if ot.redisCmd != "" {
		allFields = append(allFields, WithRedisCommand(ot.redisCmd, ot.redisKey))
	}
	allFields = append(allFields, fields...)
	ot.logger.Error(ot.ctx, msg, allFields...)
}

// ─────────────────────────────────────────────────────────────────────────────
// Log Aggregator
// ─────────────────────────────────────────────────────────────────────────────

// LogAggregator aggregates log statistics over time.
type LogAggregator struct {
	mu sync.RWMutex

	// Counts by level
	counts map[LogLevel]int64

	// Counts by operation
	operationCounts map[string]int64

	// Error counts by category
	errorCounts map[string]int64

	// Slow operation threshold
	slowThreshold time.Duration

	// Slow operation count
	slowOperations int64

	// Start time
	startTime time.Time
}

// NewLogAggregator creates a new log aggregator.
func NewLogAggregator(slowThreshold time.Duration) *LogAggregator {
	return &LogAggregator{
		counts:          make(map[LogLevel]int64),
		operationCounts: make(map[string]int64),
		errorCounts:     make(map[string]int64),
		slowThreshold:   slowThreshold,
		startTime:       time.Now(),
	}
}

// Record records a log entry.
func (la *LogAggregator) Record(entry *LogEntry) {
	la.mu.Lock()
	defer la.mu.Unlock()

	// Parse level
	var level LogLevel
	switch entry.Level {
	case "DEBUG":
		level = LogLevelDebug
	case "INFO":
		level = LogLevelInfo
	case "WARN":
		level = LogLevelWarn
	case "ERROR":
		level = LogLevelError
	case "FATAL":
		level = LogLevelFatal
	}
	la.counts[level]++

	// Operation count
	if entry.Operation != "" {
		la.operationCounts[entry.Operation]++
	}

	// Error count
	if entry.ErrorCategory != "" {
		la.errorCounts[entry.ErrorCategory]++
	}

	// Slow operation
	if entry.LatencyUs > la.slowThreshold.Microseconds() {
		la.slowOperations++
	}
}

// Summary returns a summary of aggregated logs.
func (la *LogAggregator) Summary() LogAggregateSummary {
	la.mu.RLock()
	defer la.mu.RUnlock()

	summary := LogAggregateSummary{
		Duration:       time.Since(la.startTime),
		CountByLevel:   make(map[string]int64),
		CountByOp:      make(map[string]int64),
		CountByError:   make(map[string]int64),
		SlowOperations: la.slowOperations,
	}

	for level, count := range la.counts {
		summary.CountByLevel[level.String()] = count
	}
	for op, count := range la.operationCounts {
		summary.CountByOp[op] = count
	}
	for category, count := range la.errorCounts {
		summary.CountByError[category] = count
	}

	return summary
}

// LogAggregateSummary holds aggregated log statistics.
type LogAggregateSummary struct {
	Duration       time.Duration    `json:"duration"`
	CountByLevel   map[string]int64 `json:"count_by_level"`
	CountByOp      map[string]int64 `json:"count_by_operation"`
	CountByError   map[string]int64 `json:"count_by_error"`
	SlowOperations int64            `json:"slow_operations"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Global Logger Instance (optional)
// ─────────────────────────────────────────────────────────────────────────────

var (
	globalLogger     *StructuredLogger
	globalLoggerOnce sync.Once
)

// SetGlobalLogger sets the global logger instance.
func SetGlobalLogger(sl *StructuredLogger) {
	globalLogger = sl
}

// GetGlobalLogger returns the global logger, creating one if needed.
func GetGlobalLogger() *StructuredLogger {
	globalLoggerOnce.Do(func() {
		if globalLogger == nil {
			globalLogger = NewStructuredLogger(DefaultLoggerConfig())
		}
	})
	return globalLogger
}

// Convenience methods using global logger

// LogDebug logs a debug message using the global logger.
func LogDebug(ctx context.Context, msg string, fields ...func(*LogEntry)) {
	GetGlobalLogger().Debug(ctx, msg, fields...)
}

// LogInfo logs an info message using the global logger.
func LogInfo(ctx context.Context, msg string, fields ...func(*LogEntry)) {
	GetGlobalLogger().Info(ctx, msg, fields...)
}

// LogWarn logs a warning message using the global logger.
func LogWarn(ctx context.Context, msg string, fields ...func(*LogEntry)) {
	GetGlobalLogger().Warn(ctx, msg, fields...)
}

// LogError logs an error message using the global logger.
func LogError(ctx context.Context, msg string, fields ...func(*LogEntry)) {
	GetGlobalLogger().Error(ctx, msg, fields...)
}
