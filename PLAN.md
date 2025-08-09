# Descry Development Plan

## Project Vision

Descry is an embeddable rules engine for Go applications that provides runtime monitoring, debugging, and observability capabilities. It allows developers to define monitoring rules using a simple DSL and automatically collects Go runtime metrics, HTTP performance data, and custom application metrics.

### Key Goals
- **Zero-friction Integration**: Drop-in library with minimal setup
- **Production Ready**: Low overhead, secure sandboxed execution  
- **Developer-Friendly**: Intuitive DSL, visual debugging, time-travel debugging
- **Extensible**: Plugin system for custom metrics and actions
- **Self-Contained**: No external dependencies for core functionality

## Development Phases

### Phase 1: Core Rules Engine Library ⏳

**Objective**: Build the foundational rules engine with automatic metric collection

#### Core Components
- [ ] **Engine Architecture** (`/pkg/descry/engine.go`)
  - [ ] DSL tokenizer and parser
  - [ ] AST evaluation engine
  - [ ] Thread-safe rule execution
  - [ ] State management system

- [ ] **Automatic Metric Collection** (`/pkg/descry/metrics/`)
  - [ ] Go runtime metrics (heap, goroutines, GC stats)
  - [ ] HTTP middleware for request/response monitoring
  - [ ] Stack trace capture on rule triggers
  - [ ] Custom metrics API for applications

- [ ] **Action System** (`/pkg/descry/actions/`)
  - [ ] Alert handlers (log, console, custom)
  - [ ] Event recording for dashboard
  - [ ] Metric export (JSON logs)
  - [ ] Stack trace and heap profile capture

- [ ] **DSL Language Features**
  - [ ] Basic operators: `>`, `<`, `==`, `&&`, `||`
  - [ ] Metric access: `heap.alloc`, `goroutines.count`, `gc.pause`
  - [ ] Functions: `avg()`, `max()`, `trend()`, `alert()`, `log()`
  - [ ] String interpolation: `"Memory: ${heap.alloc}"`

**Deliverables**:
- Core library with basic DSL support
- Automatic Go runtime metric collection
- Simple rule evaluation and action execution
- JSON structured logging for external tools

### Phase 2: Visualization & Dashboard System ⏳

**Objective**: Build web-based monitoring and playback system

#### Dashboard Components
- [ ] **Web Interface** (`/pkg/descry/dashboard/`)
  - [ ] Real-time metrics display
  - [ ] Time-series graphs (memory, goroutines, response times)
  - [ ] Event timeline with rule triggers
  - [ ] System health overview

- [ ] **Interactive Features**
  - [ ] Live rule editor with syntax highlighting
  - [ ] Rule validation and testing
  - [ ] Alert management (view, acknowledge, dismiss)
  - [ ] Metric correlation views

- [ ] **Playback System** (`/pkg/descry/storage/`)
  - [ ] Event recorder with timestamped snapshots
  - [ ] Time-travel debugging interface
  - [ ] Historical data scrubbing
  - [ ] State reconstruction at any point in time

- [ ] **Data Storage**
  - [ ] In-memory circular buffer for recent events
  - [ ] Configurable retention policies
  - [ ] Export to external systems (Prometheus, InfluxDB)

**Deliverables**:
- Web dashboard accessible at `localhost:9090`
- Real-time monitoring with historical playback
- Interactive rule editing and management
- Event correlation and debugging tools

### Phase 3: Enhanced Example Application ⏳

**Objective**: Demonstrate Descry capabilities with realistic monitoring scenarios

#### Integration Enhancements
- [ ] **Server Integration** (`descry-example/cmd/server/main.go`)
  - [ ] Load rules from `.dscr` files at startup
  - [ ] Initialize Descry metrics collection
  - [ ] Add HTTP middleware for automatic monitoring
  - [ ] Expose Descry API endpoints

