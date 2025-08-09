package descry

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/chosenoffset/descry/pkg/descry/actions"
	"github.com/chosenoffset/descry/pkg/descry/dashboard"
	"github.com/chosenoffset/descry/pkg/descry/metrics"
	"github.com/chosenoffset/descry/pkg/descry/parser"
)

type Engine struct {
	runtimeCollector *metrics.RuntimeCollector
	httpMetrics      *metrics.HTTPMetrics
	rules            []*Rule
	evaluator        *Evaluator
	actionRegistry   *actions.ActionRegistry
	dashboard        *dashboard.Server
	dashboardRunning bool
	running          bool
	stopCh           chan struct{}
	mutex            sync.RWMutex
	
	// Resource limits
	limits           *ResourceLimits
	
	// Sandboxing
	customMetrics    map[string]float64
	metricsMutex     sync.RWMutex
}

type Rule struct {
	Name        string
	Source      string
	AST         *parser.Program
	LastTrigger time.Time
}

// ResourceLimits defines limits for resource usage
type ResourceLimits struct {
	MaxRules              int           // Maximum number of rules
	MaxRuleComplexity     int           // Maximum AST nodes per rule
	MaxMemoryUsage        uint64        // Maximum memory usage in bytes
	MaxCPUTime            time.Duration // Maximum CPU time per evaluation
	MaxEvaluationTime     time.Duration // Maximum wall-clock time per evaluation
	MaxMetricHistorySize  int           // Maximum number of metric history entries
	MaxCustomMetrics      int           // Maximum number of custom metrics
}

// DefaultResourceLimits returns reasonable default limits
func DefaultResourceLimits() *ResourceLimits {
	return &ResourceLimits{
		MaxRules:              100,
		MaxRuleComplexity:     1000,
		MaxMemoryUsage:        100 * 1024 * 1024, // 100MB
		MaxCPUTime:            100 * time.Millisecond,
		MaxEvaluationTime:     1 * time.Second,
		MaxMetricHistorySize:  10000,
		MaxCustomMetrics:      1000,
	}
}

func NewEngine() *Engine {
	engine := &Engine{
		runtimeCollector: metrics.NewRuntimeCollector(1000, 100*time.Millisecond),
		httpMetrics:      metrics.NewHTTPMetrics(1000),
		rules:            make([]*Rule, 0),
		actionRegistry:   actions.NewActionRegistry(),
		dashboard:        dashboard.NewServer(9090),
		stopCh:           make(chan struct{}),
		limits:           DefaultResourceLimits(),
		customMetrics:    make(map[string]float64),
	}
	
	// Enable runtime memory limit enforcement
	EnableMemoryLimitEnforcement(engine.limits.MaxMemoryUsage)
	
	engine.evaluator = NewEvaluator(engine)
	
	// Register default action handlers
	engine.actionRegistry.RegisterHandler(actions.AlertAction, &actions.ConsoleAlertHandler{})
	engine.actionRegistry.RegisterHandler(actions.LogAction, actions.NewLogHandler(nil))
	
	// Register dashboard handlers
	dashboardHandler := actions.NewDashboardHandler(engine.dashboard.SendEventUpdate)
	engine.actionRegistry.RegisterHandler(actions.AlertAction, dashboardHandler)
	engine.actionRegistry.RegisterHandler(actions.LogAction, dashboardHandler)
	
	// Set rules provider for dashboard
	engine.dashboard.SetRulesProvider(func() interface{} {
		rules := engine.GetRules()
		ruleData := make([]map[string]interface{}, len(rules))
		for i, rule := range rules {
			ruleData[i] = map[string]interface{}{
				"name":         rule.Name,
				"source":       rule.Source,
				"last_trigger": rule.LastTrigger,
			}
		}
		return ruleData
	})
	
	return engine
}

func (e *Engine) Start() {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	if e.running {
		return
	}

	e.running = true
	e.runtimeCollector.Start()
	
	// Start dashboard
	go func() {
		if err := e.dashboard.Start(); err != nil {
			fmt.Printf("Dashboard failed to start: %v\n", err)
			e.mutex.Lock()
			e.dashboardRunning = false
			e.mutex.Unlock()
			return
		}
		e.mutex.Lock()
		e.dashboardRunning = true
		e.mutex.Unlock()
	}()
	
	// Start rule evaluation loop
	go e.evaluationLoop()
}

func (e *Engine) Stop() {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	if !e.running {
		return
	}

	e.running = false
	close(e.stopCh)
	e.stopCh = make(chan struct{}) // Recreate channel for potential restart
	e.runtimeCollector.Stop()
	e.dashboard.Stop()
}

