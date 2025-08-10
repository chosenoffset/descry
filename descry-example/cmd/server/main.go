// Package main provides the Descry example application - a financial ledger system
// that demonstrates real-world integration of Descry monitoring capabilities.
//
// This application showcases:
//   - Automatic HTTP request monitoring via middleware
//   - Custom business metric integration (account balances, transaction volumes)
//   - Rule-based monitoring with realistic thresholds
//   - Dashboard integration for real-time visualization
//   - Production-ready error handling and logging
//
// The server runs on :8080 with the following API endpoints:
//   - POST /account: Create new account with initial balance
//   - GET /balance?id=<account_id>: Get account balance
//   - POST /transfer: Transfer funds between accounts
//   - GET /descry/metrics: Current monitoring metrics
//   - GET /descry/rules: Active monitoring rules
//   - GET /descry/events: Recent rule triggers and alerts
//
// The Descry dashboard is available at http://localhost:9090
//
// Usage:
//   go run descry-example/cmd/server/main.go
//
// The server will load monitoring rules from ./rules/*.dscr files and begin
// monitoring application performance and business metrics automatically.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/chosenoffset/descry/descry-example/internal/ledger"
	"github.com/chosenoffset/descry/pkg/descry"
)

func main() {
	// Initialize Descry engine
	engine := descry.NewEngine()
	
	// Load monitoring rules from files
	if err := loadRules(engine, "./rules"); err != nil {
		log.Fatalf("Failed to load rules: %v", err)
	}
	
	// Start the Descry engine (metrics collection and rule evaluation)
	engine.Start()
	defer engine.Stop()
	
	// Initialize ledger
	l := ledger.NewLedger()
	
	// Get HTTP middleware for monitoring
	middleware := engine.HTTPMiddleware()
	
	// Set up routes with monitoring middleware
	mux := http.NewServeMux()
	mux.HandleFunc("/account", middleware(l.HandleCreateAccount))
	mux.HandleFunc("/balance", middleware(l.HandleGetBalance))
	mux.HandleFunc("/transfer", middleware(l.HandleTransfer))
	
	// Add Descry API endpoints
	mux.HandleFunc("/descry/metrics", handleDescryMetrics(engine))
	mux.HandleFunc("/descry/rules", handleDescryRules(engine))
	mux.HandleFunc("/descry/events", handleDescryEvents(engine))
	
	server := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Println("HTTP server listening on :8080")
	log.Println("Descry dashboard available at http://localhost:9090")
	log.Printf("Loaded %d monitoring rules", len(engine.GetRules()))
	
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

// loadRules loads all .dscr files from the specified directory
func loadRules(engine *descry.Engine, rulesDir string) error {
	files, err := filepath.Glob(filepath.Join(rulesDir, "*.dscr"))
	if err != nil {
		return fmt.Errorf("failed to scan rules directory: %w", err)
	}
	
	if len(files) == 0 {
		log.Println("Warning: No rule files found in", rulesDir)
		return nil
	}
	
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			log.Printf("Warning: Failed to read rule file %s: %v", file, err)
			continue
		}
		
		// Skip empty files
		if len(strings.TrimSpace(string(content))) == 0 {
			log.Printf("Warning: Skipping empty rule file %s", file)
			continue
		}
		
		ruleName := strings.TrimSuffix(filepath.Base(file), ".dscr")
		if err := engine.AddRule(ruleName, string(content)); err != nil {
			log.Printf("Warning: Failed to load rule %s: %v", ruleName, err)
			continue
		}
		
		log.Printf("Loaded rule file: %s", file)
	}
	
	return nil
}

// handleDescryMetrics exposes current metrics as JSON
func handleDescryMetrics(engine *descry.Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		
		runtimeMetrics := engine.GetRuntimeMetrics()
		httpStats := engine.GetHTTPMetrics()
		
		response := map[string]interface{}{
			"runtime": map[string]interface{}{
				"heap_alloc":    runtimeMetrics.HeapAlloc,
				"heap_sys":      runtimeMetrics.HeapSys,
				"heap_objects":  runtimeMetrics.HeapObjects,
				"goroutines":    runtimeMetrics.NumGoroutine,
				"gc_cycles":     runtimeMetrics.NumGC,
				"gc_pause_ns":   runtimeMetrics.PauseTotalNs,
			},
			"http": map[string]interface{}{
				"request_count":         httpStats.RequestCount,
				"error_count":          httpStats.ErrorCount,
				"error_rate":           httpStats.ErrorRate,
				"avg_response_time_ms": float64(httpStats.AvgResponseTime) / 1000000,
				"max_response_time_ms": float64(httpStats.MaxResponseTime) / 1000000,
				"pending_requests":     httpStats.PendingRequests,
			},
		}
		
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, "failed to encode metrics", http.StatusInternalServerError)
			log.Printf("Error encoding metrics: %v", err)
		}
	}
}

// handleDescryRules exposes active monitoring rules
func handleDescryRules(engine *descry.Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		
		rules := engine.GetRules()
		ruleData := make([]map[string]interface{}, len(rules))
		
		for i, rule := range rules {
			ruleData[i] = map[string]interface{}{
				"name":         rule.Name,
				"source":       rule.Source,
				"last_trigger": rule.LastTrigger.Format(time.RFC3339),
			}
		}
		
		response := map[string]interface{}{
			"rules": ruleData,
		}
		
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, "failed to encode rules", http.StatusInternalServerError)
			log.Printf("Error encoding rules: %v", err)
		}
	}
}

// handleDescryEvents returns recent rule triggers and alerts
func handleDescryEvents(engine *descry.Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		
		// Parse query parameters
		query := r.URL.Query()
		limit := 50 // default limit
		if limitStr := query.Get("limit"); limitStr != "" {
			if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
				limit = parsedLimit
				if limit > 500 { // max limit
					limit = 500
				}
			}
		}
		
		eventType := query.Get("type") // optional filter by type
		
		// Get event history from engine
		events := engine.GetEventHistory(limit, eventType)
		
		response := map[string]interface{}{
			"events":      events,
			"total_count": len(events),
			"limit":       limit,
		}
		
		if eventType != "" {
			response["filtered_by"] = eventType
		}
		
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, "failed to encode events", http.StatusInternalServerError)
			log.Printf("Error encoding events: %v", err)
		}
	}
}
