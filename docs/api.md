# Descry API Reference

This document describes the HTTP REST API and WebSocket interfaces provided by Descry for monitoring integration and dashboard connectivity.

## HTTP REST API

### Overview

Descry provides HTTP endpoints for accessing monitoring data, rule information, and system metrics. These endpoints can be used to build custom dashboards, integrate with external monitoring systems, or debug monitoring issues.

**Base URL Structure:**
- Production: `http://your-domain.com/descry/`
- Development: `http://localhost:8080/descry/` (example server)
- Dashboard: `http://localhost:9090` (dashboard server)

### Authentication

Currently, Descry API endpoints do not require authentication. This may change in future versions for production deployments.

### Response Format

All API responses use JSON format with consistent structure:

```json
{
  "data": { ... },
  "error": "error message if any",
  "timestamp": "2025-01-01T12:00:00Z"
}
```

## Monitoring Endpoints

### GET /descry/metrics

Retrieves current system and application metrics.

**Response Format:**
```json
{
  "runtime": {
    "heap_alloc": 12345678,
    "heap_sys": 23456789,
    "heap_objects": 98765,
    "goroutines": 42,
    "gc_cycles": 15,
    "gc_pause_ns": 1234567
  },
  "http": {
    "request_count": 1523,
    "error_count": 12,
    "error_rate": 0.0079,
    "avg_response_time_ms": 145.3,
    "max_response_time_ms": 2341.7,
    "pending_requests": 3
  },
  "custom": {
    "orders.pending": 156,
    "users.active": 2341,
    "cache.hit_rate": 0.92
  }
}
```

**Fields Description:**

**Runtime Metrics:**
- `heap_alloc` - Currently allocated heap memory in bytes
- `heap_sys` - Total heap memory obtained from OS in bytes
- `heap_objects` - Number of allocated heap objects
- `goroutines` - Number of active goroutines
- `gc_cycles` - Number of completed garbage collection cycles
- `gc_pause_ns` - Total GC pause time in nanoseconds

**HTTP Metrics:**
- `request_count` - Total number of HTTP requests processed
- `error_count` - Number of requests that returned 4xx or 5xx status
- `error_rate` - Percentage of requests that resulted in errors (0.0-1.0)
- `avg_response_time_ms` - Average response time in milliseconds
- `max_response_time_ms` - Maximum response time in milliseconds
- `pending_requests` - Number of currently processing requests

**Custom Metrics:**
Application-specific metrics set via `engine.UpdateCustomMetric()`

**Example Request:**
```bash
curl http://localhost:8080/descry/metrics
```

**Example Response:**
```json
{
  "runtime": {
    "heap_alloc": 15728640,
    "heap_sys": 67108864,
    "heap_objects": 123456,
    "goroutines": 8,
    "gc_cycles": 5,
    "gc_pause_ns": 789123
  },
  "http": {
    "request_count": 842,
    "error_count": 3,
    "error_rate": 0.0036,
    "avg_response_time_ms": 87.4,
    "max_response_time_ms": 1203.6,
    "pending_requests": 1
  }
}
```

### GET /descry/rules

Retrieves information about active monitoring rules.

**Response Format:**
```json
{
  "rules": [
    {
      "name": "memory-monitoring",
      "source": "when heap.alloc > 100MB { alert(\"High memory\") }",
      "last_trigger": "2025-01-01T12:34:56Z",
      "trigger_count": 15
    }
  ]
}
```

**Fields Description:**
- `name` - Rule identifier (filename without .dscr extension)
- `source` - Complete rule source code
- `last_trigger` - ISO 8601 timestamp of most recent rule execution
- `trigger_count` - Number of times this rule has been triggered

**Example Request:**
```bash
curl http://localhost:8080/descry/rules
```

### GET /descry/events

Retrieves recent rule trigger events and alerts.

**Current Status:** ⚠️ **Placeholder Implementation**

The events endpoint currently returns an empty response as event history storage is not yet implemented.

**Current Response:**
```json
{
  "events": [],
  "message": "Event history not implemented yet"
}
```

**Planned Response Format (Future Implementation):**
```json
{
  "events": [
    {
      "id": "event-12345",
      "type": "alert",
      "rule_name": "memory-monitoring",
      "message": "High memory usage: 150MB",
      "timestamp": "2025-01-01T12:34:56Z",
      "metrics_snapshot": {
        "heap.alloc": 157286400,
        "goroutines.count": 45
      }
    }
  ],
  "total_count": 1,
  "page": 1,
  "limit": 50
}
```

**Future Query Parameters:**
- `limit` - Number of events to return (default: 50, max: 500)
- `offset` - Number of events to skip for pagination
- `rule` - Filter events by rule name
- `type` - Filter by event type (alert, log, dashboard_event)
- `since` - ISO 8601 timestamp to filter events after this time

**Example Request (Future):**
```bash
curl "http://localhost:8080/descry/events?limit=10&type=alert&since=2025-01-01T10:00:00Z"
```

## HTTP Middleware Integration