func (e *Engine) AddRule(name, source string) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	
	// Check rule count limit
	if len(e.rules) >= e.limits.MaxRules {
		return fmt.Errorf("maximum number of rules exceeded (%d)", e.limits.MaxRules)
	}
	
	lexer := parser.NewLexer(source)
	p := parser.New(lexer)
	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		return fmt.Errorf("parse errors: %v", p.Errors())
	}
	
	// Check rule complexity using efficient NodeCounter interface
	complexity := program.CountNodes()
	if complexity > e.limits.MaxRuleComplexity {
		return fmt.Errorf("rule complexity (%d nodes) exceeds limit (%d)", complexity, e.limits.MaxRuleComplexity)
	}

	rule := &Rule{
		Name:   name,
		Source: source,
		AST:    program,
	}

	e.rules = append(e.rules, rule)
	return nil
}

// LoadRule is an alias for AddRule for backward compatibility
func (e *Engine) LoadRule(name, source string) error {
	return e.AddRule(name, source)
}

// ClearRules removes all rules from the engine
func (e *Engine) ClearRules() {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.rules = make([]*Rule, 0)
}

// IsRunning returns true if the engine is currently running
func (e *Engine) IsRunning() bool {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	return e.running
}

// EvaluateRules manually triggers rule evaluation (for testing)
func (e *Engine) EvaluateRules() {
	e.evaluateRules()
}

// UpdateCustomMetric updates a custom metric value with limits checking
func (e *Engine) UpdateCustomMetric(name string, value float64) error {
	e.metricsMutex.Lock()
	defer e.metricsMutex.Unlock()
	
	// Check custom metric count limit
	if len(e.customMetrics) >= e.limits.MaxCustomMetrics {
		if _, exists := e.customMetrics[name]; !exists {
			return fmt.Errorf("maximum number of custom metrics exceeded (%d)", e.limits.MaxCustomMetrics)
		}
	}
	
	e.customMetrics[name] = value
	return nil
}

// GetCustomMetric retrieves a custom metric value
func (e *Engine) GetCustomMetric(name string) (float64, bool) {
	e.metricsMutex.RLock()
	defer e.metricsMutex.RUnlock()
	value, exists := e.customMetrics[name]
	return value, exists
}

// SetResourceLimits updates the resource limits
func (e *Engine) SetResourceLimits(limits *ResourceLimits) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.limits = limits
}

// GetResourceLimits returns the current resource limits
func (e *Engine) GetResourceLimits() *ResourceLimits {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	return e.limits
}

// Legacy countASTNodes function removed - now using efficient NodeCounter interface

// StartDashboard starts the dashboard server (uses configured port)
func (e *Engine) StartDashboard() error {
	return e.dashboard.Start()
}

func (e *Engine) GetRuntimeMetrics() metrics.RuntimeMetrics {
	return e.runtimeCollector.GetCurrent()
}

func (e *Engine) GetHTTPMetrics() metrics.HTTPStats {
	return e.httpMetrics.GetStats()
}

func (e *Engine) HTTPMiddleware() func(http.HandlerFunc) http.HandlerFunc {
	return e.httpMetrics.Middleware
}

func (e *Engine) GetRules() []*Rule {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	return e.rules
}

func (e *Engine) evaluationLoop() {
	ticker := time.NewTicker(1 * time.Second) // Evaluate rules every second
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			e.evaluateRules()
			e.sendMetricsToDashboard()
		case <-e.stopCh:
			return
		}
	}
}

func (e *Engine) evaluateRules() {
	e.mutex.RLock()
	rules := make([]*Rule, len(e.rules))
	copy(rules, e.rules)
	e.mutex.RUnlock()

	for _, rule := range rules {
		e.evaluateRule(rule)
	}
}

func (e *Engine) evaluateRule(rule *Rule) {
	// Create context with timeout for evaluation
	ctx, cancel := context.WithTimeout(context.Background(), e.limits.MaxEvaluationTime)
	defer cancel()
	
	// Create resource tracker for this evaluation
	tracker := NewResourceTracker(ctx, e.limits.MaxMemoryUsage, e.limits.MaxCPUTime)
	defer tracker.Cancel()
	
	// Channel for result communication
	type evalResult struct {
		result interface{}
		err    error
	}
	
	resultCh := make(chan evalResult, 1)
	
	// Start evaluation in goroutine with proper cleanup
	go func() {
		defer func() {
			if r := recover(); r != nil {
				resultCh <- evalResult{nil, fmt.Errorf("panic during rule evaluation: %v", r)}
			}
		}()
		
		// Set current rule name for action handlers
		e.evaluator.SetCurrentRuleName(rule.Name)
		
		// Context-aware evaluation
		result := e.evaluator.EvalWithContext(tracker.Context(), rule.AST)
		resultCh <- evalResult{result, nil}
	}()
	
	// Resource monitoring ticker
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case result := <-resultCh:
			// Evaluation completed successfully
			if result.err != nil {
				e.logError("Rule evaluation error", rule.Name, result.err, tracker)
				return
			}
			e.handleEvaluationResult(rule, result.result, tracker)
			return
			
		case <-ticker.C:
			// Periodic resource limit checking
			if err := tracker.CheckLimits(); err != nil {
				if IsResourceLimitError(err) {
					e.logResourceLimit("Rule evaluation resource limit exceeded", rule.Name, err, tracker)
				} else {
					e.logError("Rule evaluation cancelled", rule.Name, err, tracker)
				}
				return
			}
			
		case <-ctx.Done():
			// Timeout or cancellation
			e.logError("Rule evaluation timeout", rule.Name, ctx.Err(), tracker)
			return
		}
	}
}

