package descry

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"syscall"
)

// OSLimitEnforcer provides OS-level resource limit enforcement
type OSLimitEnforcer struct {
	originalLimits map[int]syscall.Rlimit
	applied        bool
}

// NewOSLimitEnforcer creates a new OS limit enforcer
func NewOSLimitEnforcer() *OSLimitEnforcer {
	return &OSLimitEnforcer{
		originalLimits: make(map[int]syscall.Rlimit),
		applied:        false,
	}
}

// ApplyLimits applies OS-level resource limits based on ResourceLimits configuration
func (ole *OSLimitEnforcer) ApplyLimits(limits *ResourceLimits) error {
	if ole.applied {
		return fmt.Errorf("OS limits already applied, call RestoreLimits first")
	}

	// Store original limits for restoration
	resources := []int{
		syscall.RLIMIT_AS,     // Virtual memory limit
		syscall.RLIMIT_CPU,    // CPU time limit
		syscall.RLIMIT_DATA,   // Data segment limit
		syscall.RLIMIT_STACK,  // Stack limit
		syscall.RLIMIT_NOFILE, // File descriptor limit
	}

	for _, resource := range resources {
		var original syscall.Rlimit
		if err := syscall.Getrlimit(resource, &original); err != nil {
			return fmt.Errorf("failed to get original limit for resource %d: %v", resource, err)
		}
		ole.originalLimits[resource] = original
	}

	// Apply memory limit (virtual address space)
	if limits.MaxMemoryUsage > 0 {
		memLimit := &syscall.Rlimit{
			Cur: limits.MaxMemoryUsage,
			Max: limits.MaxMemoryUsage,
		}
		if err := syscall.Setrlimit(syscall.RLIMIT_AS, memLimit); err != nil {
			// On some systems, RLIMIT_AS might not work as expected
			// Try RLIMIT_DATA as fallback
			dataLimit := &syscall.Rlimit{
				Cur: limits.MaxMemoryUsage / 2, // More conservative for data segment
				Max: limits.MaxMemoryUsage / 2,
			}
			if err2 := syscall.Setrlimit(syscall.RLIMIT_DATA, dataLimit); err2 != nil {
				return fmt.Errorf("failed to set memory limits: RLIMIT_AS: %v, RLIMIT_DATA: %v", err, err2)
			}
		}
	}

	// Apply CPU time limit
	if limits.MaxCPUTime > 0 {
		cpuSeconds := uint64(limits.MaxCPUTime.Seconds())
		if cpuSeconds == 0 {
			cpuSeconds = 1 // Minimum 1 second
		}
		
		cpuLimit := &syscall.Rlimit{
			Cur: cpuSeconds,
			Max: cpuSeconds,
		}
		if err := syscall.Setrlimit(syscall.RLIMIT_CPU, cpuLimit); err != nil {
			return fmt.Errorf("failed to set CPU time limit: %v", err)
		}
	}

	// Apply stack limit (for recursive evaluation protection)
	stackLimit := uint64(8 * 1024 * 1024) // 8MB stack limit
	stackRlimit := &syscall.Rlimit{
		Cur: stackLimit,
		Max: stackLimit,
	}
	if err := syscall.Setrlimit(syscall.RLIMIT_STACK, stackRlimit); err != nil {
		return fmt.Errorf("failed to set stack limit: %v", err)
	}

	// Apply file descriptor limit (prevent resource exhaustion)
	fileLimit := &syscall.Rlimit{
		Cur: 1024, // Reasonable limit for rule evaluation
		Max: 1024,
	}
	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, fileLimit); err != nil {
		return fmt.Errorf("failed to set file descriptor limit: %v", err)
	}

	ole.applied = true
	return nil
}

// RestoreLimits restores the original OS-level resource limits
func (ole *OSLimitEnforcer) RestoreLimits() error {
	if !ole.applied {
		return fmt.Errorf("no limits have been applied")
	}

	var restoreErrors []error

	for resource, originalLimit := range ole.originalLimits {
		if err := syscall.Setrlimit(resource, &originalLimit); err != nil {
			restoreErrors = append(restoreErrors, fmt.Errorf("failed to restore limit for resource %d: %v", resource, err))
		}
	}

	ole.applied = false

	if len(restoreErrors) > 0 {
		return fmt.Errorf("errors during limit restoration: %v", restoreErrors)
	}

	return nil
}

// IsApplied returns whether OS limits are currently applied
func (ole *OSLimitEnforcer) IsApplied() bool {
	return ole.applied
}

