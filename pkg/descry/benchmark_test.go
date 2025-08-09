package descry

import (
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/chosenoffset/descry/pkg/descry/metrics"
)

// BenchmarkEngineCreation benchmarks the time it takes to create a new engine
func BenchmarkEngineCreation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		engine := NewEngine()
		_ = engine
	}
}

// BenchmarkRuleLoading benchmarks loading a single rule
func BenchmarkRuleLoading(b *testing.B) {
	engine := NewEngine()
	rule := `when heap.alloc > 100MB { alert("Memory high") }`
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := engine.LoadRule("test_rule", rule)
		if err != nil {
			b.Fatal(err)
		}
		engine.ClearRules() // Clean up for next iteration
	}
}

// BenchmarkRuleEvaluation benchmarks single rule evaluation
func BenchmarkRuleEvaluation(b *testing.B) {
	engine := NewEngine()
	rule := `when heap.alloc > 0 { log("Always true") }`
	
	err := engine.LoadRule("bench_rule", rule)
	if err != nil {
		b.Fatal(err)
	}
	
	engine.Start()
	defer engine.Stop()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.EvaluateRules()
	}
}

// BenchmarkMultipleRuleEvaluation benchmarks evaluation of multiple rules
func BenchmarkMultipleRuleEvaluation(b *testing.B) {
	engine := NewEngine()
	
	// Load multiple rules
	rules := []string{
		`when heap.alloc > 100MB { alert("Memory high") }`,
		`when goroutines.count > 1000 { alert("Too many goroutines") }`,
		`when gc.pause > 10ms { log("GC pause high") }`,
		`when heap.objects > 1000000 { log("Many objects") }`,
		`when stack.size > 10MB { alert("Stack size high") }`,
	}
	
	for i, rule := range rules {
		err := engine.LoadRule(fmt.Sprintf("rule_%d", i), rule)
		if err != nil {
			b.Fatal(err)
		}
	}
	
	engine.Start()
	defer engine.Stop()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.EvaluateRules()
	}
}

// BenchmarkConcurrentRuleEvaluation benchmarks concurrent rule evaluation
func BenchmarkConcurrentRuleEvaluation(b *testing.B) {
	engine := NewEngine()
	rule := `when heap.alloc > 0 { log("Always true") }`
	
	err := engine.LoadRule("concurrent_rule", rule)
	if err != nil {
		b.Fatal(err)
	}
	
	engine.Start()
	defer engine.Stop()
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			engine.EvaluateRules()
		}
	})
}

// BenchmarkMetricCollection benchmarks runtime metric collection
func BenchmarkMetricCollection(b *testing.B) {
	collector := metrics.NewRuntimeCollector(1000, 100*time.Millisecond)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = collector.GetCurrent()
	}
}

// BenchmarkMemoryUsage measures memory usage during rule evaluation
func BenchmarkMemoryUsage(b *testing.B) {
	engine := NewEngine()
	
	// Load complex rules with aggregations
	rules := []string{
		`when avg("heap.alloc", 60) > 100MB { alert("Avg memory high") }`,
		`when max("goroutines.count", 30) > 1000 { alert("Max goroutines high") }`,
		`when trend("gc.pause", 120) > 0 { log("GC pause trending up") }`,
	}
	
	for i, rule := range rules {
		err := engine.LoadRule(fmt.Sprintf("memory_rule_%d", i), rule)
		if err != nil {
			b.Fatal(err)
		}
	}
	
	engine.Start()
	defer engine.Stop()
	
	// Collect baseline memory
	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)
	
	b.ResetTimer()
	
	// Run evaluations and measure memory
	for i := 0; i < b.N; i++ {
		engine.EvaluateRules()
		
		// Simulate some metric history for aggregations
		if i%10 == 0 {
			engine.UpdateCustomMetric("test.metric", float64(i))
		}
	}
	
	b.StopTimer()
	
	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)
	
	// Report memory usage
	allocatedDuringBench := m2.TotalAlloc - m1.TotalAlloc
	b.ReportMetric(float64(allocatedDuringBench)/float64(b.N), "alloc-bytes/op")
}

// BenchmarkHighThroughput simulates high-throughput monitoring
func BenchmarkHighThroughput(b *testing.B) {
	engine := NewEngine()
	
	// Load production-like rules
	rules := []string{
		`when heap.alloc > 500MB { alert("Memory critical") }`,
		`when goroutines.count > 10000 { alert("Goroutine leak") }`,
		`when avg("http.response_time", 60) > 500 { alert("Slow responses") }`,
		`when max("http.error_rate", 30) > 0.05 { alert("High error rate") }`,
		`when gc.pause > 100ms { alert("GC pause too long") }`,
	}
	
	for i, rule := range rules {
		err := engine.LoadRule(fmt.Sprintf("prod_rule_%d", i), rule)
		if err != nil {
			b.Fatal(err)
		}
	}
	
	engine.Start()
	defer engine.Stop()
	
	// Simulate concurrent metric updates
	var wg sync.WaitGroup
	numGoroutines := 10
	
	b.ResetTimer()
	
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < b.N/numGoroutines; j++ {
				// Simulate HTTP requests
				engine.UpdateCustomMetric("http.response_time", float64(100+j%400))
				engine.UpdateCustomMetric("http.error_rate", float64(j%20)/1000.0)
				
				// Evaluate rules
				engine.EvaluateRules()
				
				// Small delay to simulate realistic load
				time.Sleep(time.Microsecond * 10)
			}
		}(i)
	}
	
	wg.Wait()
}

// BenchmarkRuleComplexity benchmarks rules of varying complexity
func BenchmarkRuleComplexity(b *testing.B) {
	testCases := []struct {
		name string
		rule string
	}{
		{
			name: "Simple",
			rule: `when heap.alloc > 100MB { alert("simple") }`,
		},
		{
			name: "Medium",
			rule: `when heap.alloc > 100MB && goroutines.count > 100 { alert("medium") }`,
		},
		{
			name: "Complex",
			rule: `when heap.alloc > 100MB && avg("goroutines.count", 60) > 100 && trend("gc.pause", 120) > 0 { alert("complex") }`,
		},
		{
			name: "Very Complex",
			rule: `when (heap.alloc > 100MB || max("heap.alloc", 300) > 200MB) && (avg("goroutines.count", 60) > 100 && trend("goroutines.count", 300) > 0) && gc.pause > 10ms { alert("very complex") }`,
		},
	}
	
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			engine := NewEngine()
			err := engine.LoadRule("complexity_rule", tc.rule)
			if err != nil {
				b.Fatal(err)
			}
			
			engine.Start()
			defer engine.Stop()
			
			// Pre-populate some metric history for aggregations
			for i := 0; i < 300; i++ {
				engine.UpdateCustomMetric("goroutines.count", float64(50+i))
				engine.UpdateCustomMetric("heap.alloc", float64(50*1024*1024+i*1024*1024))
				engine.UpdateCustomMetric("gc.pause", float64(5+i%20))
			}
			
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				engine.EvaluateRules()
			}
		})
	}
}