package descry

import (
	"fmt"
	"sync"
	"time"

	"github.com/chosenoffset/descry/pkg/descry/actions"
	"github.com/chosenoffset/descry/pkg/descry/dashboard"
	"github.com/chosenoffset/descry/pkg/descry/metrics"
	"github.com/chosenoffset/descry/pkg/descry/parser"
)

type Engine struct {
	runtimeCollector *metrics.RuntimeCollector
	rules            []*Rule
	evaluator        *Evaluator
	actionRegistry   *actions.ActionRegistry
	dashboard        *dashboard.Server
	dashboardRunning bool
	running          bool
	stopCh           chan struct{}
	mutex            sync.RWMutex
}

type Rule struct {
	Name        string
	Source      string
	AST         *parser.Program
	LastTrigger time.Time
}

func NewEngine() *Engine {
	engine := &Engine{
		runtimeCollector: metrics.NewRuntimeCollector(1000, 100*time.Millisecond),
		rules:            make([]*Rule, 0),
		actionRegistry:   actions.NewActionRegistry(),
		dashboard:        dashboard.NewServer(9090),
		stopCh:           make(chan struct{}),
	}
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
	lexer := parser.NewLexer(source)
	p := parser.New(lexer)
	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		return fmt.Errorf("parse errors: %v", p.Errors())
	}

	rule := &Rule{
		Name:   name,
		Source: source,
		AST:    program,
	}

	e.rules = append(e.rules, rule)
	return nil
}

func (e *Engine) GetRuntimeMetrics() metrics.RuntimeMetrics {
	return e.runtimeCollector.GetCurrent()
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
	// Set the current rule name for action handlers
	e.evaluator.SetCurrentRuleName(rule.Name)
	
	result := e.evaluator.Eval(rule.AST)
	
	// Check if evaluation resulted in an error
	if result != nil && result.Type() == "ERROR" {
		fmt.Printf("Rule evaluation error for '%s': %s\n", rule.Name, result.Inspect())
		return
	}

	// If the rule triggered (when condition was true and executed the body), update last trigger time
	if result != nil && result.Type() == "RULE_TRIGGERED" {
		e.mutex.Lock()
		rule.LastTrigger = time.Now()
		e.mutex.Unlock()
		
		// Send event to dashboard
		e.dashboard.SendEventUpdate("rule_triggered", "Rule condition met", rule.Name, nil)
	}
}

func (e *Engine) sendMetricsToDashboard() {
	e.mutex.RLock()
	dashboardRunning := e.dashboardRunning
	e.mutex.RUnlock()
	
	if !dashboardRunning {
		return // Dashboard not available, skip sending metrics
	}
	
	metrics := e.runtimeCollector.GetCurrent()
	
	dashboardMetrics := map[string]interface{}{
		"heap.alloc":       metrics.HeapAlloc,
		"heap.sys":         metrics.HeapSys,
		"heap.idle":        metrics.HeapIdle,
		"heap.inuse":       metrics.HeapInuse,
		"heap.released":    metrics.HeapReleased,
		"heap.objects":     metrics.HeapObjects,
		"goroutines.count": metrics.NumGoroutine,
		"gc.num":           metrics.NumGC,
		"gc.pause":         metrics.PauseTotalNs,
		"gc.cpu_fraction":  metrics.GCCPUFraction,
	}
	
	e.dashboard.SendMetricUpdate(dashboardMetrics)
}

func (e *Engine) GetDashboard() *dashboard.Server {
	return e.dashboard
}