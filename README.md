# Descry

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Version](https://img.shields.io/badge/go-%3E%3D1.21-00ADD8.svg)](https://golang.org/)

**Descry** is an embeddable rules engine for Go applications that provides runtime monitoring, debugging, and observability capabilities. Define monitoring rules using a simple DSL and automatically collect Go runtime metrics, HTTP performance data, and custom application metrics.

```go
// Example rule: Monitor memory usage and alert on potential leaks
when heap.alloc > 200MB && trend(heap.alloc, 5m) > 0 {
    alert("Potential memory leak: ${heap.alloc}")
    capture_heap_profile()
}
```

## Features

### Core Engine
- **Zero-friction Integration**: Drop-in library with minimal setup
- **Production Ready**: Low overhead, secure sandboxed execution with thread-safe concurrent operations
- **Automatic Metrics**: Collect Go runtime stats (heap, goroutines, GC) without instrumentation
- **Intuitive DSL**: Write monitoring rules in plain English-like syntax
- **Real-time Monitoring**: Continuous evaluation with configurable intervals
- **Extensible**: Plugin system for custom metrics and actions
- **Self-contained**: No external dependencies for core functionality

### Advanced Dashboard
- **Web-based Dashboard**: Modern web interface with real-time monitoring at `localhost:9090`
- **Time-Travel Debugging**: Historical data playback with configurable speed and time ranges
- **Interactive Rule Editor**: Visual DSL editor with syntax validation and live testing
- **Alert Management**: Comprehensive alert lifecycle with acknowledgment, resolution, and notes
- **Metric Correlation**: Advanced statistical analysis with anomaly detection and scatter plots
- **WebSocket Streaming**: Real-time data updates with Chart.js visualization
- **Historical Analysis**: Store and analyze up to 1000 historical metric snapshots

## 🚀 Quick Start

### Installation

```bash
go get github.com/chosenoffset/descry
```

### Basic Usage

```go
package main

import (
    "time"
    "github.com/chosenoffset/descry/pkg/descry"
)

func main() {
    // Create and start the monitoring engine
    engine := descry.NewEngine()
    engine.Start()
    defer engine.Stop()

    // Add monitoring rules
    err := engine.AddRule("memory_monitor", `
        when heap.alloc > 200MB {
            alert("High memory usage detected: ${heap.alloc}")
        }
    `)
    if err != nil {
        panic(err)
    }

    // Your application runs here
    // Descry monitors in the background
    time.Sleep(time.Minute)
}
```

## 📚 DSL Syntax

Descry uses an intuitive Domain-Specific Language for defining monitoring rules:

### Basic Structure
```dscr
when <condition> {
    <actions>
}
```

### Supported Metrics
- **Memory**: `heap.alloc`, `heap.sys`, `heap.objects`
- **Garbage Collection**: `gc.pause`, `gc.count`, `gc.cpu_fraction`
- **Goroutines**: `goroutines.count`
- **HTTP**: `http.response_time`, `http.request_rate` *(integrated with example application)*

### Operators
- **Comparison**: `>`, `<`, `>=`, `<=`, `==`, `!=`
- **Logical**: `&&` (and), `||` (or), `!` (not)
- **Units**: `MB`, `GB`, `ms`, `s`, `m`

### Functions
- **Aggregation**: `avg(metric, duration)`, `max(metric, duration)` ✅
- **Trend Analysis**: `trend(metric, duration)` ✅
- **Actions**: `alert(message)`, `log(message)` ✅

### Example Rules

```dscr
# Memory leak detection
when heap.alloc > 500MB && trend(heap.alloc, 5m) > 0 {
    alert("Potential memory leak detected")
    log("Heap allocation: ${heap.alloc}")
}

# Goroutine leak monitoring
when goroutines.count > 1000 {
    alert("High goroutine count: ${goroutines.count}")
}

# Performance monitoring
when avg(http.response_time, 2m) > 500ms && http.request_rate > 100/s {
    alert("Performance degradation under load")
}
```

## Contributing

We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) for details.

### Development Setup

```bash
# Clone the repository
git clone https://github.com/chosenoffset/descry.git
cd descry

# Run the complete demo (server + dashboard)
go run descry-example/cmd/server/main.go
# Dashboard available at http://localhost:9090

# In another terminal, generate realistic load
go run descry-example/cmd/fuzz/main.go

# Run tests
go test ./...
```

### Dashboard Features Demo

1. **Live Monitoring**: View real-time metrics at `http://localhost:9090`
2. **Time Travel**: Use the "Time Travel" tab to replay historical data with variable speed
3. **Rule Editor**: Create and test monitoring rules with live syntax validation
4. **Alert Manager**: Manage alert lifecycle with acknowledgment, resolution, and notes
5. **Correlation Analysis**: Analyze relationships between metrics with scatter plots and anomaly detection

## Example Application

The [`descry-example/`](descry-example/) directory contains a complete **financial ledger application** that demonstrates real-world Descry integration:

This example demonstrates how to integrate Descry into a real application with minimal overhead while gaining comprehensive observability.

## Documentation

- **[Getting Started](docs/getting-started.md)** - Detailed setup and usage guide
- **[DSL Reference](docs/dsl-reference.md)** - Complete language documentation
- **[API Documentation](docs/api.md)** - Go package API reference
- **[Complete Example Application](descry-example/)** - Full financial ledger demo with realistic monitoring scenarios

## License

Copyright 2025 Chosen Offset

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

## Support

- **Issues**: [GitHub Issues](https://github.com/chosenoffset/descry/issues)
- **Discussions**: [GitHub Discussions](https://github.com/chosenoffset/descry/discussions)
- **Email**: chosenoffset@gmail.com

---

Built by [Chosen Offset](https://chosenoffset.com)