- [ ] **Natural Error Generation**
  - [ ] Memory pressure scenarios (large data structures)
  - [ ] Concurrency stress (goroutine leaks via high load)
  - [ ] Performance degradation (CPU-bound operations)
  - [ ] Resource exhaustion (connection pool limits)

- [ ] **Realistic Rule Examples** (`descry-example/rules/`)
  - [ ] `memory.dscr`: Memory leak and pressure detection
  - [ ] `perf.dscr`: Response time and throughput monitoring
  - [ ] `concurrency.dscr`: Goroutine leak detection
  - [ ] `errors.dscr`: Error rate and pattern monitoring

- [ ] **Enhanced Fuzzing** (`descry-example/cmd/fuzz/main.go`)
  - [ ] Variable load patterns (bursts, sustained load)
  - [ ] Invalid request generation (malformed JSON, large payloads)
  - [ ] Concurrent operation scenarios
  - [ ] Edge case testing (negative balances, non-existent accounts)

**Deliverables**:
- Fully integrated example application with realistic monitoring
- Comprehensive rule library covering common scenarios
- Load testing that demonstrates natural error conditions
- Clear documentation of integration patterns

### Phase 4: Advanced Features & Developer Experience ⏳

**Objective**: Production-ready features and developer tools

#### Advanced Monitoring
- [ ] **Machine Learning Features**
  - [ ] Anomaly detection for metric patterns
  - [ ] Baseline establishment for normal behavior
  - [ ] Predictive alerting based on trends
  - [ ] False positive reduction

- [ ] **Distributed Tracing**
  - [ ] Request correlation across goroutines
  - [ ] Cross-service call tracking (when integrated)
  - [ ] Performance bottleneck identification
  - [ ] Call graph visualization

- [ ] **Advanced Actions**
  - [ ] Circuit breaker integration
  - [ ] Automatic scaling triggers
  - [ ] Performance profiling automation
  - [ ] Custom webhook notifications

#### Developer Tools
- [ ] **CLI Tools** (`/cmd/descry/`)
  - [ ] `descry analyze`: Historical data analysis
  - [ ] `descry replay`: Time-range playback tool
  - [ ] `descry validate`: Rule syntax validation
  - [ ] `descry export`: Data export utilities

- [ ] **IDE Integration**
  - [ ] VS Code extension for rule editing
  - [ ] Syntax highlighting for `.dscr` files
  - [ ] Rule debugging and testing
  - [ ] Live metric display in editor

- [ ] **Testing Framework**
  - [ ] Unit testing for rules against historical data
  - [ ] Rule performance benchmarking
  - [ ] Integration testing helpers
  - [ ] Mock metric generators

**Deliverables**:
- Production-ready library with ML-based anomaly detection
- Complete CLI toolkit for rule management
- IDE integration for improved developer experience
- Comprehensive testing and validation framework

## Technical Architecture

### Core Library Structure
```
/pkg/descry/
├── engine.go          # Main API and engine coordination
├── parser/            # DSL parsing and AST generation
│   ├── lexer.go      # Token generation
│   ├── parser.go     # AST construction
│   └── ast.go        # AST node definitions
├── metrics/           # Metric collection systems
│   ├── runtime.go    # Go runtime metrics
│   ├── http.go       # HTTP middleware
│   └── custom.go     # Custom metric API
├── actions/           # Action handler implementations
│   ├── alert.go      # Alert actions
│   ├── log.go        # Logging actions
│   └── dashboard.go  # Dashboard events
├── dashboard/         # Web interface components
│   ├── server.go     # HTTP server
│   ├── static/       # Web assets
│   └── api/          # REST API handlers
└── storage/           # Event storage and replay
    ├── memory.go     # In-memory storage
    ├── export.go     # External system exports
    └── replay.go     # Playback functionality
```

### Integration Points
1. **Application Startup**: Initialize Descry, load rules, start collection
2. **HTTP Middleware**: Automatic request/response monitoring
3. **Periodic Collection**: Background goroutine feeding metrics to engine
4. **Rule Evaluation**: Continuous evaluation with configurable intervals
5. **Action Execution**: Pluggable system for handling rule triggers

