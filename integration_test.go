package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chosenoffset/descry/pkg/descry"
	"github.com/chosenoffset/descry/pkg/descry/metrics"
)

// IntegrationTestSuite runs comprehensive integration tests
func TestIntegrationSuite(t *testing.T) {
	t.Run("EngineLifecycle", testEngineLifecycle)
	t.Run("RuleProcessing", testRuleProcessing)
	t.Run("MetricCollection", testMetricCollection)
	t.Run("DashboardAPI", testDashboardAPI)
	t.Run("ConcurrentOperations", testConcurrentOperations)
	t.Run("ErrorHandling", testErrorHandling)
	t.Run("PerformanceUnderLoad", testPerformanceUnderLoad)
}

// testEngineLifecycle tests complete engine lifecycle
func testEngineLifecycle(t *testing.T) {
	engine := descry.NewEngine()
	
	// Test initial state
	if engine.IsRunning() {
		t.Error("Engine should not be running initially")
	}
	
	// Test rule loading
	rule := `when heap.alloc > 0 { log("Engine test") }`
	err := engine.LoadRule("test_rule", rule)
	if err != nil {
		t.Fatalf("Failed to load rule: %v", err)
	}
	
	// Test engine start
	engine.Start()
	if !engine.IsRunning() {
		t.Error("Engine should be running after start")
	}
	
	// Wait for some evaluations
	time.Sleep(200 * time.Millisecond)
	
	// Test engine stop
	engine.Stop()
	if engine.IsRunning() {
		t.Error("Engine should not be running after stop")
	}
}

// testRuleProcessing tests rule loading, evaluation, and actions
func testRuleProcessing(t *testing.T) {
	engine := descry.NewEngine()
	
	// Test various rule types
	testCases := []struct {
		name string
		rule string
		shouldError bool
	}{
		{
			name: "Simple comparison",
			rule: `when heap.alloc > 0 { log("Always true") }`,
			shouldError: false,
		},
		{
			name: "Complex condition",
			rule: `when heap.alloc > 100MB && goroutines.count > 10 { alert("Complex rule") }`,
			shouldError: false,
		},
		{
			name: "With aggregation",
			rule: `when avg("heap.alloc", 60) > 50MB { log("Average rule") }`,
			shouldError: false,
		},
		{
			name: "Invalid syntax",
			rule: `when heap.alloc > { alert("Invalid") }`,
			shouldError: true,
		},
		{
			name: "Unknown metric",
			rule: `when unknown.metric > 100 { log("Unknown") }`,
			shouldError: false, // Should load but never trigger
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := engine.LoadRule(tc.name, tc.rule)
			if tc.shouldError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tc.shouldError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
	
	// Test rule evaluation
	engine.Start()
	defer engine.Stop()
	
	time.Sleep(300 * time.Millisecond) // Allow some evaluations
	
	// Test rule clearing
	engine.ClearRules()
	// Engine should still be running but with no rules
	if !engine.IsRunning() {
		t.Error("Engine should still be running after clearing rules")
	}
}

// testMetricCollection tests metric collection and aggregation
func testMetricCollection(t *testing.T) {
	engine := descry.NewEngine()
	
	// Test custom metric updates
	testMetrics := map[string]float64{
		"test.counter": 42.0,
		"test.gauge": 3.14,
		"test.histogram": 99.5,
	}
	
	for name, value := range testMetrics {
		engine.UpdateCustomMetric(name, value)
	}
	
	// Test runtime metrics collection
	collector := metrics.NewRuntimeCollector(1000, 100*time.Millisecond)
	runtimeMetrics := collector.GetCurrent()
	
	expectedMetrics := []string{
		"heap.alloc",
		"heap.sys",
		"heap.objects",
		"goroutines.count",
		"gc.pause",
		"stack.size",
	}
	
	for _, expectedMetric := range expectedMetrics {
		switch expectedMetric {
		case "heap.alloc":
			if runtimeMetrics.HeapAlloc == 0 {
				t.Errorf("Expected metric %s to have a value", expectedMetric)
			}
		case "goroutines.count":
			if runtimeMetrics.NumGoroutine == 0 {
				t.Errorf("Expected metric %s to have a value", expectedMetric)
			}
		}
	}
	
	// Test metric aggregations with rules
	rule := `when avg("test.counter", 10) > 40 { log("Average test passed") }`
	err := engine.LoadRule("aggregation_test", rule)
	if err != nil {
		t.Fatalf("Failed to load aggregation rule: %v", err)
	}
	
	engine.Start()
	defer engine.Stop()
	
	// Update metrics to build history
	for i := 0; i < 20; i++ {
		engine.UpdateCustomMetric("test.counter", float64(40+i))
		time.Sleep(10 * time.Millisecond)
	}
	
	time.Sleep(100 * time.Millisecond) // Allow evaluation
}

// testDashboardAPI tests dashboard HTTP API endpoints
func testDashboardAPI(t *testing.T) {
	engine := descry.NewEngine()
	
	// Start dashboard
	go engine.StartDashboard() // Use default port
	time.Sleep(500 * time.Millisecond) // Allow server to start
	
	baseURL := "http://localhost:9090"
	
	testCases := []struct {
		name string
		endpoint string
		method string
		expectedStatus int
	}{
		{
			name: "Dashboard root",
			endpoint: "/",
			method: "GET",
			expectedStatus: http.StatusOK,
		},
		{
			name: "Metrics API",
			endpoint: "/api/metrics",
			method: "GET",
			expectedStatus: http.StatusOK,
		},
		{
			name: "Rules API",
			endpoint: "/api/rules",
			method: "GET",
			expectedStatus: http.StatusOK,
		},
		{
			name: "Events API",
			endpoint: "/api/events",
			method: "GET",
			expectedStatus: http.StatusOK,
		},
		{
			name: "Non-existent endpoint",
			endpoint: "/api/nonexistent",
			method: "GET",
			expectedStatus: http.StatusNotFound,
		},
	}
	
	client := &http.Client{Timeout: 5 * time.Second}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			url := baseURL + tc.endpoint
			req, err := http.NewRequest(tc.method, url, nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}
			
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			defer resp.Body.Close()
			
			if resp.StatusCode != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d", tc.expectedStatus, resp.StatusCode)
			}
			
			// Validate JSON responses for API endpoints
			if strings.HasPrefix(tc.endpoint, "/api/") && resp.StatusCode == http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					t.Fatalf("Failed to read response body: %v", err)
				}
				
				var jsonData interface{}
				if err := json.Unmarshal(body, &jsonData); err != nil {
					t.Errorf("Invalid JSON response: %v", err)
				}
			}
		})
	}
}