### Automatic Request Monitoring

Descry provides HTTP middleware that automatically collects request metrics:

```go
import "github.com/chosenoffset/descry/pkg/descry"

func main() {
    engine := descry.New()
    
    // Get middleware
    middleware := engine.HTTPMiddleware()
    
    // Apply to specific handlers
    http.HandleFunc("/api/users", middleware(usersHandler))
    
    // Or wrap entire mux
    mux := http.NewServeMux()
    mux.HandleFunc("/api/users", usersHandler)
    wrappedMux := engine.HTTPMiddleware(mux)
}
```

**Collected Metrics:**
- Request count and error rates
- Response times (min, max, average)
- Active request tracking
- Status code distribution

### Custom Middleware Integration

**Gin Framework:**
```go
import (
    "github.com/gin-gonic/gin"
    "github.com/chosenoffset/descry/pkg/descry"
)

func main() {
    engine := descry.New()
    router := gin.Default()
    
    // Convert Descry middleware to Gin middleware
    router.Use(gin.WrapH(engine.HTTPMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Your handler logic
    }))))
}
```

**Gorilla Mux:**
```go
import (
    "github.com/gorilla/mux"
    "github.com/chosenoffset/descry/pkg/descry"
)

func main() {
    engine := descry.New()
    router := mux.NewRouter()
    
    router.Use(func(next http.Handler) http.Handler {
        return engine.HTTPMiddleware(next)
    })
}
```

## WebSocket API (Dashboard Integration)

### Dashboard Connection

The Descry dashboard connects via WebSocket to receive real-time updates.

**WebSocket Endpoint:** `ws://localhost:9090/ws`

**Connection Flow:**
1. Dashboard connects to WebSocket endpoint
2. Server sends initial metrics snapshot
3. Server streams metric updates every 100ms
4. Server sends rule trigger events as they occur

### Message Format

**Metric Updates:**
```json
{
  "type": "metrics_update",
  "timestamp": "2025-01-01T12:34:56Z",
  "data": {
    "runtime": { ... },
    "http": { ... },
    "custom": { ... }
  }
}
```

**Rule Trigger Events:**
```json
{
  "type": "rule_trigger",
  "timestamp": "2025-01-01T12:34:56Z",
  "data": {
    "rule_name": "memory-monitoring",
    "event_type": "alert",
    "message": "High memory usage detected",
    "metrics": {
      "heap.alloc": 157286400
    }
  }
}
```

**Dashboard Events:**
```json
{
  "type": "dashboard_event",
  "timestamp": "2025-01-01T12:34:56Z",
  "data": {
    "event_type": "performance_alert",
    "payload": {
      "response_time": 245.7,
      "threshold": 200.0
    }
  }
}
```

### JavaScript Client Example

```javascript
const ws = new WebSocket('ws://localhost:9090/ws');

ws.onopen = function() {
    console.log('Connected to Descry dashboard');
};

ws.onmessage = function(event) {
    const message = JSON.parse(event.data);
    
    switch (message.type) {
        case 'metrics_update':
            updateMetricsDisplay(message.data);
            break;
        case 'rule_trigger':
            displayAlert(message.data);
            break;
        case 'dashboard_event':
            handleDashboardEvent(message.data);
            break;
    }
};

function updateMetricsDisplay(metrics) {
    document.getElementById('heap-alloc').textContent = 
        formatBytes(metrics.runtime.heap_alloc);
    document.getElementById('goroutines').textContent = 
        metrics.runtime.goroutines;
    // ... update other metric displays
}
```

## Custom Metrics API

### Updating Custom Metrics

Custom business metrics can be added programmatically:

```go
engine := descry.New()

// Update custom metrics that rules can reference
engine.UpdateCustomMetric("orders.pending", float64(len(pendingOrders)))
engine.UpdateCustomMetric("users.active", float64(activeUsers))
engine.UpdateCustomMetric("cache.hit_rate", cacheHits/totalRequests)

// Metrics become available in rules
// when orders.pending > 100 { alert("High pending orders") }
```

### Metric Naming Conventions

**Category-based naming:**
- `orders.pending` - Business metrics
- `cache.hit_rate` - Infrastructure metrics
- `user.sessions` - User activity metrics

**Best Practices:**
- Use lowercase with dots for namespacing
- Keep names concise but descriptive
- Use consistent units (percentages as 0.0-1.0, bytes, counts)

## Configuration API

### Engine Configuration

```go
engine := descry.New()

// Set update interval (default: 100ms)
engine.SetUpdateInterval(500 * time.Millisecond)

// Configure alert handlers
engine.SetAlertHandler(func(message string) {
    // Send to external alerting system
    sendToSlack(message)
    sendToEmail(message)
})

// Configure log handlers
engine.SetLogHandler(func(message string) {
    log.Printf("DESCRY: %s", message)
})

// Configure dashboard event handler
engine.SetDashboardEventHandler(func(eventType string, data map[string]interface{}) {
    // Send to custom visualization system
    sendToDashboard(eventType, data)
})
```

