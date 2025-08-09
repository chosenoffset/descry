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

## ✨ Features

- **🔍 Zero-friction Integration**: Drop-in library with minimal setup
- **🚀 Production Ready**: Low overhead, secure sandboxed execution
- **📊 Automatic Metrics**: Collect Go runtime stats (heap, goroutines, GC) without instrumentation
- **🎯 Intuitive DSL**: Write monitoring rules in plain English-like syntax
- **⚡ Real-time Monitoring**: Continuous evaluation with configurable intervals
- **🔌 Extensible**: Plugin system for custom metrics and actions
- **🛡️ Self-contained**: No external dependencies for core functionality

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
- **HTTP** *(coming soon)*: `http.response_time`, `http.request_rate`, `http.error_rate`

### Operators
- **Comparison**: `>`, `<`, `>=`, `<=`, `==`, `!=`
- **Logical**: `&&` (and), `||` (or), `!` (not)
- **Units**: `MB`, `GB`, `ms`, `s`, `m`

### Functions *(coming soon)*
- **Aggregation**: `avg(metric, duration)`, `max(metric, duration)`
- **Trend Analysis**: `trend(metric, duration)`
- **Actions**: `alert(message)`, `log(message)`

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

## 🎯 Use Cases

- **Memory Leak Detection**: Monitor heap growth trends and alert on abnormal patterns
- **Performance Monitoring**: Track response times, throughput, and error rates
- **Resource Management**: Monitor goroutine counts, file descriptors, and connection pools
- **Capacity Planning**: Collect historical data for scaling decisions
- **Debugging**: Time-travel debugging with historical state reconstruction
- **SLA Monitoring**: Track service level objectives and alert on violations

## 🛣️ Roadmap

### Phase 1: Core Engine ✅
- [x] DSL parser and tokenizer
- [x] AST evaluation engine
- [x] Go runtime metrics collection
- [x] Basic rule management

### Phase 2: Dashboard & Visualization 🚧
- [ ] Web-based monitoring dashboard
- [ ] Real-time metrics display
- [ ] Historical data playback
- [ ] Interactive rule editor

### Phase 3: Advanced Features 📋
- [ ] Machine learning anomaly detection
- [ ] Distributed tracing support
- [ ] Custom metrics API
- [ ] External system integrations

### Phase 4: Production Features 📋
- [ ] Performance profiling automation
- [ ] CLI tools for analysis and replay
- [ ] IDE extensions
- [ ] Comprehensive testing framework

## 🤝 Contributing

We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) for details.

### Development Setup

```bash
# Clone the repository
git clone https://github.com/chosenoffset/descry.git
cd descry

# Run the example application
go run descry-example/cmd/server/main.go

# Generate load for testing
go run descry-example/cmd/fuzz/main.go

# Run tests
go test ./...
```

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