// testConcurrentOperations tests thread safety under concurrent load
func testConcurrentOperations(t *testing.T) {
	engine := descry.NewEngine()
	
	// Load rules concurrently
	var wg sync.WaitGroup
	numGoroutines := 10
	rulesPerGoroutine := 5
	
	engine.Start()
	defer engine.Stop()
	
	// Concurrent rule loading
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < rulesPerGoroutine; j++ {
				rule := fmt.Sprintf(`when heap.alloc > %dMB { log("Concurrent rule %d-%d") }`, 
					(id*rulesPerGoroutine+j)*10, id, j)
				err := engine.LoadRule(fmt.Sprintf("concurrent_%d_%d", id, j), rule)
				if err != nil {
					t.Errorf("Failed to load concurrent rule: %v", err)
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	// Concurrent metric updates
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				engine.UpdateCustomMetric(fmt.Sprintf("concurrent.metric.%d", id), float64(j))
				time.Sleep(time.Millisecond)
			}
		}(i)
	}
	
	wg.Wait()
	
	// Allow some evaluation time
	time.Sleep(200 * time.Millisecond)
}

// testErrorHandling tests error conditions and recovery
func testErrorHandling(t *testing.T) {
	engine := descry.NewEngine()
	
	// Test malformed rules
	malformedRules := []string{
		`when { alert("Missing condition") }`,
		`when heap.alloc > { alert("Missing value") }`,
		`when heap.alloc > 100MB alert("Missing braces")`,
		`when heap.alloc > 100MB { alert() }`, // Missing message
		`when heap.alloc > 100MB { unknown_action("test") }`, // Unknown action
	}
	
	for i, rule := range malformedRules {
		err := engine.LoadRule(fmt.Sprintf("malformed_%d", i), rule)
		if err == nil {
			t.Errorf("Expected error for malformed rule: %s", rule)
		}
	}
	
	// Test that engine continues working after errors
	validRule := `when heap.alloc > 0 { log("Still working") }`
	err := engine.LoadRule("valid_after_errors", validRule)
	if err != nil {
		t.Errorf("Engine should still work after error conditions: %v", err)
	}
	
	engine.Start()
	defer engine.Stop()
	
	// Test metric update edge cases
	edgeCases := map[string]float64{
		"zero": 0.0,
		"negative": -42.0,
		"large": 1e12,
		"small": 1e-12,
	}
	
	for name, value := range edgeCases {
		engine.UpdateCustomMetric(name, value)
	}
	
	time.Sleep(100 * time.Millisecond)
}

