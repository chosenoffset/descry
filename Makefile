# Descry Monitoring System Makefile
# Provides convenient commands for building, running, and testing the application

# Configuration
SERVER_PORT ?= 8080
DASHBOARD_PORT ?= 9090

.PHONY: help build clean run-server run-fuzz run-dashboard stop test lint fmt deps dev demo

# Default target
help: ## Show this help message
	@echo "Descry Monitoring System - Available Commands:"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "Quick Start:"
	@echo "  make demo    # Start server + fuzz client + show dashboard URL"
	@echo "  make stop    # Stop all running processes"

# Build targets
build: ## Build all binaries
	@echo "Building Descry binaries..."
	@mkdir -p bin
	@go build -o bin/server descry-example/cmd/server/main.go
	@go build -o bin/fuzz descry-example/cmd/fuzz/main.go
	@echo "✅ Built binaries in ./bin/"

build-server: ## Build only the server binary
	@echo "Building server..."
	@mkdir -p bin
	@go build -o bin/server descry-example/cmd/server/main.go
	@echo "✅ Server built as ./bin/server"

build-fuzz: ## Build only the fuzz client binary
	@echo "Building fuzz client..."
	@mkdir -p bin
	@go build -o bin/fuzz descry-example/cmd/fuzz/main.go
	@echo "✅ Fuzz client built as ./bin/fuzz"

# Development targets
dev: stop build run-server ## Stop, build, and start server for development
	@sleep 2
	@echo ""
	@echo "Development server started!"
	@echo "Dashboard: http://localhost:$(DASHBOARD_PORT)"
	@echo "API: http://localhost:$(SERVER_PORT)/descry/metrics"
	@echo ""
	@echo "Run 'make run-fuzz' in another terminal to generate load"
	@echo "Run 'make stop' to shut down"

demo: stop build ## Full demo: start server + fuzz client + show URLs
	@echo "Starting Descry Demo..."
	@echo ""
	@mkdir -p logs
	@echo "Starting server..."
	@cd descry-example && ../bin/server > ../logs/server.log 2>&1 & \
	SERVER_PID=$$!; \
	echo $$SERVER_PID > ../logs/server.pid; \
	echo "Server started (PID: $$SERVER_PID)"; \
	sleep 3; \
	echo "Starting fuzz client..."; \
	../bin/fuzz > ../logs/fuzz.log 2>&1 & \
	FUZZ_PID=$$!; \
	echo $$FUZZ_PID > ../logs/fuzz.pid; \
	echo "Fuzz client started (PID: $$FUZZ_PID)"
	@echo ""
	@echo "Descry Dashboard: http://localhost:$(DASHBOARD_PORT)"
	@echo "Server API: http://localhost:$(SERVER_PORT)/descry/metrics"
	@echo "Rules API: http://localhost:$(SERVER_PORT)/descry/rules"
	@echo "Events API: http://localhost:$(SERVER_PORT)/descry/events"
	@echo "Status API: http://localhost:$(SERVER_PORT)/descry/status"
	@echo ""
	@echo "Use 'make stop' to shut down all processes"
	@echo "Use 'make logs' to view real-time logs"

# Run targets
run-server: ## Run the server (from correct directory with rules)
	@echo "Starting Descry server..."
	@mkdir -p logs
	@cd descry-example && go run cmd/server/main.go

start-server: ## Start server in background (simple version)
	@echo "Starting server in background..."
	@mkdir -p logs
	@cd descry-example && go run cmd/server/main.go > ../logs/server.log 2>&1 &
	@echo "✅ Server started - check logs/server.log"
	@echo "📊 Dashboard: http://localhost:$(DASHBOARD_PORT)"

start-fuzz: ## Start fuzz client in background (simple version)
	@echo "Starting fuzz client in background..."
	@mkdir -p logs
	@cd descry-example && go run cmd/fuzz/main.go > ../logs/fuzz.log 2>&1 &
	@echo "✅ Fuzz client started - check logs/fuzz.log"

run-fuzz: ## Run the fuzz load generator
	@echo "Starting fuzz client (load generator)..."
	@mkdir -p logs
	@cd descry-example && go run cmd/fuzz/main.go

run-server-bg: ## Run server in background
	@echo "Starting server in background..."
	@mkdir -p logs
	@cd descry-example && go run cmd/server/main.go > ../logs/server.log 2>&1 & echo $$! > ../logs/server.pid
	@echo "Server started (PID: $$(cat logs/server.pid))"

run-fuzz-bg: ## Run fuzz client in background
	@echo "Starting fuzz client in background..."
	@mkdir -p logs
	@cd descry-example && go run cmd/fuzz/main.go > ../logs/fuzz.log 2>&1 & echo $$! > ../logs/fuzz.pid
	@echo "Fuzz client started (PID: $$(cat logs/fuzz.pid))"

# Management targets
stop: ## Stop all running Descry processes
	@echo "Stopping Descry processes..."
	@-pkill -f "go run.*server" 2>/dev/null
	@-pkill -f "go run.*fuzz" 2>/dev/null  
	@-if [ -f logs/server.pid ]; then kill $$(cat logs/server.pid) 2>/dev/null; rm -f logs/server.pid; fi
	@-if [ -f logs/fuzz.pid ]; then kill $$(cat logs/fuzz.pid) 2>/dev/null; rm -f logs/fuzz.pid; fi
	@-lsof -ti:$(SERVER_PORT) 2>/dev/null | xargs kill -9 2>/dev/null
	@-lsof -ti:$(DASHBOARD_PORT) 2>/dev/null | xargs kill -9 2>/dev/null
	@echo "All processes stopped"

