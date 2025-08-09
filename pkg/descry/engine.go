package descry

import (
	"fmt"
	"sync"
	"time"

	"github.com/chosenoffset/descry/pkg/descry/actions"
	"github.com/chosenoffset/descry/pkg/descry/metrics"
	"github.com/chosenoffset/descry/pkg/descry/parser"
)

type Engine struct {
	runtimeCollector *metrics.RuntimeCollector
	rules            []*Rule
	evaluator        *Evaluator
	actionRegistry   *actions.ActionRegistry
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
		stopCh:           make(chan struct{}),
	}
	engine.evaluator = NewEvaluator(engine)
	
	// Register default action handlers
	engine.actionRegistry.RegisterHandler(actions.AlertAction, &actions.ConsoleAlertHandler{})
	engine.actionRegistry.RegisterHandler(actions.LogAction, actions.NewLogHandler(nil))
	
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
	}
}