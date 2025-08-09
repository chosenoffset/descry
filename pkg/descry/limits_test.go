package descry

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chosenoffset/descry/pkg/descry/parser"
)

func TestResourceLimits(t *testing.T) {
	t.Run("MaxRulesLimit", testMaxRulesLimit)
	t.Run("MaxRuleComplexityLimit", testMaxRuleComplexityLimit)
	t.Run("MaxCustomMetricsLimit", testMaxCustomMetricsLimit)
	t.Run("EvaluationTimeoutLimit", testEvaluationTimeoutLimit)
	t.Run("DefaultLimits", testDefaultLimits)
	t.Run("CustomLimits", testCustomLimits)
}

func testMaxRulesLimit(t *testing.T) {
	engine := NewEngine()
	
	// Set a low limit for testing
	limits := engine.GetResourceLimits()
	limits.MaxRules = 3
	engine.SetResourceLimits(limits)
	
	// Add rules up to the limit
	for i := 0; i < 3; i++ {
		rule := `when heap.alloc > 0 { log("test") }`
		err := engine.AddRule(fmt.Sprintf("rule_%d", i), rule)
		if err != nil {
			t.Fatalf("Failed to add rule %d: %v", i, err)
		}
	}
	
	// Try to add one more rule - should fail
	err := engine.AddRule("excess_rule", `when heap.alloc > 0 { log("excess") }`)
	if err == nil {
		t.Error("Expected error when exceeding maximum rules limit")
	}
	
	if !strings.Contains(err.Error(), "maximum number of rules exceeded") {
		t.Errorf("Expected 'maximum number of rules exceeded' error, got: %v", err)
	}
}

func testMaxRuleComplexityLimit(t *testing.T) {
	engine := NewEngine()
	
	// Set a low complexity limit for testing
	limits := engine.GetResourceLimits()
	limits.MaxRuleComplexity = 10 // Adjusted for actual AST complexity
	engine.SetResourceLimits(limits)
	
	// Simple rule should work
	simpleRule := `when heap.alloc > 0 { log("simple") }`
	err := engine.AddRule("simple_rule", simpleRule)
	if err != nil {
		t.Fatalf("Simple rule should be accepted: %v", err)
	}
	
	// Complex rule should be rejected
	complexRule := `when heap.alloc > 100MB && goroutines.count > 100 && avg("heap.alloc", 60) > 50MB && max("goroutines.count", 30) > 200 { alert("complex") }`
	err = engine.AddRule("complex_rule", complexRule)
	if err == nil {
		t.Error("Expected error when exceeding rule complexity limit")
	}
	
	if !strings.Contains(err.Error(), "rule complexity") {
		t.Errorf("Expected 'rule complexity' error, got: %v", err)
	}
}

func testMaxCustomMetricsLimit(t *testing.T) {
	engine := NewEngine()
	
	// Set a low limit for testing
	limits := engine.GetResourceLimits()
	limits.MaxCustomMetrics = 2
	engine.SetResourceLimits(limits)
	
	// Add metrics up to the limit
	err := engine.UpdateCustomMetric("metric1", 1.0)
	if err != nil {
		t.Fatalf("Failed to add metric1: %v", err)
	}
	
	err = engine.UpdateCustomMetric("metric2", 2.0)
	if err != nil {
		t.Fatalf("Failed to add metric2: %v", err)
	}
	
	// Try to add one more metric - should fail
	err = engine.UpdateCustomMetric("metric3", 3.0)
	if err == nil {
		t.Error("Expected error when exceeding maximum custom metrics limit")
	}
	
	if !strings.Contains(err.Error(), "maximum number of custom metrics exceeded") {
		t.Errorf("Expected 'maximum number of custom metrics exceeded' error, got: %v", err)
	}
	
	// Updating existing metric should still work
	err = engine.UpdateCustomMetric("metric1", 1.5)
	if err != nil {
		t.Errorf("Updating existing metric should work: %v", err)
	}
	
	// Verify metric values
	value, exists := engine.GetCustomMetric("metric1")
	if !exists || value != 1.5 {
		t.Errorf("Expected metric1 = 1.5, got %f (exists: %t)", value, exists)
	}
}

func testEvaluationTimeoutLimit(t *testing.T) {
	engine := NewEngine()
	
	// Set very short timeout for testing
	limits := engine.GetResourceLimits()
	limits.MaxEvaluationTime = 1 * time.Millisecond
	limits.MaxCPUTime = 1 * time.Millisecond
	engine.SetResourceLimits(limits)
	
	// Add a rule that could potentially take time
	rule := `when heap.alloc > 0 { log("timeout test") }`
	err := engine.AddRule("timeout_rule", rule)
	if err != nil {
		t.Fatalf("Failed to add rule: %v", err)
	}
	
	engine.Start()
	defer engine.Stop()
	
	// Let the engine run for a bit - timeouts should be logged but not crash
	time.Sleep(100 * time.Millisecond)
	
	// Engine should still be running despite timeouts
	if !engine.IsRunning() {
		t.Error("Engine should still be running after evaluation timeouts")
	}
}

func testDefaultLimits(t *testing.T) {
	engine := NewEngine()
	limits := engine.GetResourceLimits()
	
	// Verify default limits are reasonable
	if limits.MaxRules <= 0 || limits.MaxRules > 10000 {
		t.Errorf("Default MaxRules should be reasonable, got %d", limits.MaxRules)
	}
	
	if limits.MaxRuleComplexity <= 0 || limits.MaxRuleComplexity > 100000 {
		t.Errorf("Default MaxRuleComplexity should be reasonable, got %d", limits.MaxRuleComplexity)
	}
	
	if limits.MaxMemoryUsage <= 0 || limits.MaxMemoryUsage > 1024*1024*1024 {
		t.Errorf("Default MaxMemoryUsage should be reasonable, got %d", limits.MaxMemoryUsage)
	}
	
	if limits.MaxCPUTime <= 0 || limits.MaxCPUTime > time.Second {
		t.Errorf("Default MaxCPUTime should be reasonable, got %v", limits.MaxCPUTime)
	}
	
	if limits.MaxEvaluationTime <= 0 || limits.MaxEvaluationTime > 10*time.Second {
		t.Errorf("Default MaxEvaluationTime should be reasonable, got %v", limits.MaxEvaluationTime)
	}
}