status: ## Show status of Descry processes and services
	@echo "📊 Descry System Status:"
	@echo ""
	@echo "Processes:"
	@pgrep -fl "go run.*server" || echo "  Server: Not running"
	@pgrep -fl "go run.*fuzz" || echo "  Fuzz client: Not running"
	@echo ""
	@echo "Port Status:"
	@lsof -ti:$(SERVER_PORT) >/dev/null && echo "  :$(SERVER_PORT) - Server API ✅" || echo "  :$(SERVER_PORT) - Server API ❌"
	@lsof -ti:$(DASHBOARD_PORT) >/dev/null && echo "  :$(DASHBOARD_PORT) - Dashboard ✅" || echo "  :$(DASHBOARD_PORT) - Dashboard ❌"
	@echo ""
	@echo "Service Health:"
	@curl -s http://localhost:$(SERVER_PORT)/descry/status >/dev/null && echo "  API Health: ✅ OK" || echo "  API Health: ❌ Down"
	@curl -s http://localhost:$(DASHBOARD_PORT)/api/metrics >/dev/null && echo "  Dashboard: ✅ OK" || echo "  Dashboard: ❌ Down"

logs: ## Show real-time logs from running processes
	@if [ -f logs/server.log ]; then echo "=== Server Logs (last 20 lines) ==="; tail -20 logs/server.log; echo ""; fi
	@if [ -f logs/fuzz.log ]; then echo "=== Fuzz Client Logs (last 10 lines) ==="; tail -10 logs/fuzz.log; echo ""; fi
	@echo "Use 'tail -f logs/server.log' for live server logs"

logs-live: ## Follow live logs from all processes
	@echo "Following live logs (Ctrl+C to stop)..."
	@if [ -f logs/server.log ] && [ -f logs/fuzz.log ]; then \
		tail -f logs/server.log logs/fuzz.log; \
	elif [ -f logs/server.log ]; then \
		tail -f logs/server.log; \
	else \
		echo "No log files found. Start services with 'make demo' first."; \
	fi

# Testing and validation
test: ## Run all tests
	@echo "Running tests..."
	@go test ./...

test-api: ## Test API endpoints
	@echo "Testing API endpoints..."
	@echo "Server metrics:"
	@curl -s http://localhost:$(SERVER_PORT)/descry/metrics | jq '.runtime.heap_alloc' || echo "❌ Server not responding"
	@echo "Dashboard metrics:"
	@curl -s http://localhost:$(DASHBOARD_PORT)/api/metrics | jq '.data.timestamp' || echo "❌ Dashboard not responding"
	@echo "Rules loaded:"
	@curl -s http://localhost:$(DASHBOARD_PORT)/api/rules | jq '.data | length' || echo "❌ Rules API not responding"

validate: ## Validate the dashboard fix is working
	@echo "Validating Dashboard Fix..."
	@echo ""
	@echo "Testing metrics API:"
	@TIMESTAMP=$$(curl -s http://localhost:$(DASHBOARD_PORT)/api/metrics | jq -r '.data.timestamp'); \
	if [ "$$TIMESTAMP" != "0001-01-01T00:00:00Z" ] && [ "$$TIMESTAMP" != "null" ]; then \
		echo "  Metrics API: Working (timestamp: $$TIMESTAMP)"; \
	else \
		echo "  Metrics API: Failed (timestamp: $$TIMESTAMP)"; \
	fi
	@echo ""
	@echo "Testing rule count:"
	@RULES=$$(curl -s http://localhost:$(DASHBOARD_PORT)/api/rules | jq '.data | length'); \
	if [ "$$RULES" -gt "0" ]; then \
		echo "  Rules loaded: $$RULES rules active"; \
	else \
		echo "  No rules loaded - check ./descry-example/rules/ directory"; \
	fi

# Development tools
lint: ## Run linter
	@echo "Running linter..."
	@golangci-lint run || echo "Install golangci-lint for linting support"

fmt: ## Format Go code
	@echo "Formatting code..."
	@go fmt ./...

vet: ## Run go vet
	@echo "Running go vet..."
	@go vet ./...

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy

# Cleanup
clean: stop ## Clean build artifacts and logs
	@echo "Cleaning up..."
	@rm -rf bin/
	@rm -rf logs/
	@go clean
	@echo "Cleanup complete"

# Quick access URLs (for convenience)
urls: ## Show important URLs
	@echo "Descry URLs:"
	@echo "  Dashboard:     http://localhost:$(DASHBOARD_PORT)"
	@echo "  Server API:    http://localhost:$(SERVER_PORT)/descry/metrics"
	@echo "  Rules:         http://localhost:$(SERVER_PORT)/descry/rules"
	@echo "  Events:        http://localhost:$(SERVER_PORT)/descry/events"
	@echo "  Status:        http://localhost:$(SERVER_PORT)/descry/status"

# Rule management
rules: ## Show loaded monitoring rules
	@echo "Active Monitoring Rules:"
	@ls -la descry-example/rules/ | grep '.dscr' || echo "No rule files found"
	@echo ""
	@echo "Rule files in ./descry-example/rules/:"
	@find descry-example/rules -name "*.dscr" -exec basename {} \; | sed 's/.dscr//' | sort

# Binary execution (using pre-built binaries)
server: build-server ## Build and run server using binary
	@echo "Running server binary..."
	@cd descry-example && ../bin/server

fuzz: build-fuzz ## Build and run fuzz client using binary
	@echo "Running fuzz binary..."
	@cd descry-example && ../bin/fuzz