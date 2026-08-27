package telemetry

import (
	"log/slog"
	"os"
	"sync/atomic"
)

var (
	// Core atomic metrics for observability (Issue 44)
	HttpRequests       atomic.Uint64
	WebSocketConns     atomic.Uint64
	SchemasRegistered  atomic.Uint64
	EventsPublished    atomic.Uint64
	EventsDropped      atomic.Uint64
	ConversionFailures atomic.Uint64

	// Logger is the application-wide structured logger (Issue 45)
	Logger *slog.Logger
)

func init() {
	// Initialize a JSON structured logger writing to stdout
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}
	handler := slog.NewJSONHandler(os.Stdout, opts)
	Logger = slog.New(handler)
	slog.SetDefault(Logger)
}

// GetMetricsSnapshot returns a map of the current metric counters for health endpoints.
func GetMetricsSnapshot() map[string]uint64 {
	return map[string]uint64{
		"http_requests_total":       HttpRequests.Load(),
		"websocket_conns_active":    WebSocketConns.Load(),
		"schemas_registered_total":  SchemasRegistered.Load(),
		"events_published_total":    EventsPublished.Load(),
		"events_dropped_total":      EventsDropped.Load(),
		"conversion_failures_total": ConversionFailures.Load(),
	}
}