func testCustomLimits(t *testing.T) {
	engine := NewEngine()
	
	customLimits := &ResourceLimits{
		MaxRules:              50,
		MaxRuleComplexity:     500,
		MaxMemoryUsage:        50 * 1024 * 1024, // 50MB
		MaxCPUTime:            50 * time.Millisecond,
		MaxEvaluationTime:     500 * time.Millisecond,
		MaxMetricHistorySize:  5000,
		MaxCustomMetrics:      500,
	}
	
	engine.SetResourceLimits(customLimits)
	retrievedLimits := engine.GetResourceLimits()
	
	if retrievedLimits.MaxRules != customLimits.MaxRules {
		t.Errorf("Expected MaxRules %d, got %d", customLimits.MaxRules, retrievedLimits.MaxRules)
	}
	
	if retrievedLimits.MaxRuleComplexity != customLimits.MaxRuleComplexity {
		t.Errorf("Expected MaxRuleComplexity %d, got %d", customLimits.MaxRuleComplexity, retrievedLimits.MaxRuleComplexity)
	}
	
	if retrievedLimits.MaxMemoryUsage != customLimits.MaxMemoryUsage {
		t.Errorf("Expected MaxMemoryUsage %d, got %d", customLimits.MaxMemoryUsage, retrievedLimits.MaxMemoryUsage)
	}
}

func TestASTComplexityCalculation(t *testing.T) {
	testCases := []struct {
		name     string
		rule     string
		expected int // Approximate expected complexity
	}{
		{
			name:     "Simple rule",
			rule:     `when heap.alloc > 100MB { log("simple") }`,
			expected: 6, // Approximate count
		},
		{
			name:     "Medium complexity",
			rule:     `when heap.alloc > 100MB && goroutines.count > 100 { alert("medium") }`,
			expected: 10, // Approximate count
		},
		{
			name:     "High complexity with aggregation",
			rule:     `when avg("heap.alloc", 60) > 100MB && max("goroutines.count", 30) > 1000 { log("complex") }`,
			expected: 15, // Approximate count
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			lexer := parser.NewLexer(tc.rule)
			p := parser.New(lexer)
			program := p.ParseProgram()
			
			if len(p.Errors()) > 0 {
				t.Fatalf("Parse errors: %v", p.Errors())
			}
			
			complexity := countASTNodes(program)
			
			// Allow some flexibility in complexity calculation
			if complexity < tc.expected/2 || complexity > tc.expected*2 {
				t.Errorf("Expected complexity around %d, got %d for rule: %s", tc.expected, complexity, tc.rule)
			}
		})
	}
}

func TestSandboxingSecurity(t *testing.T) {
	engine := NewEngine()
	
	// Test that the engine handles potentially problematic rules safely
	// NOTE: Actual security patterns are tested in security_test.go
	problematicRules := []string{
		`when heap.alloc > 100MB { log("system information") }`,
		`when heap.alloc > 100MB { alert("external reference") }`,
		`when heap.alloc > 100MB { log("file path reference") }`,
	}
	
	for i, rule := range problematicRules {
		err := engine.AddRule(fmt.Sprintf("problematic_%d", i), rule)
		// These should parse successfully (sandboxing happens during evaluation)
		if err != nil {
			// Only fail if it's a parse error, not a security rejection
			if !strings.Contains(err.Error(), "parse errors") {
				continue // Skip if it's not a parse error
			}
			t.Errorf("Rule should parse (sandboxing happens during eval): %v", err)
		}
	}
	
	// Engine should still be functional
	validRule := `when heap.alloc > 0 { log("still working") }`
	err := engine.AddRule("valid_rule", validRule)
	if err != nil {
		t.Errorf("Engine should still work after problematic rules: %v", err)
	}
}

func TestConcurrentLimitEnforcement(t *testing.T) {
	engine := NewEngine()
	
	// Set low limits for testing
	limits := engine.GetResourceLimits()
	limits.MaxCustomMetrics = 10
	engine.SetResourceLimits(limits)
	
	// Try to add metrics concurrently
	var wg sync.WaitGroup
	errors := make(chan error, 20)
	
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			err := engine.UpdateCustomMetric(fmt.Sprintf("concurrent_%d", id), float64(id))
			if err != nil {
				errors <- err
			}
		}(i)
	}
	
	wg.Wait()
	close(errors)
	
	// Some operations should have failed due to limits
	errorCount := 0
	for err := range errors {
		if strings.Contains(err.Error(), "maximum number of custom metrics exceeded") {
			errorCount++
		}
	}
	
	if errorCount == 0 {
		t.Error("Expected some concurrent operations to fail due to limits")
	}
	
	// But some metrics should have been added successfully
	successCount := 0
	for i := 0; i < 20; i++ {
		if _, exists := engine.GetCustomMetric(fmt.Sprintf("concurrent_%d", i)); exists {
			successCount++
		}
	}
	
	if successCount == 0 {
		t.Error("Expected some concurrent operations to succeed")
	}
	
	if successCount > limits.MaxCustomMetrics {
		t.Errorf("More metrics were added (%d) than the limit allows (%d)", successCount, limits.MaxCustomMetrics)
	}
}