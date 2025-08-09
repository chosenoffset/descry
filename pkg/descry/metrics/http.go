package metrics

import (
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// HTTPMetrics tracks HTTP request/response statistics
type HTTPMetrics struct {
	requestCount     int64         // Total requests
	errorCount       int64         // Error responses (>= 400)
	totalResponseTime int64        // Sum of all response times (nanoseconds)
	maxResponseTime   int64        // Maximum response time (nanoseconds)
	pendingRequests   int64        // Currently processing requests
	startTime        time.Time     // When metrics collection started
	
	// Response time samples for statistical analysis
	responseTimes    []int64
	responseTimeMu   sync.RWMutex
	bufferIndex      int64         // Atomic counter for circular buffer
	maxSamples       int
}

// NewHTTPMetrics creates a new HTTP metrics collector
func NewHTTPMetrics(maxSamples int) *HTTPMetrics {
	if maxSamples <= 0 {
		maxSamples = 1000 // Default sample size
	}
	
	return &HTTPMetrics{
		responseTimes: make([]int64, 0, maxSamples),
		maxSamples:   maxSamples,
		startTime:    time.Now(),
	}
}

// HTTPStats represents current HTTP performance statistics
type HTTPStats struct {
	RequestCount      int64   `json:"request_count"`
	ErrorCount        int64   `json:"error_count"`
	ErrorRate         float64 `json:"error_rate"`         // Percentage
	RequestRate       float64 `json:"request_rate"`       // Per second
	AvgResponseTime   int64   `json:"avg_response_time"`  // Nanoseconds
	MaxResponseTime   int64   `json:"max_response_time"`  // Nanoseconds
	PendingRequests   int64   `json:"pending_requests"`
	Timestamp         time.Time `json:"timestamp"`
}

// ResponseWriter wrapper to capture status codes
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.written = true
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(data []byte) (int, error) {
	if !rw.written {
		rw.statusCode = http.StatusOK
		rw.written = true
	}
	return rw.ResponseWriter.Write(data)
}

// Middleware creates HTTP middleware that collects performance metrics
func (h *HTTPMetrics) Middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()
		atomic.AddInt64(&h.pendingRequests, 1)
		defer atomic.AddInt64(&h.pendingRequests, -1)
		
		// Wrap response writer to capture status code
		wrapped := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}
		
		// Process request
		next(wrapped, r)
		
		// Calculate metrics
		duration := time.Since(startTime)
		durationNs := duration.Nanoseconds()
		
		// Update counters
		atomic.AddInt64(&h.requestCount, 1)
		atomic.AddInt64(&h.totalResponseTime, durationNs)
		
		// Update max response time
		for {
			current := atomic.LoadInt64(&h.maxResponseTime)
			if durationNs <= current {
				break
			}
			if atomic.CompareAndSwapInt64(&h.maxResponseTime, current, durationNs) {
				break
			}
		}
		
		// Count errors (status >= 400)
		if wrapped.statusCode >= 400 {
			atomic.AddInt64(&h.errorCount, 1)
		}
		
		// Store response time sample (with lock)
		h.responseTimeMu.Lock()
		if len(h.responseTimes) < h.maxSamples {
			h.responseTimes = append(h.responseTimes, durationNs)
		} else {
			// Circular buffer - use atomic counter for safe indexing
			index := atomic.AddInt64(&h.bufferIndex, 1) % int64(h.maxSamples)
			h.responseTimes[index] = durationNs
		}
		h.responseTimeMu.Unlock()
	}
}

// GetStats returns current HTTP performance statistics
func (h *HTTPMetrics) GetStats() HTTPStats {
	requestCount := atomic.LoadInt64(&h.requestCount)
	errorCount := atomic.LoadInt64(&h.errorCount)
	totalResponseTime := atomic.LoadInt64(&h.totalResponseTime)
	maxResponseTime := atomic.LoadInt64(&h.maxResponseTime)
	pendingRequests := atomic.LoadInt64(&h.pendingRequests)
	
	stats := HTTPStats{
		RequestCount:    requestCount,
		ErrorCount:      errorCount,
		MaxResponseTime: maxResponseTime,
		PendingRequests: pendingRequests,
		Timestamp:       time.Now(),
	}
	
	if requestCount > 0 {
		stats.ErrorRate = float64(errorCount) / float64(requestCount) * 100
		stats.AvgResponseTime = totalResponseTime / requestCount
		
		// Calculate request rate based on actual uptime
		uptime := time.Since(h.startTime)
		if uptime > 0 {
			stats.RequestRate = float64(requestCount) / uptime.Seconds()
		}
	}
	
	return stats
}

// GetResponseTimeSamples returns recent response time samples (thread-safe copy)
func (h *HTTPMetrics) GetResponseTimeSamples() []int64 {
	h.responseTimeMu.RLock()
	defer h.responseTimeMu.RUnlock()
	
	samples := make([]int64, len(h.responseTimes))
	copy(samples, h.responseTimes)
	return samples
}

// Reset clears all metrics (useful for testing)
func (h *HTTPMetrics) Reset() {
	atomic.StoreInt64(&h.requestCount, 0)
	atomic.StoreInt64(&h.errorCount, 0)
	atomic.StoreInt64(&h.totalResponseTime, 0)
	atomic.StoreInt64(&h.maxResponseTime, 0)
	atomic.StoreInt64(&h.pendingRequests, 0)
	atomic.StoreInt64(&h.bufferIndex, 0)
	h.startTime = time.Now()
	
	h.responseTimeMu.Lock()
	h.responseTimes = h.responseTimes[:0]
	h.responseTimeMu.Unlock()
}