### Rule Management

```go
// Load rules from file
err := engine.LoadRulesFromFile("rules/monitoring.dscr")

// Load rules from string
rules := `when heap.alloc > 100MB { alert("High memory") }`
err = engine.LoadRulesFromString("inline-rule", rules)

// Get active rules
rules := engine.GetRules()

// Remove rule
engine.RemoveRule("rule-name")
```

## Error Handling

### HTTP Error Responses

All endpoints return appropriate HTTP status codes:

- `200 OK` - Successful request
- `400 Bad Request` - Invalid parameters
- `404 Not Found` - Endpoint does not exist
- `405 Method Not Allowed` - Wrong HTTP method
- `500 Internal Server Error` - Server processing error

**Error Response Format:**
```json
{
  "error": "detailed error message",
  "code": "ERROR_CODE",
  "timestamp": "2025-01-01T12:34:56Z"
}
```

### Common Errors

**Missing Metrics:**
```json
{
  "error": "No metrics available - engine not started",
  "code": "ENGINE_NOT_STARTED"
}
```

**Rule Parse Errors:**
```json
{
  "error": "Failed to parse rule: syntax error at line 3",
  "code": "RULE_PARSE_ERROR",
  "details": {
    "line": 3,
    "column": 15,
    "rule_name": "memory-monitoring"
  }
}
```

## Integration Examples

### Prometheus Integration

Export Descry metrics to Prometheus:

```go
import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    heapAllocGauge = promauto.NewGauge(prometheus.GaugeOpts{
        Name: "descry_heap_alloc_bytes",
        Help: "Current heap allocation in bytes",
    })
    
    httpRequestsTotal = promauto.NewCounter(prometheus.CounterOpts{
        Name: "descry_http_requests_total", 
        Help: "Total HTTP requests processed",
    })
)

func exportToPrometheus(engine *descry.Engine) {
    metrics := engine.GetRuntimeMetrics()
    httpStats := engine.GetHTTPMetrics()
    
    heapAllocGauge.Set(float64(metrics.HeapAlloc))
    httpRequestsTotal.Add(float64(httpStats.RequestCount))
}
```

### InfluxDB Integration

Send Descry metrics to InfluxDB:

```go
import "github.com/influxdata/influxdb-client-go/v2"

func sendToInfluxDB(engine *descry.Engine, client influxdb2.Client) {
    metrics := engine.GetRuntimeMetrics()
    httpStats := engine.GetHTTPMetrics()
    
    writeAPI := client.WriteAPIBlocking("myorg", "mybucket")
    
    p := influxdb2.NewPoint("descry_metrics",
        map[string]string{"host": "server1"},
        map[string]interface{}{
            "heap_alloc": metrics.HeapAlloc,
            "goroutines": metrics.NumGoroutine,
            "http_requests": httpStats.RequestCount,
            "error_rate": httpStats.ErrorRate,
        },
        time.Now())
    
    writeAPI.WritePoint(context.Background(), p)
}
```

### Grafana Dashboard

Create Grafana dashboard using Descry metrics:

```json
{
  "dashboard": {
    "title": "Descry Monitoring",
    "panels": [
      {
        "title": "Heap Memory Usage",
        "type": "graph",
        "targets": [
          {
            "expr": "descry_heap_alloc_bytes",
            "legendFormat": "Heap Allocation"
          }
        ]
      },
      {
        "title": "HTTP Request Rate",
        "type": "graph", 
        "targets": [
          {
            "expr": "rate(descry_http_requests_total[1m])",
            "legendFormat": "Requests/sec"
          }
        ]
      }
    ]
  }
}
```

## Security Considerations

### Network Security
- API endpoints should be behind authentication in production
- Use HTTPS for all external API access
- Restrict dashboard access to authorized users
- Consider rate limiting for API endpoints

### Data Privacy
- Metrics may contain sensitive business information
- Rule source code may reveal application logic
- Implement appropriate access controls
- Consider data retention policies

### Performance Impact
- API endpoints are lightweight but consider caching for high-traffic scenarios
- WebSocket connections have minimal overhead
- Custom metrics updates are thread-safe but should be batched if frequent

## Future Enhancements

### Planned Features
- **Event History Storage**: Persistent storage for rule triggers and alerts
- **Rule Hot Reloading**: Update rules without restarting the application
- **Metric Retention Policies**: Configurable data retention and cleanup
- **Advanced Filtering**: Time-range queries and complex metric filtering
- **Batch Metric Updates**: Efficient bulk custom metric updates
- **Authentication**: Built-in authentication and authorization
- **Rate Limiting**: Configurable API rate limits

### API Versioning
Future versions will include API versioning:
- `GET /v1/descry/metrics` - Version 1 API
- `GET /v2/descry/metrics` - Version 2 API with enhanced features

Current API will be considered v1 and maintained for backward compatibility.

For the most up-to-date API documentation and examples, visit the project repository or check the built-in API documentation at `/descry/docs` (when implemented).