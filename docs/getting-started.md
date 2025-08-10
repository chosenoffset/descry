# Getting Started with Descry

Descry is an embeddable rules engine for Go applications that provides runtime monitoring, debugging, and observability capabilities. This guide will help you get started with integrating Descry into your Go applications.

## Installation

### Prerequisites
- Go 1.24.5 or higher
- A Go project where you want to add monitoring

### Install Descry
```bash
go get github.com/chosenoffset/descry
```

## Basic Integration

### 1. Basic HTTP Server with Descry

Create a simple HTTP server with Descry monitoring:

```go
package main

import (
    "context"
    "log"
    "net/http"
    "time"

    "github.com/chosenoffset/descry/pkg/descry"
)

func main() {
    // Initialize Descry engine
    engine := descry.New()
    
    // Load rules from file
    if err := engine.LoadRulesFromFile("rules/basic.dscr"); err != nil {
        log.Printf("Warning: Could not load rules: %v", err)
    }
    
    // Start the engine
    ctx := context.Background()
    if err := engine.Start(ctx); err != nil {
        log.Fatalf("Failed to start Descry engine: %v", err)
    }
    defer engine.Stop()

    // Create HTTP server with Descry middleware
    mux := http.NewServeMux()
    
    // Add your application routes
    mux.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
        time.Sleep(100 * time.Millisecond) // Simulate some work
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("Hello, World!"))
    })
    
    // Add Descry monitoring endpoints
    mux.HandleFunc("/descry/metrics", engine.MetricsHandler)
    mux.HandleFunc("/descry/rules", engine.RulesHandler)
    
    // Wrap with Descry HTTP middleware for automatic monitoring
    handler := engine.HTTPMiddleware(mux)
    
    log.Println("Server starting on :8080")
    log.Println("Descry dashboard available at http://localhost:9090")
    
    if err := http.ListenAndServe(":8080", handler); err != nil {
        log.Fatalf("Server failed: %v", err)
    }
}
```

### 2. Create Your First Rule File

Create a `rules/basic.dscr` file with monitoring rules:

```dscr
# Memory monitoring
when heap.alloc > 100MB {
  log("High memory usage: ${heap.alloc}")
}

# Performance monitoring
when avg(http.response_time, 1m) > 200ms {
  alert("Slow response times detected: ${avg(http.response_time, 1m)}")
}

# Goroutine leak detection
when goroutines.count > 100 {
  log("High goroutine count: ${goroutines.count}")
}

# Error rate monitoring
when http.error_rate > 0.1 {
  alert("High error rate: ${http.error_rate * 100}%")
}
```

### 3. Run Your Application

```bash
# Make sure the rules directory exists
mkdir -p rules

# Create the basic.dscr file with the content above
# Then run your application
go run main.go
```

Your application will now:
- Monitor HTTP requests automatically
- Collect Go runtime metrics
- Execute rules every few seconds
- Provide metrics at `http://localhost:8080/descry/metrics`
- Start the dashboard at `http://localhost:9090`

## Working with the Example Application

The repository includes a complete example application that demonstrates real-world Descry usage.

### Running the Example Server

```bash
# Clone the repository
git clone https://github.com/chosenoffset/descry.git
cd descry

# Run the example server (includes a financial ledger system)
go run descry-example/cmd/server/main.go
```

The example server provides these endpoints:
- `POST /account` - Create new account
- `GET /balance?id=<account_id>` - Get account balance
- `POST /transfer` - Transfer funds between accounts
- `GET /descry/metrics` - Current metrics
- `GET /descry/rules` - Active monitoring rules

### Generating Load for Testing

In another terminal, run the fuzzing client to generate realistic load:

```bash
go run descry-example/cmd/fuzz/main.go
```

This will create natural load patterns that trigger various monitoring rules.

### Accessing the Dashboard

Visit `http://localhost:9090` to see the real-time monitoring dashboard (when implemented).

## Key Concepts

### Automatic Metrics Collection

Descry automatically collects these metrics:

**Runtime Metrics:**
- `heap.alloc` - Current heap allocation
- `heap.sys` - Total heap memory from OS
- `goroutines.count` - Number of active goroutines  
- `gc.pause` - Last GC pause duration
- `gc.cpu_fraction` - Fraction of CPU time spent in GC

**HTTP Metrics (with middleware):**
- `http.response_time` - Request response time
- `http.error_rate` - Error rate (4xx/5xx responses)
- `http.request_count` - Total request count
- `http.pending_requests` - Currently processing requests

