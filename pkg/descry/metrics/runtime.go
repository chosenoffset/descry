package metrics

import (
	"runtime"
	"sync"
	"time"
)

type RuntimeMetrics struct {
	// Memory metrics
	HeapAlloc      uint64    `json:"heap_alloc"`
	HeapSys        uint64    `json:"heap_sys"`
	HeapIdle       uint64    `json:"heap_idle"`
	HeapInuse      uint64    `json:"heap_inuse"`
	HeapReleased   uint64    `json:"heap_released"`
	HeapObjects    uint64    `json:"heap_objects"`
	StackInuse     uint64    `json:"stack_inuse"`
	StackSys       uint64    `json:"stack_sys"`
	MSpanInuse     uint64    `json:"mspan_inuse"`
	MSpanSys       uint64    `json:"mspan_sys"`
	MCacheInuse    uint64    `json:"mcache_inuse"`
	MCacheSys      uint64    `json:"mcache_sys"`
	OtherSys       uint64    `json:"other_sys"`
	Sys            uint64    `json:"sys"`
	
	// GC metrics
	NextGC         uint64    `json:"next_gc"`
	LastGC         uint64    `json:"last_gc"`
	PauseTotalNs   uint64    `json:"pause_total_ns"`
	NumGC          uint32    `json:"num_gc"`
	NumForcedGC    uint32    `json:"num_forced_gc"`
	GCCPUFraction  float64   `json:"gc_cpu_fraction"`
	
	// Goroutine metrics
	NumGoroutine   int       `json:"num_goroutine"`
	NumCgoCall     int64     `json:"num_cgo_call"`
	
	// Timestamp
	Timestamp      time.Time `json:"timestamp"`
}

type RuntimeCollector struct {
	mu             sync.RWMutex
	current        RuntimeMetrics
	history        []RuntimeMetrics
	maxHistory     int
	collectInterval time.Duration
	stopCh         chan struct{}
	running        bool
}

func NewRuntimeCollector(maxHistory int, collectInterval time.Duration) *RuntimeCollector {
	return &RuntimeCollector{
		history:         make([]RuntimeMetrics, 0, maxHistory),
		maxHistory:      maxHistory,
		collectInterval: collectInterval,
		stopCh:          make(chan struct{}),
	}
}

func (rc *RuntimeCollector) Start() {
	rc.mu.Lock()
	if rc.running {
		rc.mu.Unlock()
		return
	}
	rc.running = true
	rc.mu.Unlock()

	go rc.collectLoop()
}

func (rc *RuntimeCollector) Stop() {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	
	if !rc.running {
		return
	}
	rc.running = false
	close(rc.stopCh)
	rc.stopCh = make(chan struct{}) // Recreate for potential restart
}

func (rc *RuntimeCollector) collectLoop() {
	ticker := time.NewTicker(rc.collectInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rc.collectMetrics()
		case <-rc.stopCh:
			return
		}
	}
}

func (rc *RuntimeCollector) collectMetrics() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	metrics := RuntimeMetrics{
		// Memory metrics
		HeapAlloc:      m.HeapAlloc,
		HeapSys:        m.HeapSys,
		HeapIdle:       m.HeapIdle,
		HeapInuse:      m.HeapInuse,
		HeapReleased:   m.HeapReleased,
		HeapObjects:    m.HeapObjects,
		StackInuse:     m.StackInuse,
		StackSys:       m.StackSys,
		MSpanInuse:     m.MSpanInuse,
		MSpanSys:       m.MSpanSys,
		MCacheInuse:    m.MCacheInuse,
		MCacheSys:      m.MCacheSys,
		OtherSys:       m.OtherSys,
		Sys:            m.Sys,
		
		// GC metrics
		NextGC:         m.NextGC,
		LastGC:         m.LastGC,
		PauseTotalNs:   m.PauseTotalNs,
		NumGC:          m.NumGC,
		NumForcedGC:    m.NumForcedGC,
		GCCPUFraction:  m.GCCPUFraction,
		
		// Goroutine metrics
		NumGoroutine:   runtime.NumGoroutine(),
		NumCgoCall:     runtime.NumCgoCall(),
		
		// Timestamp
		Timestamp:      time.Now(),
	}

	rc.mu.Lock()
	rc.current = metrics
	
	// Add to history
	rc.history = append(rc.history, metrics)
	if len(rc.history) > rc.maxHistory {
		// Remove oldest entry
		copy(rc.history, rc.history[1:])
		rc.history = rc.history[:rc.maxHistory]
	}
	rc.mu.Unlock()
}

func (rc *RuntimeCollector) GetCurrent() RuntimeMetrics {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.current
}

func (rc *RuntimeCollector) GetHistory() []RuntimeMetrics {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	
	// Return a copy to prevent data races
	history := make([]RuntimeMetrics, len(rc.history))
	copy(history, rc.history)
	return history
}

func (rc *RuntimeCollector) GetHistoryWindow(duration time.Duration) []RuntimeMetrics {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	
	if len(rc.history) == 0 {
		return []RuntimeMetrics{}
	}
	
	cutoff := time.Now().Add(-duration)
	var result []RuntimeMetrics
	
	for _, metrics := range rc.history {
		if metrics.Timestamp.After(cutoff) {
			result = append(result, metrics)
		}
	}
	
	return result
}

// Utility functions for common metrics calculations

func (rc *RuntimeCollector) GetHeapAllocMB() float64 {
	current := rc.GetCurrent()
	return float64(current.HeapAlloc) / (1024 * 1024)
}

func (rc *RuntimeCollector) GetHeapSysMB() float64 {
	current := rc.GetCurrent()
	return float64(current.HeapSys) / (1024 * 1024)
}

func (rc *RuntimeCollector) GetGoroutineCount() int {
	current := rc.GetCurrent()
	return current.NumGoroutine
}

func (rc *RuntimeCollector) GetGCCount() uint32 {
	current := rc.GetCurrent()
	return current.NumGC
}

// Trend calculation: returns the change rate per minute for heap allocation
func (rc *RuntimeCollector) GetHeapAllocTrend(duration time.Duration) float64 {
	history := rc.GetHistoryWindow(duration)
	if len(history) < 2 {
		return 0
	}
	
	oldest := history[0]
	newest := history[len(history)-1]
	
	timeDiff := newest.Timestamp.Sub(oldest.Timestamp)
	if timeDiff.Seconds() == 0 {
		return 0
	}
	
	allocDiff := float64(newest.HeapAlloc) - float64(oldest.HeapAlloc)
	bytesPerSecond := allocDiff / timeDiff.Seconds()
	
	// Convert to bytes per minute
	return bytesPerSecond * 60
}

// Calculate average metric over a time window
func (rc *RuntimeCollector) GetAverageHeapAlloc(duration time.Duration) float64 {
	history := rc.GetHistoryWindow(duration)
	if len(history) == 0 {
		return 0
	}
	
	var sum uint64
	for _, metrics := range history {
		sum += metrics.HeapAlloc
	}
	
	return float64(sum) / float64(len(history))
}

func (rc *RuntimeCollector) GetMaxHeapAlloc(duration time.Duration) uint64 {
	history := rc.GetHistoryWindow(duration)
	if len(history) == 0 {
		return 0
	}
	
	max := history[0].HeapAlloc
	for _, metrics := range history {
		if metrics.HeapAlloc > max {
			max = metrics.HeapAlloc
		}
	}
	
	return max
}