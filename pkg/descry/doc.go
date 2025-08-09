// Package descry provides an embeddable rules engine for Go applications that enables
// runtime monitoring, debugging, and observability with minimal performance overhead.
//
// # Overview
//
// Descry allows developers to define monitoring rules using a simple Domain Specific Language (DSL)
// and automatically collects Go runtime metrics, HTTP performance data, and custom application metrics.
// It includes a comprehensive web dashboard for real-time visualization, time-travel debugging,
// and collaborative alert management.
//
// # Quick Start
//
//	package main
//
//	import "github.com/chosenoffset/descry/pkg/descry"
//
//	func main() {
//		engine := descry.NewEngine()
//		engine.Start()
//		defer engine.Stop()
//
//		// Add a memory monitoring rule
//		engine.AddRule("memory_check", `
//			when heap.alloc > 200MB {
//				alert("High memory usage: ${heap.alloc}")
//			}
//		`)
//
//		// Access dashboard at http://localhost:9090
//		select {} // Keep running
//	}
//
// # HTTP Integration
//
// For HTTP applications, add the middleware to automatically monitor request performance:
//
//	http.Handle("/api/", engine.HTTPMiddleware()(apiHandler))
//
// # Architecture
//
// Descry consists of several integrated components:
//
//   - Engine: Main coordination and rule execution
//   - Parser: DSL lexical analysis and AST generation  
//   - Metrics: Automatic collection of runtime and HTTP metrics
//   - Actions: Pluggable handlers for rule triggers (alerts, logging)
//   - Dashboard: Web-based real-time visualization and management
//
// # DSL Reference  
//
// The Descry DSL supports condition-based rules with automatic actions:
//
//	when <condition> { <action> }
//
// Available metrics:
//   - Runtime: heap.alloc, heap.sys, goroutines.count, gc.pause, gc.cpu_fraction
//   - HTTP: http.response_time, http.request_rate, http.error_rate, http.pending_requests
//   - Custom: Any metrics you define with engine.UpdateCustomMetric()
//
// Available functions:
//   - alert(message): Trigger an alert with the given message
//   - log(message): Write a log entry  
//   - avg(metric, duration): Calculate average over time period
//   - max(metric, duration): Find maximum over time period
//   - trend(metric, duration): Calculate trend direction (+1, 0, -1)
//
// Time units: ms, s, m (milliseconds, seconds, minutes)
// Memory units: MB, GB (megabytes, gigabytes)
//
// # Dashboard Features
//
// The web dashboard at http://localhost:9090 provides:
//
//   1. Live Monitoring: Real-time charts and system health overview
//   2. Time Travel: Historical playback with configurable speed control  
//   3. Rule Editor: Interactive rule creation with syntax validation
//   4. Alert Manager: Full alert lifecycle with collaboration features
//   5. Metric Correlation: Statistical analysis and anomaly detection
//
// # Production Considerations
//
// Descry is designed for production use with built-in safety features:
//
//   - Resource Limits: Configurable limits for memory, CPU, rule complexity
//   - Sandboxed Execution: Rules cannot access filesystem or network
//   - Thread Safety: All operations are goroutine-safe
//   - Minimal Overhead: Efficient metrics collection and rule evaluation
//   - Security Hardening: Input validation and XSS prevention
//
// # Example Application
//
// See the descry-example directory for a complete integration example
// with a financial ledger application that demonstrates:
//
//   - HTTP middleware integration
//   - Custom business metric collection
//   - Realistic monitoring rules
//   - Load testing scenarios
//
// Run the example with:
//
//	go run descry-example/cmd/server/main.go
//
// Generate load with:
//
//	go run descry-example/cmd/fuzz/main.go
//
// # License
//
// Descry is licensed under the Apache License 2.0.
// See the LICENSE file for details.
package descry