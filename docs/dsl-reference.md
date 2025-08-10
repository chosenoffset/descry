# Descry DSL Reference

The Descry Domain Specific Language (DSL) is designed for writing monitoring rules that evaluate runtime metrics and trigger actions. This reference provides complete syntax documentation and examples.

## Grammar Overview

### Basic Syntax

```dscr
when <condition> {
  <action>
}
```

**Components:**
- `when` - Keyword to define a monitoring rule
- `<condition>` - Boolean expression that evaluates metrics
- `<action>` - Function call that executes when condition is true

### Rule Structure

```dscr
# Comment - Rules can have comments
when heap.alloc > 100MB && trend(heap.alloc, 5m) > 0 {
  alert("Memory usage is increasing: ${heap.alloc}")
  log("Current trend: ${trend(heap.alloc, 5m)}")
}
```

## Available Metrics

### Runtime Metrics

These metrics are automatically collected from the Go runtime:

#### Memory Metrics
- `heap.alloc` - Current bytes allocated and in use
- `heap.sys` - Total bytes obtained from the OS for heap
- `heap.idle` - Bytes in idle spans
- `heap.inuse` - Bytes in in-use spans
- `heap.released` - Bytes released to the OS

#### Garbage Collector Metrics  
- `gc.pause` - Duration of the last GC pause (nanoseconds)
- `gc.cpu_fraction` - Fraction of CPU time spent in GC since program start
- `gc.num` - Number of completed GC cycles

#### Concurrency Metrics
- `goroutines.count` - Number of active goroutines

### HTTP Metrics

Available when using Descry's HTTP middleware:

#### Request Metrics
- `http.request_count` - Total number of HTTP requests
- `http.pending_requests` - Currently active requests
- `http.response_time` - Response time of the most recent request (milliseconds)

#### Error Tracking
- `http.error_rate` - Percentage of requests returning 4xx/5xx status codes
- `http.status_2xx` - Count of 2xx responses
- `http.status_4xx` - Count of 4xx responses  
- `http.status_5xx` - Count of 5xx responses

### Custom Metrics

Application-specific metrics can be added via the API:

```go
engine.UpdateCustomMetric("orders.pending", float64(pendingOrders))
engine.UpdateCustomMetric("users.active", float64(activeUsers))
engine.UpdateCustomMetric("cache.hit_rate", float64(cacheHits)/float64(totalRequests))
```

Use in rules:
```dscr
when orders.pending > 1000 {
  alert("High pending orders: ${orders.pending}")
}
```

## Data Types

### Numbers

**Integers:**
```dscr
when goroutines.count > 100 { ... }
```

**Floats:**  
```dscr
when gc.cpu_fraction > 0.1 { ... }
when http.error_rate > 0.05 { ... }
```

### Units

Descry supports human-readable units for time and memory:

#### Memory Units
- `B` - Bytes
- `KB` - Kilobytes (1000 bytes)
- `MB` - Megabytes (1000² bytes)
- `GB` - Gigabytes (1000³ bytes)

Examples:
```dscr
when heap.alloc > 512MB { ... }
when heap.sys > 2GB { ... }
```

#### Time Units
- `ms` - Milliseconds
- `s` - Seconds  
- `m` - Minutes
- `h` - Hours

Examples:
```dscr
when avg(http.response_time, 30s) > 500ms { ... }
when trend(heap.alloc, 5m) > 10MB { ... }
```

### Strings

Used in function calls and interpolation:
```dscr
when condition {
  alert("Critical error detected")
  log("Memory usage: ${heap.alloc}")
}
```

**String Interpolation:**
Variables can be embedded in strings using `${variable}` syntax:
```dscr
alert("High memory: ${heap.alloc} with ${goroutines.count} goroutines")
```

## Operators

### Comparison Operators
- `>` - Greater than
- `>=` - Greater than or equal  
- `<` - Less than
- `<=` - Less than or equal
- `==` - Equal
- `!=` - Not equal

### Logical Operators
- `&&` - Logical AND
- `||` - Logical OR

### Arithmetic Operators
- `+` - Addition
- `-` - Subtraction
- `*` - Multiplication
- `/` - Division

### Operator Precedence

From highest to lowest:
1. `()` - Parentheses
2. `*`, `/` - Multiplication, Division
3. `+`, `-` - Addition, Subtraction  
4. `>`, `>=`, `<`, `<=`, `==`, `!=` - Comparison
5. `&&` - Logical AND
6. `||` - Logical OR

## Functions

### Statistical Functions

#### `avg(metric, duration)`
Calculates the average value of a metric over a time period.

