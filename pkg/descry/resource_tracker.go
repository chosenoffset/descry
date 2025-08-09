package descry

import (
	"context"
	"fmt"
	"runtime"
	"syscall"
	"time"
)

// ResourceTracker provides comprehensive resource monitoring for rule evaluation
type ResourceTracker struct {
	memoryTracker *MemoryTracker
	cpuTracker    *CPUTracker
	ctx           context.Context
	cancel        context.CancelFunc
}

// MemoryTracker monitors memory usage with absolute budget limits
type MemoryTracker struct {
	initialMemory uint64
	maxMemory     uint64
	budget        uint64
	checkInterval time.Duration
}

// CPUTracker monitors actual CPU time usage (not wall-clock time)
type CPUTracker struct {
	startTime   time.Time
	startCPU    time.Duration
	maxCPUTime  time.Duration
	lastCheck   time.Time
}

// NewResourceTracker creates a new resource tracker with the specified limits
func NewResourceTracker(ctx context.Context, memoryLimit uint64, cpuLimit time.Duration) *ResourceTracker {
	// Create child context with cancellation
	childCtx, cancel := context.WithCancel(ctx)
	
	tracker := &ResourceTracker{
		ctx:    childCtx,
		cancel: cancel,
	}
	
	// Initialize memory tracker
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	tracker.memoryTracker = &MemoryTracker{
		initialMemory: m.Alloc,
		maxMemory:     m.Alloc + memoryLimit,
		budget:        memoryLimit,
		checkInterval: 10 * time.Millisecond,
	}
	
	// Initialize CPU tracker
	tracker.cpuTracker = newCPUTracker(cpuLimit)
	
	return tracker
}

// newCPUTracker creates a CPU tracker that measures actual CPU time
func newCPUTracker(maxCPUTime time.Duration) *CPUTracker {
	var usage syscall.Rusage
	err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage)
	if err != nil {
		// Fallback to wall-clock time if syscall fails
		return &CPUTracker{
			startTime:   time.Now(),
			startCPU:    0,
			maxCPUTime:  maxCPUTime,
			lastCheck:   time.Now(),
		}
	}
	
	// Calculate current CPU time (user + system)
	startCPU := time.Duration(usage.Utime.Sec)*time.Second + 
	           time.Duration(usage.Utime.Usec)*time.Microsecond +
	           time.Duration(usage.Stime.Sec)*time.Second + 
	           time.Duration(usage.Stime.Usec)*time.Microsecond
	
	return &CPUTracker{
		startTime:   time.Now(),
		startCPU:    startCPU,
		maxCPUTime:  maxCPUTime,
		lastCheck:   time.Now(),
	}
}

// CheckMemoryLimit verifies that memory usage is within limits
func (mt *MemoryTracker) CheckMemoryLimit() error {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	// Check absolute memory limit
	if m.Alloc > mt.maxMemory {
		return &ResourceLimitError{
			Resource: "memory",
			Current:  m.Alloc,
			Limit:    mt.maxMemory,
			Message:  fmt.Sprintf("memory limit exceeded: current=%d bytes, limit=%d bytes", m.Alloc, mt.maxMemory),
		}
	}
	
	return nil
}

// CheckCPULimit verifies that CPU usage is within limits
func (ct *CPUTracker) CheckCPULimit() error {
	var usage syscall.Rusage
	err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage)
	if err != nil {
		// Fallback to wall-clock time measurement
		wallTime := time.Since(ct.startTime)
		if wallTime > ct.maxCPUTime {
			return &ResourceLimitError{
				Resource: "cpu_fallback",
				Current:  uint64(wallTime.Nanoseconds()),
				Limit:    uint64(ct.maxCPUTime.Nanoseconds()),
				Message:  fmt.Sprintf("CPU time limit exceeded (fallback): wall-time=%v, limit=%v", wallTime, ct.maxCPUTime),
			}
		}
		return nil
	}
	
	// Calculate current CPU time
	currentCPU := time.Duration(usage.Utime.Sec)*time.Second + 
	             time.Duration(usage.Utime.Usec)*time.Microsecond +
	             time.Duration(usage.Stime.Sec)*time.Second + 
	             time.Duration(usage.Stime.Usec)*time.Microsecond
	
	cpuUsed := currentCPU - ct.startCPU
	if cpuUsed > ct.maxCPUTime {
		return &ResourceLimitError{
			Resource: "cpu",
			Current:  uint64(cpuUsed.Nanoseconds()),
			Limit:    uint64(ct.maxCPUTime.Nanoseconds()),
			Message:  fmt.Sprintf("CPU time limit exceeded: used=%v, limit=%v", cpuUsed, ct.maxCPUTime),
		}
	}
	
	return nil
}

