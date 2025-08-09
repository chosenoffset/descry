package descry

import (
	"fmt"
	"time"

	"github.com/chosenoffset/descry/pkg/descry/metrics"
	"github.com/chosenoffset/descry/pkg/descry/parser"
)

type Engine struct {
	runtimeCollector *metrics.RuntimeCollector
	rules            []*Rule
}

type Rule struct {
	Name        string
	Source      string
	AST         *parser.Program
	LastTrigger time.Time
}

func NewEngine() *Engine {
	return &Engine{
		runtimeCollector: metrics.NewRuntimeCollector(1000, 100*time.Millisecond),
		rules:            make([]*Rule, 0),
	}
}

func (e *Engine) Start() {
	e.runtimeCollector.Start()
}

func (e *Engine) Stop() {
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
	return e.rules
}