**Parameters:**
- `metric` - Metric path as string (e.g., "http.response_time")
- `duration` - Time period (e.g., 30s, 5m, 1h)

**Returns:** Average value as float

**Examples:**
```dscr
when avg("http.response_time", 2m) > 500ms {
  alert("Average response time is slow")
}

when avg("heap.alloc", 30s) > 200MB {
  log("High average memory usage")
}
```

#### `trend(metric, duration)`
Calculates the trend (rate of change) of a metric over time.

**Parameters:**
- `metric` - Metric path as string
- `duration` - Time period for trend calculation

**Returns:** Rate of change per second

**Examples:**
```dscr
# Detect memory leaks (increasing trend)
when trend("heap.alloc", 5m) > 1MB {
  alert("Memory usage is increasing")
}

# Detect performance improvement (decreasing trend)
when trend("http.response_time", 2m) < -50ms {
  log("Response times are improving")
}
```

#### `max(metric, duration)`
Finds the maximum value of a metric over a time period.

**Parameters:**
- `metric` - Metric path as string  
- `duration` - Time period

**Returns:** Maximum value

**Examples:**
```dscr
when max("http.response_time", 1m) > 2000ms {
  alert("Very slow request detected")
}
```

### Action Functions

#### `alert(message)`
Sends an alert to configured alert handlers.

**Parameters:**
- `message` - Alert message string (supports interpolation)

**Examples:**
```dscr
when heap.alloc > 1GB {
  alert("Critical memory usage: ${heap.alloc}")
}

when http.error_rate > 0.1 {
  alert("High error rate: ${http.error_rate * 100}%")
}
```

#### `log(message)`
Writes a log message to the configured logger.

**Parameters:**
- `message` - Log message string (supports interpolation)

**Examples:**
```dscr
when goroutines.count > 50 {
  log("Goroutine count is high: ${goroutines.count}")
}
```

#### `dashboard_event(event_type, data)`
Sends an event to the dashboard for visualization.

**Parameters:**
- `event_type` - String identifying the event type
- `data` - Object with event data

**Examples:**
```dscr
when avg("http.response_time", 1m) > 300ms {
  dashboard_event("performance_alert", {
    "response_time": avg("http.response_time", 1m),
    "threshold": 300
  })
}
```

## Advanced Examples

### Memory Leak Detection

```dscr
# Basic memory leak detection
when heap.alloc > 500MB && trend("heap.alloc", 5m) > 0 {
  alert("Potential memory leak detected: ${heap.alloc}")
}

# Advanced leak detection with multiple conditions
when heap.alloc > 200MB && 
     trend("heap.alloc", 5m) > 10MB && 
     gc.cpu_fraction > 0.2 {
  alert("Memory leak with high GC pressure")
  dashboard_event("memory_leak", {
    "heap_alloc": heap.alloc,
    "trend": trend("heap.alloc", 5m),
    "gc_pressure": gc.cpu_fraction
  })
}
```

### Performance Monitoring

```dscr
# Response time monitoring
when avg("http.response_time", 2m) > 500ms && 
     http.request_count > 100 {
  alert("High latency under load: ${avg('http.response_time', 2m)}ms")
}

# Detect performance degradation
when avg("http.response_time", 1m) > avg("http.response_time", 10m) * 1.5 {
  alert("Response times degraded significantly")
}

# Traffic spike detection
when http.pending_requests > 50 &&
     trend("http.request_count", 30s) > 10 {
  log("Traffic spike detected: ${http.pending_requests} pending requests")
}
```

### Error Rate Monitoring

```dscr
# Basic error rate monitoring
when http.error_rate > 0.05 {
  alert("High error rate: ${http.error_rate * 100}%")
}

# Trending error rate
when http.error_rate > 0.01 && trend("http.error_rate", 2m) > 0.005 {
  alert("Error rate is increasing: ${http.error_rate * 100}%")
}
```

### Concurrency Issues

```dscr
# Goroutine leak detection
when goroutines.count > 1000 {
  alert("Critical goroutine count: ${goroutines.count}")
}

# Rapid goroutine growth
when goroutines.count > 100 && trend("goroutines.count", 1m) > 50 {
  alert("Rapid goroutine growth detected")
}

# Memory contention detection
when gc.cpu_fraction > 0.3 && goroutines.count > 500 {
  alert("High GC pressure with many goroutines - possible contention")
}
```

### Business Logic Monitoring

```dscr
# Order processing monitoring
when orders.pending > 1000 {
  alert("High pending orders: ${orders.pending}")
}

# User session monitoring
when users.active > max("users.active", 24h) * 1.2 {
  log("Record high user activity: ${users.active} active users")
}

# Cache performance monitoring
when cache.hit_rate < 0.8 {
  log("Cache hit rate is low: ${cache.hit_rate * 100}%")
  dashboard_event("cache_performance", {"hit_rate": cache.hit_rate})
}
```

