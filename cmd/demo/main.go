package main

import (
	"fmt"
	"log"
	"runtime"
	"time"

	"github.com/chosenoffset/descry/pkg/descry"
)

func main() {
	fmt.Println("Starting Descry Dashboard Demo...")
	
	// Create and start the engine
	engine := descry.NewEngine()
	
	// Add some sample rules
	rules := []struct {
		name   string
		source string
	}{
		{
			name:   "memory_alert",
			source: `when heap.alloc > 10MB { alert("High memory usage: ${heap.alloc}") }`,
		},
		{
			name:   "goroutine_monitor",
			source: `when goroutines.count > 100 { log("High goroutine count: ${goroutines.count}") }`,
		},
		{
			name:   "gc_frequency",
			source: `when gc.num > 5 && avg("gc.pause", 10) > 1ms { alert("Frequent GC with high pause times") }`,
		},
	}
	
	for _, rule := range rules {
		if err := engine.AddRule(rule.name, rule.source); err != nil {
			log.Printf("Error adding rule %s: %v", rule.name, err)
			continue
		}
		fmt.Printf("Added rule: %s\n", rule.name)
	}
	
	// Start the engine (this starts the dashboard too)
	engine.Start()
	defer engine.Stop()
	
	fmt.Println("Descry engine started!")
	fmt.Println("Dashboard available at: http://localhost:9090")
	fmt.Println("API endpoints:")
	fmt.Println("  - GET /api/metrics  - Current metrics")
	fmt.Println("  - GET /api/events   - Recent events")
	fmt.Println("  - GET /api/rules    - Active rules")
	fmt.Println()
	fmt.Println("Generating load to trigger rules...")
	
	// Generate some load to trigger rules
	go generateLoad()
	
	// Keep the program running
	select {}
}

func generateLoad() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	
	allocations := make([][]byte, 0)
	
	for {
		select {
		case <-ticker.C:
			// Generate memory pressure
			if len(allocations) < 100 {
				// Allocate 1MB chunks
				data := make([]byte, 1024*1024)
				allocations = append(allocations, data)
				fmt.Printf("Allocated %d MB total\n", len(allocations))
			} else {
				// Clear some allocations
				allocations = allocations[:50]
				runtime.GC()
				fmt.Println("Cleaned up memory, triggered GC")
			}
			
			// Generate goroutines
			if runtime.NumGoroutine() < 50 {
				for i := 0; i < 10; i++ {
					go func() {
						time.Sleep(5 * time.Second)
					}()
				}
			}
		}
	}
}