### Security Considerations
- **Sandboxed Execution**: Rules cannot access filesystem or network
- **Resource Limits**: Memory and CPU limits for rule evaluation
- **Input Validation**: Strict parsing and validation of rule syntax
- **Minimal Privileges**: Library runs with application permissions only

## Session Progress Tracking

### Session 1: Project Planning ✅
- [x] Define project vision and goals
- [x] Update CLAUDE.md with new architecture
- [x] Create comprehensive PLAN.md
- [x] Establish development phases

### Session 2: Core Engine Foundation ✅
- [x] Set up `/pkg/descry/` directory structure  
- [x] Implement basic DSL tokenizer (lexer.go)
- [x] Create AST node definitions (ast.go)
- [x] Build simple rule parser (parser.go)
- [x] Add Go runtime metric collection (runtime.go)
- [x] Create basic engine integration (engine.go)
- [x] Add Apache 2.0 license and comprehensive README
- [x] Successfully parse and validate DSL syntax: `when heap.alloc > 200MB { alert("Memory leak") }`

**Session 2 Achievements**: Built complete foundation for Descry rules engine with working tokenizer, parser, AST, and runtime metrics collection. The engine can now parse complex DSL rules and automatically collect Go runtime statistics. Ready for rule evaluation implementation.

### Session 3: Rule Evaluation Engine ✅
- [x] Implement AST evaluator
- [x] Add thread-safe state management  
- [x] Create action handler system
- [x] Build metric aggregation functions (avg, max, trend)
- [x] **Critical fixes**: Race conditions, resource leaks, division by zero, thread safety

**Session 3 Achievements**: Completed production-ready rule evaluation engine with thread-safe concurrent evaluation, pluggable action handlers (alert, log), and metric aggregation functions (avg, max, trend). Fixed critical race conditions and resource leaks identified by code review. The engine can now execute complex DSL rules like `when heap.alloc > 200MB && trend("heap.alloc", 300) > 0 { alert("Memory leak") }`.

### Session 4: Basic Dashboard
- [ ] Create web server for dashboard
- [ ] Implement real-time metric display
- [ ] Add basic time-series graphing
- [ ] Create rule trigger timeline

### Session 5: Example Application Integration
- [ ] Integrate Descry into ledger server
- [ ] Create sample rule files
- [ ] Add HTTP middleware monitoring
- [ ] Update fuzzing client for realistic load

### Session 6: Advanced Dashboard Features
- [ ] Add playback/time-travel functionality
- [ ] Implement rule editor interface
- [ ] Create alert management system
- [ ] Add metric correlation views

### Session 7: Production Readiness
- [ ] Add comprehensive error handling
- [ ] Implement resource limits and sandboxing
- [ ] Create performance benchmarks
- [ ] Write integration tests

### Session 8: Documentation & Polish
- [ ] Complete API documentation
- [ ] Create getting-started guide
- [ ] Add example rule library
- [ ] Final testing and bug fixes

## Future Extensions

### Community Contributions
- **Rule Library**: Community-contributed monitoring rules
- **Custom Actions**: Plugin system for external integrations
- **Metric Sources**: Connectors for databases, message queues, etc.
- **Export Formats**: Additional time-series database support

### Enterprise Features
- **Multi-Application**: Monitor multiple services from single dashboard
- **RBAC**: Role-based access control for rules and dashboards  
- **Alerting**: Integration with PagerDuty, Slack, email systems
- **Compliance**: Audit logging and regulatory compliance features

### Advanced Analytics
- **ML Models**: Custom model training for application-specific patterns
- **Capacity Planning**: Resource usage prediction and recommendations
- **Cost Optimization**: Cloud resource usage optimization suggestions
- **Performance Recommendations**: Automated performance tuning advice

---

*This plan serves as a living document that will be updated as development progresses across multiple sessions.*