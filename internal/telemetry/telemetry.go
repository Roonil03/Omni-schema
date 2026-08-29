package telemetry

import (
	"log/slog"
	"math"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

var (
	HttpRequests       atomic.Uint64
	WebSocketConns     atomic.Uint64
	SchemasRegistered  atomic.Uint64
	EventsPublished    atomic.Uint64
	EventsDropped      atomic.Uint64
	ConversionFailures atomic.Uint64

	Logger *slog.Logger
)

type histogram struct {
	mu     sync.Mutex
	count  uint64
	sumNs  uint64
	values []float64
}

var (
	parseLat   histogram
	convertLat histogram
	encodeLat  histogram
	streamLat  histogram
)

func init() {
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	Logger = slog.New(slog.NewJSONHandler(os.Stdout, opts))
	slog.SetDefault(Logger)
}

func ObserveParse(d time.Duration)    { observe(&parseLat, d) }
func ObserveConvert(d time.Duration) { observe(&convertLat, d) }
func ObserveEncode(d time.Duration)  { observe(&encodeLat, d) }
func ObserveStream(d time.Duration)  { observe(&streamLat, d) }

func observe(h *histogram, d time.Duration) {
	ms := float64(d.Nanoseconds()) / 1e6
	h.mu.Lock()
	h.count++
	h.sumNs += uint64(d.Nanoseconds())
	if len(h.values) < 4096 {
		h.values = append(h.values, ms)
	} else {
		h.values[h.count%4096] = ms
	}
	h.mu.Unlock()
}

func snapshotHist(h *histogram) map[string]float64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.count == 0 {
		return map[string]float64{"count": 0, "p50": 0, "p95": 0, "p99": 0}
	}
	cp := append([]float64(nil), h.values...)
	return map[string]float64{
		"count": float64(h.count),
		"p50":   percentile(cp, 0.50),
		"p95":   percentile(cp, 0.95),
		"p99":   percentile(cp, 0.99),
	}
}

func percentile(vals []float64, p float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	// insertion-ish sort copy
	s := append([]float64(nil), vals...)
	for i := 1; i < len(s); i++ {
		j := i
		for j > 0 && s[j-1] > s[j] {
			s[j-1], s[j] = s[j], s[j-1]
			j--
		}
	}
	idx := int(math.Ceil(p*float64(len(s)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(s) {
		idx = len(s) - 1
	}
	return s[idx]
}

func GetMetricsSnapshot() map[string]any {
	return map[string]any{
		"http_requests_total":       HttpRequests.Load(),
		"websocket_conns_active":    WebSocketConns.Load(),
		"schemas_registered_total":  SchemasRegistered.Load(),
		"events_published_total":    EventsPublished.Load(),
		"events_dropped_total":      EventsDropped.Load(),
		"conversion_failures_total": ConversionFailures.Load(),
		"latency_ms": map[string]any{
			"parse":    snapshotHist(&parseLat),
			"convert":  snapshotHist(&convertLat),
			"encode":   snapshotHist(&encodeLat),
			"stream":   snapshotHist(&streamLat),
		},
	}
}