// testPerformanceUnderLoad tests system behavior under sustained load
func testPerformanceUnderLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}
	
	engine := descry.NewEngine()
	
	// Load multiple rules
	rules := []string{
		`when heap.alloc > 100MB { log("Memory check") }`,
		`when goroutines.count > 50 { log("Goroutine check") }`,
		`when avg("custom.metric", 30) > 500 { alert("Average high") }`,
		`when max("custom.metric", 60) > 1000 { alert("Max high") }`,
		`when trend("custom.metric", 120) > 0 { log("Trending up") }`,
	}
	
	for i, rule := range rules {
		err := engine.LoadRule(fmt.Sprintf("load_rule_%d", i), rule)
		if err != nil {
			t.Fatalf("Failed to load rule: %v", err)
		}
	}
	
	engine.Start()
	defer engine.Stop()
	
	// Generate sustained load
	var wg sync.WaitGroup
	duration := 5 * time.Second
	numWorkers := 20
	
	startTime := time.Now()
	
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			counter := 0
			for time.Since(startTime) < duration {
				// Update metrics
				engine.UpdateCustomMetric("custom.metric", float64(counter%1000))
				engine.UpdateCustomMetric(fmt.Sprintf("worker.%d.counter", workerID), float64(counter))
				
				counter++
				time.Sleep(time.Millisecond * 5)
			}
		}(i)
	}
	
	wg.Wait()
	
	// Verify engine is still responsive
	testMetricValue := 42.0
	engine.UpdateCustomMetric("post_load_test", testMetricValue)
	
	// Allow final evaluation
	time.Sleep(100 * time.Millisecond)
	
	if !engine.IsRunning() {
		t.Error("Engine should still be running after load test")
	}
}

// TestSystemIntegration tests integration with example application
func TestSystemIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping system integration test in short mode")
	}
	
	// Test if example server can be built and started
	t.Run("ExampleServerBuild", func(t *testing.T) {
		cmd := exec.Command("go", "build", "-o", "/tmp/test_server", "./descry-example/cmd/server/main.go")
		cmd.Dir = "."
		
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		
		err := cmd.Run()
		if err != nil {
			t.Fatalf("Failed to build example server: %v\nStderr: %s", err, stderr.String())
		}
		
		// Clean up
		os.Remove("/tmp/test_server")
	})
	
	t.Run("FuzzClientBuild", func(t *testing.T) {
		cmd := exec.Command("go", "build", "-o", "/tmp/test_fuzz", "./descry-example/cmd/fuzz/main.go")
		cmd.Dir = "."
		
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		
		err := cmd.Run()
		if err != nil {
			t.Fatalf("Failed to build fuzz client: %v\nStderr: %s", err, stderr.String())
		}
		
		// Clean up
		os.Remove("/tmp/test_fuzz")
	})
}

// TestRuleFiles tests that rule files can be loaded and parsed
func TestRuleFiles(t *testing.T) {
	ruleFiles := []string{
		"descry-example/rules/memory.dscr",
		"descry-example/rules/perf.dscr",
		"descry-example/rules/concurrency.dscr",
		"descry-example/rules/dev.dscr",
	}
	
	engine := descry.NewEngine()
	
	for _, filename := range ruleFiles {
		t.Run(filename, func(t *testing.T) {
			content, err := os.ReadFile(filename)
			if err != nil {
				t.Fatalf("Failed to read rule file %s: %v", filename, err)
			}
			
			// Parse and load rules from file
			rules := strings.Split(string(content), "\n\n")
			for i, rule := range rules {
				rule = strings.TrimSpace(rule)
				if rule == "" || strings.HasPrefix(rule, "#") {
					continue
				}
				
				err := engine.LoadRule(fmt.Sprintf("%s_rule_%d", filename, i), rule)
				if err != nil {
					t.Errorf("Failed to load rule from %s: %v\nRule: %s", filename, err, rule)
				}
			}
		})
	}
}