// handleEvaluationResult processes the result of rule evaluation
func (e *Engine) handleEvaluationResult(rule *Rule, result interface{}, tracker *ResourceTracker) {
	if result == nil {
		return
	}
	
	// Type check with safe casting
	if typed, ok := result.(interface{ Type() string }); ok {
		switch typed.Type() {
		case "ERROR":
			if inspector, ok := result.(interface{ Inspect() string }); ok {
				e.logError("Rule evaluation logic error", rule.Name, 
					fmt.Errorf("rule error: %s", inspector.Inspect()), tracker)
			} else {
				e.logError("Rule evaluation logic error", rule.Name, 
					fmt.Errorf("unknown rule evaluation error"), tracker)
			}
			return
			
		case "RULE_TRIGGERED":
			e.mutex.Lock()
			rule.LastTrigger = time.Now()
			e.mutex.Unlock()
			
			// Send event to dashboard
			e.dashboard.SendEventUpdate("rule_triggered", "Rule condition met", rule.Name, nil)
			
			// Log successful trigger with resource stats
			memStats := tracker.GetMemoryStats()
			cpuStats := tracker.GetCPUStats()
			
			e.logRuleTrigger(rule.Name, memStats, cpuStats)
		}
	}
}

// logError logs evaluation errors with resource context
func (e *Engine) logError(message, ruleName string, err error, tracker *ResourceTracker) {
	memStats := tracker.GetMemoryStats()
	cpuStats := tracker.GetCPUStats()
	
	fmt.Printf("ERROR [%s] %s: %v | Memory: %.1f%% (current: %d bytes) | CPU: %v/%v (%.1f%% efficiency)\n",
		ruleName, message, err,
		memStats.BudgetUsed, memStats.CurrentAlloc,
		cpuStats.CPUTimeUsed, cpuStats.MaxCPUTime, cpuStats.CPUEfficiency)
}

// logResourceLimit logs resource limit violations
func (e *Engine) logResourceLimit(message, ruleName string, err error, tracker *ResourceTracker) {
	memStats := tracker.GetMemoryStats()
	cpuStats := tracker.GetCPUStats()
	
	fmt.Printf("LIMIT [%s] %s: %v | Memory: %.1f%% budget used | CPU: %v used of %v allowed\n",
		ruleName, message, err,
		memStats.BudgetUsed,
		cpuStats.CPUTimeUsed, cpuStats.MaxCPUTime)
}

// logRuleTrigger logs successful rule triggers with performance metrics
func (e *Engine) logRuleTrigger(ruleName string, memStats MemoryStats, cpuStats CPUStats) {
	fmt.Printf("TRIGGER [%s] Rule condition met | Memory: %.1f%% budget | CPU: %v (%.1f%% efficiency)\n",
		ruleName, memStats.BudgetUsed, cpuStats.CPUTimeUsed, cpuStats.CPUEfficiency)
}

func (e *Engine) sendMetricsToDashboard() {
	e.mutex.RLock()
	dashboardRunning := e.dashboardRunning
	e.mutex.RUnlock()
	
	if !dashboardRunning {
		return // Dashboard not available, skip sending metrics
	}
	
	runtimeMetrics := e.runtimeCollector.GetCurrent()
	httpStats := e.httpMetrics.GetStats()
	
	dashboardMetrics := map[string]interface{}{
		// Runtime metrics
		"heap.alloc":       runtimeMetrics.HeapAlloc,
		"heap.sys":         runtimeMetrics.HeapSys,
		"heap.idle":        runtimeMetrics.HeapIdle,
		"heap.inuse":       runtimeMetrics.HeapInuse,
		"heap.released":    runtimeMetrics.HeapReleased,
		"heap.objects":     runtimeMetrics.HeapObjects,
		"goroutines.count": runtimeMetrics.NumGoroutine,
		"gc.num":           runtimeMetrics.NumGC,
		"gc.pause":         runtimeMetrics.PauseTotalNs,
		"gc.cpu_fraction":  runtimeMetrics.GCCPUFraction,
		// HTTP metrics
		"http.request_count":    httpStats.RequestCount,
		"http.error_count":      httpStats.ErrorCount,
		"http.error_rate":       httpStats.ErrorRate,
		"http.request_rate":     httpStats.RequestRate,
		"http.response_time":    httpStats.AvgResponseTime,
		"http.max_response_time": httpStats.MaxResponseTime,
		"http.pending_requests": httpStats.PendingRequests,
	}
	
	e.dashboard.SendMetricUpdate(dashboardMetrics)
}

func (e *Engine) GetDashboard() *dashboard.Server {
	return e.dashboard
}