### Custom Metrics

You can add custom business metrics:

```go
// Update a custom metric
engine.UpdateCustomMetric("orders.pending", float64(pendingOrders))
engine.UpdateCustomMetric("user.sessions", float64(activeSessions))

// Use in rules
// when orders.pending > 1000 { alert("High pending orders") }
```

### Rule Actions

Rules can trigger various actions:

```dscr
# Logging
when condition { log("message") }

# Alerts (sent to configured alert handlers)
when condition { alert("urgent message") }

# Dashboard events (for visualization)
when condition { dashboard_event("event_type", {"key": "value"}) }
```

## Configuration Options

### Engine Configuration

```go
engine := descry.New()

// Set custom update interval (default: 100ms)
engine.SetUpdateInterval(500 * time.Millisecond)

// Configure alert handlers
engine.SetAlertHandler(func(message string) {
    // Send to your alerting system
    log.Printf("ALERT: %s", message)
})

// Load multiple rule files
engine.LoadRulesFromFile("rules/memory.dscr")
engine.LoadRulesFromFile("rules/performance.dscr")
```

### Rule File Organization

Organize rules by concern:

```
rules/
├── memory.dscr      # Memory and GC monitoring
├── performance.dscr # HTTP performance rules
├── business.dscr    # Custom business logic rules
└── alerts.dscr      # Critical alerting rules
```

## Integration Patterns

### With Popular Web Frameworks

**Gin Framework:**
```go
router := gin.Default()
router.Use(gin.WrapH(engine.HTTPMiddleware(http.DefaultServeMux)))
```

**Gorilla Mux:**
```go
router := mux.NewRouter()
router.Use(func(next http.Handler) http.Handler {
    return engine.HTTPMiddleware(next)
})
```

### With Existing Monitoring

Descry complements existing monitoring solutions:

```go
// Export metrics to Prometheus
engine.SetCustomHandler("prometheus", func(metrics map[string]float64) {
    // Update Prometheus gauges
})

// Send events to external systems
engine.SetAlertHandler(func(message string) {
    // Send to Slack, PagerDuty, etc.
})
```

## Common Configuration

### Development Setup

```go
if os.Getenv("ENV") == "development" {
    // More verbose logging in development
    engine.SetLogLevel("debug")
    
    // Lower thresholds for testing
    engine.LoadRulesFromFile("rules/development.dscr")
} else {
    // Production rules
    engine.LoadRulesFromFile("rules/production.dscr")
}
```

### Production Considerations

- **Resource Usage**: Descry has minimal overhead but monitor its impact
- **Rule Complexity**: Keep rules simple for better performance
- **Update Interval**: Balance monitoring granularity with resource usage
- **Alert Fatigue**: Tune thresholds to avoid excessive alerts

## Troubleshooting

### Common Issues

**Port 9090 in use:**
```bash
# Find what's using the port
lsof -i :9090
# Kill the process or change Descry's dashboard port
```

**Rules not loading:**
- Check file path is correct relative to working directory
- Verify rule syntax with examples
- Check logs for parsing errors

**No metrics appearing:**
- Ensure HTTP middleware is properly configured
- Check that the engine is started before serving requests
- Verify rule evaluation is happening (check logs)

**Dashboard not accessible:**
- Confirm dashboard server started successfully
- Check firewall settings
- Verify port 9090 is not blocked

### Debug Mode

Enable debug logging to troubleshoot issues:

```go
engine.SetLogLevel("debug")
```

This will show:
- Rule parsing results
- Metric collection details
- Rule evaluation outcomes
- Dashboard connection status

## Next Steps

- **Learn the DSL**: Read the [DSL Reference](dsl-reference.md) for advanced rule writing
- **Explore the API**: Check the [API Reference](api.md) for integration details
- **Study Examples**: Look at `descry-example/` for real-world patterns
- **Join the Community**: Contribute to the project or ask questions

## Resources

- [DSL Reference](dsl-reference.md) - Complete DSL documentation
- [API Reference](api.md) - HTTP API and integration details
- [Architecture](architecture.md) - Technical architecture overview
- [Troubleshooting](troubleshooting.md) - Common issues and solutions
- [Contributing](../CONTRIBUTING.md) - How to contribute to the project

For more advanced usage patterns and real-world examples, explore the `descry-example/` directory in the repository.