## Best Practices

### Rule Organization

**Group related rules:**
```dscr
# Memory monitoring rules
when heap.alloc > 500MB { alert("High memory usage") }
when trend("heap.alloc", 5m) > 10MB { alert("Memory leak detected") }

# Performance monitoring rules  
when avg("http.response_time", 2m) > 500ms { alert("Slow responses") }
when http.error_rate > 0.05 { alert("High error rate") }
```

**Use descriptive rule names in comments:**
```dscr
# Rule: Critical memory usage detection
when heap.alloc > 1GB {
  alert("Critical memory usage reached")
}
```

### Threshold Selection

**Start with conservative thresholds:**
```dscr
# Start with high thresholds to avoid alert fatigue
when heap.alloc > 1GB { alert("Very high memory usage") }

# Gradually lower as you understand normal behavior
when heap.alloc > 500MB { log("Elevated memory usage") }
```

**Use trend analysis for early detection:**
```dscr
# Detect issues before they become critical
when heap.alloc > 200MB && trend("heap.alloc", 5m) > 0 {
  log("Memory usage is increasing")
}
```

### Performance Considerations

**Avoid complex calculations in conditions:**
```dscr
# Good: Simple metric comparisons
when http.response_time > 1000ms { alert("Slow request") }

# Avoid: Complex nested calculations
# when avg("http.response_time", 1m) / avg("http.response_time", 1h) > 2.0 { ... }
```

**Use appropriate time windows:**
```dscr
# Short windows for immediate issues
when http.error_rate > 0.2 { alert("Immediate high error rate") }

# Longer windows for trending analysis  
when trend("heap.alloc", 10m) > 5MB { alert("Sustained memory growth") }
```

### Alert Management

**Provide context in alert messages:**
```dscr
when heap.alloc > 500MB {
  alert("High memory usage: ${heap.alloc} (${goroutines.count} goroutines)")
}
```

**Use different action types appropriately:**
```dscr
when heap.alloc > 1GB {
  alert("CRITICAL: Memory usage is critically high")  # For urgent issues
}

when heap.alloc > 500MB {
  log("Memory usage is elevated")  # For informational purposes
}
```

## File Organization

### Single File Example

`rules/monitoring.dscr`:
```dscr
# Memory monitoring
when heap.alloc > 500MB { alert("High memory usage") }

# Performance monitoring
when avg("http.response_time", 2m) > 500ms { alert("Slow responses") }

# Concurrency monitoring
when goroutines.count > 1000 { alert("High goroutine count") }
```

### Multiple File Organization

`rules/memory.dscr`:
```dscr
when heap.alloc > 500MB { alert("High memory usage") }
when trend("heap.alloc", 5m) > 10MB { alert("Memory leak detected") }
```

`rules/performance.dscr`:
```dscr
when avg("http.response_time", 2m) > 500ms { alert("Slow responses") }
when http.error_rate > 0.05 { alert("High error rate") }
```

`rules/business.dscr`:
```dscr
when orders.pending > 1000 { alert("High pending orders") }
when cache.hit_rate < 0.8 { log("Low cache hit rate") }
```

## Integration with Go Code

### Loading Rules

```go
engine := descry.New()

// Load single file
engine.LoadRulesFromFile("rules/monitoring.dscr")

// Load multiple files
engine.LoadRulesFromFile("rules/memory.dscr")  
engine.LoadRulesFromFile("rules/performance.dscr")
engine.LoadRulesFromFile("rules/business.dscr")

// Load from string
rules := `when heap.alloc > 100MB { alert("High memory") }`
engine.LoadRulesFromString(rules)
```

### Custom Metrics Integration

```go
// Update custom metrics that can be used in rules
engine.UpdateCustomMetric("orders.pending", float64(len(pendingOrders)))
engine.UpdateCustomMetric("users.active", float64(activeUserCount))

// Rules can then reference these metrics
// when orders.pending > 100 { alert("Many pending orders") }
```

### Action Handlers

```go
// Configure alert handler
engine.SetAlertHandler(func(message string) {
    // Send to Slack, email, PagerDuty, etc.
    log.Printf("ALERT: %s", message)
})

// Configure log handler  
engine.SetLogHandler(func(message string) {
    log.Printf("DESCRY: %s", message)
})
```

This DSL provides a powerful yet simple way to define monitoring rules that can detect performance issues, resource leaks, and business logic problems in real-time.