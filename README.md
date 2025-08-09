# Descry

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Version](https://img.shields.io/badge/go-%3E%3D1.21-00ADD8.svg)](https://golang.org/)

**Descry** is an embeddable rules engine for Go applications that provides runtime monitoring, debugging, and observability capabilities. Define monitoring rules using a simple DSL and automatically collect Go runtime metrics, HTTP performance data, and custom application metrics.

> 🎉 **Latest Release**: Descry now includes a comprehensive web dashboard with time-travel debugging, interactive rule editor, alert management, and statistical correlation analysis!

```go
// Example rule: Monitor memory usage and alert on potential leaks
when heap.alloc > 200MB && trend(heap.alloc, 5m) > 0 {
    alert("Potential memory leak: ${heap.alloc}")
    capture_heap_profile()
}
```

## 🆕 What's New in v0.2.0

- 🚀 **Complete Web Dashboard** with 5 integrated monitoring tabs
- 🕰️ **Time-Travel Debugging** - Replay historical data at variable speeds  
- ✏️ **Interactive Rule Editor** - Visual DSL editor with live validation and testing
- 🚨 **Alert Management System** - Full lifecycle with collaborative notes and status tracking
- 📊 **Correlation Analysis** - Statistical analysis with anomaly detection and scatter plots
- 🔒 **Production Security** - Input validation, XSS prevention, and memory leak protection
- ⚡ **Performance Optimized** - O(n log n) algorithms and efficient data structures

## ✨ Features

### Core Engine
- **🔍 Zero-friction Integration**: Drop-in library with minimal setup
- **🚀 Production Ready**: Low overhead, secure sandboxed execution with thread-safe concurrent operations
- **📊 Automatic Metrics**: Collect Go runtime stats (heap, goroutines, GC) without instrumentation
- **🎯 Intuitive DSL**: Write monitoring rules in plain English-like syntax
- **⚡ Real-time Monitoring**: Continuous evaluation with configurable intervals
- **🔌 Extensible**: Plugin system for custom metrics and actions
- **🛡️ Self-contained**: No external dependencies for core functionality

### Advanced Dashboard
- **📱 Web-based Dashboard**: Modern web interface with real-time monitoring at `localhost:9090`
- **🕰️ Time-Travel Debugging**: Historical data playback with configurable speed and time ranges
- **✏️ Interactive Rule Editor**: Visual DSL editor with syntax validation and live testing
- **🚨 Alert Management**: Comprehensive alert lifecycle with acknowledgment, resolution, and notes
- **📈 Metric Correlation**: Advanced statistical analysis with anomaly detection and scatter plots
- **🔄 WebSocket Streaming**: Real-time data updates with Chart.js visualization
- **📊 Historical Analysis**: Store and analyze up to 1000 historical metric snapshots

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

## 🏗️ Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Your App      │    │   Descry Engine  │    │   Monitoring    │
│                 │    │                  │    │   Dashboard     │
│ ┌─────────────┐ │    │ ┌──────────────┐ │    │                 │
│ │   HTTP      │◄┼────┼►│  Metrics     │ │    │ ┌─────────────┐ │
│ │   Server    │ │    │ │  Collector   │ │    │ │ Real-time   │ │
│ └─────────────┘ │    │ └──────────────┘ │    │ │ Graphs      │ │
│                 │    │                  │    │ └─────────────┘ │
│ ┌─────────────┐ │    │ ┌──────────────┐ │    │                 │
│ │  Business   │◄┼────┼►│  Rules       │ │    │ ┌─────────────┐ │
│ │  Logic      │ │    │ │  Engine      │ │    │ │ Alerts &    │ │
│ └─────────────┘ │    │ └──────────────┘ │    │ │ Timeline    │ │
└─────────────────┘    └──────────────────┘    │ └─────────────┘ │
                                               └─────────────────┘
```

## 🎯 Current Capabilities

### Live Monitoring Dashboard
- **Real-time Metrics**: Memory usage, goroutine count, GC pause time with Chart.js visualizations
- **Event Timeline**: Live feed of rule triggers and alerts with timestamps
- **WebSocket Streaming**: Sub-second updates with automatic reconnection
- **Modern UI**: Responsive design with tabbed interface and interactive controls

### Time-Travel Debugging
- **Historical Playback**: Replay any time period with configurable speed (0.5x to 10x)
- **Data Storage**: Maintains 1000 historical snapshots with efficient circular buffer
- **Time Range Selection**: Custom date/time pickers or quick presets (Last Hour, Last 10 Min)
- **Synchronized Visualization**: Charts and events replay together maintaining temporal relationships

### Interactive Rule Management
- **Visual Editor**: Monaco-like editor with DSL syntax highlighting and validation
- **Live Testing**: Test rules against current metrics with immediate feedback
- **Rule Library**: Browse and load active rules with status indicators
- **Syntax Help**: Built-in DSL reference with examples and function documentation

### Advanced Alert System
- **Full Lifecycle**: Create, acknowledge, resolve, suppress alerts with user tracking
- **Severity Levels**: Critical, High, Medium, Low with color coding and filtering
- **Collaborative Notes**: Add timestamped notes with author attribution
- **Smart Categorization**: Automatic severity detection based on message content
- **Status Filtering**: Filter by status (Active, Acknowledged, Resolved, Suppressed) and severity

### Statistical Correlation Analysis
- **Metric Relationships**: Calculate Pearson correlation coefficients between any two metrics
- **Anomaly Detection**: Identify correlation changes and unusual patterns over time
- **Interactive Scatter Plots**: Visual correlation with Chart.js scatter charts
- **Quick Analysis**: Pre-configured correlation buttons for common metric pairs
- **Historical Analysis**: Configurable time windows (15min to 6 hours) and data points (50-500)

## 🎯 Use Cases

- **Memory Leak Detection**: Monitor heap growth trends and alert on abnormal patterns
- **Performance Monitoring**: Track response times, throughput, and error rates  
- **Resource Management**: Monitor goroutine counts, file descriptors, and connection pools
- **Capacity Planning**: Collect historical data for scaling decisions
- **Debugging**: Time-travel debugging with historical state reconstruction
- **SLA Monitoring**: Track service level objectives and alert on violations
- **Incident Management**: Full alert lifecycle with team collaboration features
- **Pattern Recognition**: Statistical analysis to identify metric correlations and anomalies

## 🛣️ Roadmap

### Phase 1: Core Rules Engine ✅ **COMPLETE**
- [x] DSL tokenizer and parser with comprehensive syntax support
- [x] AST evaluation engine with thread-safe execution
- [x] Automatic Go runtime metrics collection (memory, goroutines, GC)
- [x] Action system with pluggable handlers (alert, log)
- [x] Metric aggregation functions (avg, max, trend)
- [x] Production-ready concurrency safety and error handling

### Phase 2: Dashboard & Visualization ✅ **COMPLETE**
- [x] Web-based monitoring dashboard with modern UI
- [x] Real-time metrics display with Chart.js integration
- [x] WebSocket streaming for live data updates
- [x] Time-series graphs for memory, goroutines, and GC metrics
- [x] Event timeline with rule triggers and alerts

### Phase 3: Advanced Dashboard Features ✅ **COMPLETE**
- [x] **Time-Travel Debugging**: Historical data playback with configurable speed (0.5x-10x)
- [x] **Interactive Rule Editor**: Visual DSL editor with syntax validation and live testing
- [x] **Alert Management System**: Full lifecycle with acknowledge, resolve, suppress, and notes
- [x] **Metric Correlation Analysis**: Statistical correlation with anomaly detection and scatter plots
- [x] **Historical Data Storage**: Circular buffer with 1000-entry capacity for time-range analysis

### Phase 4: Example Application Integration ✅ **COMPLETE**
- [x] Financial ledger demonstration with realistic monitoring scenarios
- [x] HTTP middleware integration for automatic request/response tracking
- [x] Comprehensive rule library (memory.dscr, perf.dscr, concurrency.dscr, dev.dscr)
- [x] Enhanced load testing with 9 different patterns (sustained, spike, memory pressure)
- [x] Production-ready example showing real-world Descry integration

### Phase 5: Production Enhancements 🚧 **IN PROGRESS**
- [x] Security hardening with input validation and XSS prevention
- [x] Memory leak prevention and efficient data structures
- [x] Performance optimization with O(n log n) algorithms
- [ ] Machine learning anomaly detection
- [ ] Distributed tracing support
- [ ] CLI tools for analysis and replay
- [ ] IDE extensions and syntax highlighting

### Phase 6: Enterprise Features 📋 **PLANNED**
- [ ] Multi-application monitoring dashboard
- [ ] RBAC (Role-based access control) for rules and dashboards
- [ ] Integration with PagerDuty, Slack, and email systems
- [ ] Advanced capacity planning and resource optimization
- [ ] Compliance features with audit logging

## 🤝 Contributing

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

## 📖 Documentation

- **[Getting Started](docs/getting-started.md)** - Detailed setup and usage guide
- **[DSL Reference](docs/dsl-reference.md)** - Complete language documentation
- **[API Documentation](docs/api.md)** - Go package API reference
- **[Examples](examples/)** - Real-world integration examples

## 🔗 Related Projects

- **[Prometheus](https://prometheus.io/)** - Time series monitoring
- **[Grafana](https://grafana.com/)** - Monitoring dashboards
- **[Jaeger](https://www.jaegertracing.io/)** - Distributed tracing
- **[OpenTelemetry](https://opentelemetry.io/)** - Observability framework

## 📄 License

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

## 🙋 Support

- **Issues**: [GitHub Issues](https://github.com/chosenoffset/descry/issues)
- **Discussions**: [GitHub Discussions](https://github.com/chosenoffset/descry/discussions)
- **Email**: chosenoffset@gmail.com

---

Built with ❤️ by [Chosen Offset](https://chosenoffset.com)