// CheckLimits verifies both memory and CPU limits
func (rt *ResourceTracker) CheckLimits() error {
	// Check if context was cancelled
	select {
	case <-rt.ctx.Done():
		return &ResourceLimitError{
			Resource: "context",
			Current:  0,
			Limit:    0,
			Message:  fmt.Sprintf("evaluation cancelled: %v", rt.ctx.Err()),
		}
	default:
	}
	
	// Check memory limits
	if err := rt.memoryTracker.CheckMemoryLimit(); err != nil {
		rt.cancel() // Cancel context on limit violation
		return err
	}
	
	// Check CPU limits
	if err := rt.cpuTracker.CheckCPULimit(); err != nil {
		rt.cancel() // Cancel context on limit violation
		return err
	}
	
	return nil
}

// Cancel cancels the resource tracker context
func (rt *ResourceTracker) Cancel() {
	rt.cancel()
}

// Context returns the tracker's context for cancellation monitoring
func (rt *ResourceTracker) Context() context.Context {
	return rt.ctx
}

// GetMemoryStats returns current memory usage statistics
func (rt *ResourceTracker) GetMemoryStats() MemoryStats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	return MemoryStats{
		CurrentAlloc:  m.Alloc,
		InitialAlloc:  rt.memoryTracker.initialMemory,
		MaxAllowed:    rt.memoryTracker.maxMemory,
		BudgetUsed:    float64(m.Alloc-rt.memoryTracker.initialMemory) / float64(rt.memoryTracker.budget) * 100,
	}
}

// GetCPUStats returns current CPU usage statistics
func (rt *ResourceTracker) GetCPUStats() CPUStats {
	wallTime := time.Since(rt.cpuTracker.startTime)
	
	var usage syscall.Rusage
	var cpuTime time.Duration
	
	err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage)
	if err == nil {
		currentCPU := time.Duration(usage.Utime.Sec)*time.Second + 
		             time.Duration(usage.Utime.Usec)*time.Microsecond +
		             time.Duration(usage.Stime.Sec)*time.Second + 
		             time.Duration(usage.Stime.Usec)*time.Microsecond
		cpuTime = currentCPU - rt.cpuTracker.startCPU
	} else {
		cpuTime = wallTime // Fallback
	}
	
	return CPUStats{
		CPUTimeUsed:   cpuTime,
		WallTimeUsed:  wallTime,
		MaxCPUTime:    rt.cpuTracker.maxCPUTime,
		CPUEfficiency: float64(cpuTime.Nanoseconds()) / float64(wallTime.Nanoseconds()) * 100,
	}
}

// ResourceLimitError represents a resource limit violation
type ResourceLimitError struct {
	Resource string
	Current  uint64
	Limit    uint64
	Message  string
}

func (e *ResourceLimitError) Error() string {
	return e.Message
}

// IsResourceLimitError checks if an error is a resource limit violation
func IsResourceLimitError(err error) bool {
	_, ok := err.(*ResourceLimitError)
	return ok
}

// MemoryStats provides memory usage statistics
type MemoryStats struct {
	CurrentAlloc uint64  // Current allocated bytes
	InitialAlloc uint64  // Initial allocated bytes at start
	MaxAllowed   uint64  // Maximum allowed bytes
	BudgetUsed   float64 // Percentage of budget used
}

// CPUStats provides CPU usage statistics
type CPUStats struct {
	CPUTimeUsed   time.Duration // Actual CPU time used
	WallTimeUsed  time.Duration // Wall-clock time elapsed
	MaxCPUTime    time.Duration // Maximum allowed CPU time
	CPUEfficiency float64       // CPU time / wall time * 100
}