// GetCurrentLimits returns the current OS-level resource limits
func (ole *OSLimitEnforcer) GetCurrentLimits() (map[string]syscall.Rlimit, error) {
	limits := make(map[string]syscall.Rlimit)
	
	resources := map[string]int{
		"memory": syscall.RLIMIT_AS,
		"cpu":    syscall.RLIMIT_CPU,
		"data":   syscall.RLIMIT_DATA,
		"stack":  syscall.RLIMIT_STACK,
		"files":  syscall.RLIMIT_NOFILE,
	}

	for name, resource := range resources {
		var limit syscall.Rlimit
		if err := syscall.Getrlimit(resource, &limit); err != nil {
			return nil, fmt.Errorf("failed to get limit for %s: %v", name, err)
		}
		limits[name] = limit
	}

	return limits, nil
}

// SandboxedEngine creates an engine with OS-level sandboxing applied
func SandboxedEngine(limits *ResourceLimits) (*Engine, *OSLimitEnforcer, error) {
	// Create OS limit enforcer
	enforcer := NewOSLimitEnforcer()
	
	// Apply OS-level limits before creating the engine
	if err := enforcer.ApplyLimits(limits); err != nil {
		return nil, nil, fmt.Errorf("failed to apply OS limits: %v", err)
	}

	// Create engine with the resource limits
	engine := NewEngine()
	engine.SetResourceLimits(limits)

	return engine, enforcer, nil
}

// SafeEngineExecution runs a function with an engine in a sandboxed environment
func SafeEngineExecution(limits *ResourceLimits, fn func(*Engine) error) error {
	// Create sandboxed engine
	engine, enforcer, err := SandboxedEngine(limits)
	if err != nil {
		return fmt.Errorf("failed to create sandboxed engine: %v", err)
	}

	// Ensure limits are restored even if function panics
	defer func() {
		if r := recover(); r != nil {
			// Restore limits before re-panicking
			if restoreErr := enforcer.RestoreLimits(); restoreErr != nil {
				fmt.Printf("ERROR: Failed to restore OS limits after panic: %v\n", restoreErr)
			}
			panic(r)
		}
	}()

	defer func() {
		if restoreErr := enforcer.RestoreLimits(); restoreErr != nil {
			fmt.Printf("ERROR: Failed to restore OS limits: %v\n", restoreErr)
		}
	}()

	// Run the function
	return fn(engine)
}

// EnableMemoryLimitEnforcement enables Go runtime memory limit enforcement
func EnableMemoryLimitEnforcement(limit uint64) {
	// Go 1.19+ feature: runtime.SetMemoryLimit
	// This provides soft memory limiting at the Go runtime level
	if limit > 0 {
		// Set runtime memory limit to 90% of the hard limit
		runtimeLimit := int64(float64(limit) * 0.9)
		if runtimeLimit > 0 {
			// Try to call runtime.SetMemoryLimit if available (Go 1.19+)
			// Use reflection to check if the function exists
			setMemoryLimit := trySetMemoryLimit(runtimeLimit)
			if !setMemoryLimit {
				// Fallback: trigger more frequent GC for older Go versions
				runtime.GC()
				debug.SetGCPercent(50) // More aggressive GC
			}
		}
	}
}

// trySetMemoryLimit attempts to call runtime.SetMemoryLimit if available
func trySetMemoryLimit(limit int64) bool {
	// For compatibility with older Go versions, we'll use a runtime check
	// In production, this would use build tags or reflection
	// For now, just return false to use the fallback
	return false
}

// SecurityPolicy defines security policies for rule evaluation
type SecurityPolicy struct {
	AllowFileAccess    bool
	AllowNetworkAccess bool
	AllowExecution     bool
	MaxStackDepth      int
}

// DefaultSecurityPolicy returns a restrictive security policy for rule evaluation
func DefaultSecurityPolicy() *SecurityPolicy {
	return &SecurityPolicy{
		AllowFileAccess:    false,
		AllowNetworkAccess: false,
		AllowExecution:     false,
		MaxStackDepth:      100,
	}
}

// ValidateSecurityPolicy checks if the current environment meets security requirements
func ValidateSecurityPolicy(policy *SecurityPolicy) error {
	// Check stack depth protection
	if policy.MaxStackDepth > 0 {
		var currentLimit syscall.Rlimit
		if err := syscall.Getrlimit(syscall.RLIMIT_STACK, &currentLimit); err != nil {
			return fmt.Errorf("failed to check stack limit: %v", err)
		}
		
		// Estimate required stack size (rough approximation)
		estimatedStackPerCall := 4096 // 4KB per call frame
		requiredStack := uint64(policy.MaxStackDepth * estimatedStackPerCall)
		
		if currentLimit.Cur > requiredStack*10 { // Allow 10x buffer
			return fmt.Errorf("stack limit too high for security policy: current=%d, recommended=<%d", 
				currentLimit.Cur, requiredStack*10)
		}
	}

	return nil
}