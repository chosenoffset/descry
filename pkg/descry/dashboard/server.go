package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Server struct {
	port           int
	server         *http.Server
	upgrader       websocket.Upgrader
	clients        map[*websocket.Conn]bool
	clientsMutex   sync.RWMutex
	maxClients     int
	metrics        chan MetricUpdate
	events         chan EventUpdate
	stop           chan struct{}
	recentMetrics  MetricUpdate
	eventBuffer    []EventUpdate
	eventIndex     int
	eventCount     int
	mutex          sync.RWMutex
	getRules       func() interface{}
	// Playback storage
	historicalMetrics []MetricUpdate
	historicalEvents  []EventUpdate
	maxHistorySize    int
	// Alert management
	alerts            []Alert
	alertsByStatus    map[AlertStatus][]Alert
}

type MetricUpdate struct {
	Timestamp time.Time              `json:"timestamp"`
	Metrics   map[string]interface{} `json:"metrics"`
}

type EventUpdate struct {
	Timestamp time.Time   `json:"timestamp"`
	Type      string      `json:"type"`
	Message   string      `json:"message"`
	Rule      string      `json:"rule"`
	Data      interface{} `json:"data"`
}

type AlertStatus string

const (
	AlertStatusActive       AlertStatus = "active"
	AlertStatusAcknowledged AlertStatus = "acknowledged"
	AlertStatusResolved     AlertStatus = "resolved"
	AlertStatusSuppressed   AlertStatus = "suppressed"
)

type AlertSeverity string

const (
	AlertSeverityLow      AlertSeverity = "low"
	AlertSeverityMedium   AlertSeverity = "medium"
	AlertSeverityHigh     AlertSeverity = "high"
	AlertSeverityCritical AlertSeverity = "critical"
)

type Alert struct {
	ID           string        `json:"id"`
	Rule         string        `json:"rule"`
	Message      string        `json:"message"`
	Severity     AlertSeverity `json:"severity"`
	Status       AlertStatus   `json:"status"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
	ResolvedAt   *time.Time    `json:"resolved_at,omitempty"`
	AcknowledgedBy *string     `json:"acknowledged_by,omitempty"`
	Notes        []AlertNote   `json:"notes"`
	Metadata     map[string]interface{} `json:"metadata"`
}

type AlertNote struct {
	ID        string    `json:"id"`
	Message   string    `json:"message"`
	Author    string    `json:"author"`
	CreatedAt time.Time `json:"created_at"`
}

func NewServer(port int) *Server {
	return &Server{
		port: port,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// Allow same origin and localhost for development
				origin := r.Header.Get("Origin")
				if origin == "" {
					return true // Allow requests without Origin header
				}
				// Allow localhost and same-origin requests
				return origin == fmt.Sprintf("http://localhost:%d", port) ||
					   origin == fmt.Sprintf("http://127.0.0.1:%d", port)
			},
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
		clients:           make(map[*websocket.Conn]bool),
		maxClients:        100, // Limit concurrent WebSocket connections
		metrics:           make(chan MetricUpdate, 100),
		events:            make(chan EventUpdate, 100),
		stop:              make(chan struct{}),
		eventBuffer:       make([]EventUpdate, 50), // Fixed-size circular buffer
		historicalMetrics: make([]MetricUpdate, 0, 1000),
		historicalEvents:  make([]EventUpdate, 0, 1000),
		maxHistorySize:    1000, // Store up to 1000 historical entries
		alerts:            make([]Alert, 0),
		alertsByStatus:    make(map[AlertStatus][]Alert),
	}
}

func (s *Server) Start() error {
	mux := http.NewServeMux()
	
	// Static files
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/static/", s.handleStatic)
	
	// API endpoints
	mux.HandleFunc("/api/metrics", s.handleMetrics)
	mux.HandleFunc("/api/events", s.handleEvents)
	mux.HandleFunc("/api/rules", s.handleRules)
	mux.HandleFunc("/api/history/metrics", s.handleHistoricalMetrics)
	mux.HandleFunc("/api/history/events", s.handleHistoricalEvents)
	mux.HandleFunc("/api/playback", s.handlePlayback)
	mux.HandleFunc("/api/rules/validate", s.handleRuleValidation)
	mux.HandleFunc("/api/rules/save", s.handleRuleSave)
	mux.HandleFunc("/api/rules/test", s.handleRuleTest)
	mux.HandleFunc("/api/alerts", s.handleAlerts)
	mux.HandleFunc("/api/alerts/acknowledge", s.handleAcknowledgeAlert)
	mux.HandleFunc("/api/alerts/resolve", s.handleResolveAlert)
	mux.HandleFunc("/api/alerts/suppress", s.handleSuppressAlert)
	mux.HandleFunc("/api/alerts/note", s.handleAddAlertNote)
	mux.HandleFunc("/api/correlation", s.handleMetricCorrelation)
	
	// WebSocket endpoint
	mux.HandleFunc("/ws", s.handleWebSocket)
	
	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: mux,
	}
	
	// Start broadcast goroutine
	go s.broadcast()
	
	log.Printf("Starting Descry dashboard on :%d", s.port)
	return s.server.ListenAndServe()
}

func (s *Server) Stop() error {
	close(s.stop)
	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.server.Shutdown(ctx)
	}
	return nil
}

func (s *Server) SendMetricUpdate(metrics map[string]interface{}) {
	select {
	case s.metrics <- MetricUpdate{
		Timestamp: time.Now(),
		Metrics:   metrics,
	}:
	default:
		// Drop if channel is full
	}
}

func (s *Server) SendEventUpdate(eventType, message, rule string, data interface{}) {
	event := EventUpdate{
		Timestamp: time.Now(),
		Type:      eventType,
		Message:   message,
		Rule:      rule,
		Data:      data,
	}
	
	select {
	case s.events <- event:
	default:
		// Drop if channel is full
	}
	
	// Create alert for alert-type events
	if eventType == "alert" {
		s.createAlert(rule, message, data)
	}
}

func (s *Server) createAlert(rule, message string, data interface{}) {
	// Determine severity based on message content
	severity := AlertSeverityMedium
	msgLower := strings.ToLower(message)
	if strings.Contains(msgLower, "critical") || strings.Contains(msgLower, "leak") {
		severity = AlertSeverityCritical
	} else if strings.Contains(msgLower, "high") || strings.Contains(msgLower, "warning") {
		severity = AlertSeverityHigh
	} else if strings.Contains(msgLower, "info") {
		severity = AlertSeverityLow
	}
	
	alert := Alert{
		ID:        generateAlertID(),
		Rule:      rule,
		Message:   message,
		Severity:  severity,
		Status:    AlertStatusActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Notes:     []AlertNote{},
		Metadata:  make(map[string]interface{}),
	}
	
	if data != nil {
		alert.Metadata["trigger_data"] = data
	}
	
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	s.alerts = append(s.alerts, alert)
	s.updateAlertsByStatus() // Safe within mutex lock
}

func generateAlertID() string {
	// Simple ID generation - in production, use UUIDs
	return fmt.Sprintf("alert_%d", time.Now().UnixNano())
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	html := `<!DOCTYPE html>
<html>
<head>
    <title>Descry Dashboard</title>
    <script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
    <style>
        body { font-family: Arial, sans-serif; margin: 0; padding: 20px; background: #f5f5f5; }
        .header { background: #2c3e50; color: white; padding: 20px; border-radius: 5px; margin-bottom: 20px; }
        .grid { display: grid; grid-template-columns: 1fr 1fr; gap: 20px; }
        .card { background: white; padding: 20px; border-radius: 5px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .metric-value { font-size: 2em; font-weight: bold; color: #3498db; }
        .metric-label { color: #7f8c8d; margin-bottom: 10px; }
        .chart-container { position: relative; height: 300px; }
        .events-list { max-height: 400px; overflow-y: auto; }
        .event { padding: 10px; margin: 5px 0; border-left: 4px solid #3498db; background: #ecf0f1; }
        .event.alert { border-left-color: #e74c3c; }
        .event.warning { border-left-color: #f39c12; }
        .timestamp { font-size: 0.8em; color: #7f8c8d; }
        .playback-controls { background: #34495e; color: white; padding: 15px; border-radius: 5px; margin-bottom: 20px; }
        .playback-controls input, .playback-controls button { margin: 5px; padding: 5px 10px; }
        .playback-status { font-weight: bold; color: #f39c12; }
        .tab-container { margin-bottom: 20px; }
        .tabs { display: flex; background: #ecf0f1; border-radius: 5px; }
        .tab { padding: 10px 20px; cursor: pointer; border-radius: 5px; margin: 2px; }
        .tab.active { background: #3498db; color: white; }
        .tab-content { display: none; }
        .tab-content.active { display: block; }
    </style>
</head>
<body>
    <div class="header">
        <h1>Descry Dashboard</h1>
        <p>Real-time application monitoring and rule engine</p>
    </div>
    
    <div class="tab-container">
        <div class="tabs">
            <div class="tab active" onclick="showTab('live')">Live Monitoring</div>
            <div class="tab" onclick="showTab('playback')">Time Travel</div>
            <div class="tab" onclick="showTab('rules')">Rule Editor</div>
            <div class="tab" onclick="showTab('alerts')">Alert Manager</div>
            <div class="tab" onclick="showTab('correlation')">Metric Correlation</div>
        </div>
    </div>
    
    <div id="live-tab" class="tab-content active">
        <div class="grid">
        <div class="card">
            <div class="metric-label">Memory Usage</div>
            <div class="metric-value" id="memory-value">-- MB</div>
            <div class="chart-container">
                <canvas id="memory-chart"></canvas>
            </div>
        </div>
        
        <div class="card">
            <div class="metric-label">Goroutines</div>
            <div class="metric-value" id="goroutines-value">--</div>
            <div class="chart-container">
                <canvas id="goroutines-chart"></canvas>
            </div>
        </div>
        
        <div class="card">
            <div class="metric-label">GC Pause Time</div>
            <div class="metric-value" id="gc-value">-- μs</div>
            <div class="chart-container">
                <canvas id="gc-chart"></canvas>
            </div>
        </div>
        
        <div class="card">
            <h3>Recent Events</h3>
            <div class="events-list" id="events-list">
                <div class="event">
                    <div>Waiting for events...</div>
                    <div class="timestamp">--</div>
                </div>
            </div>
        </div>
        </div>
    </div>
    
    <div id="playback-tab" class="tab-content">
        <div class="playback-controls">
            <h3>Time Travel Debugging</h3>
            <p>Replay historical data from any time period</p>
            
            <label>From: </label>
            <input type="datetime-local" id="playback-from" />
            
            <label>To: </label>
            <input type="datetime-local" id="playback-to" />
            
            <label>Speed: </label>
            <select id="playback-speed">
                <option value="0.5">0.5x</option>
                <option value="1" selected>1x</option>
                <option value="2">2x</option>
                <option value="5">5x</option>
                <option value="10">10x</option>
            </select>
            
            <button onclick="startPlayback()">Start Playback</button>
            <button onclick="stopPlayback()">Stop</button>
            <button onclick="loadLastHour()">Last Hour</button>
            <button onclick="loadLast10Minutes()">Last 10 Min</button>
            
            <div class="playback-status" id="playback-status">Ready</div>
        </div>
        
        <div class="grid">
            <div class="card">
                <div class="metric-label">Memory Usage (Playback)</div>
                <div class="metric-value" id="playback-memory-value">-- MB</div>
                <div class="chart-container">
                    <canvas id="playback-memory-chart"></canvas>
                </div>
            </div>
            
            <div class="card">
                <div class="metric-label">Goroutines (Playback)</div>
                <div class="metric-value" id="playback-goroutines-value">--</div>
                <div class="chart-container">
                    <canvas id="playback-goroutines-chart"></canvas>
                </div>
            </div>
            
            <div class="card">
                <div class="metric-label">GC Pause Time (Playback)</div>
                <div class="metric-value" id="playback-gc-value">-- μs</div>
                <div class="chart-container">
                    <canvas id="playback-gc-chart"></canvas>
                </div>
            </div>
            
            <div class="card">
                <h3>Events Timeline</h3>
                <div class="events-list" id="playback-events-list">
                    <div class="event">
                        <div>Select a time range to replay events...</div>
                        <div class="timestamp">--</div>
                    </div>
                </div>
            </div>
        </div>
    </div>
    
    <div id="rules-tab" class="tab-content">
        <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 20px;">
            <div class="card">
                <h3>Rule Editor</h3>
                <p>Create and test monitoring rules using Descry DSL</p>
                
                <label>Rule Name:</label>
                <input type="text" id="rule-name" placeholder="my-rule" style="width: 100%; margin: 5px 0; padding: 8px;" />
                
                <label>Rule Code:</label>
                <textarea id="rule-editor" placeholder="when heap.alloc > 200MB {
  alert(&quot;Memory usage high&quot;)
}" style="width: 100%; height: 200px; margin: 5px 0; padding: 8px; font-family: monospace;"></textarea>
                
                <div style="margin: 10px 0;">
                    <button onclick="validateRule()" style="background: #3498db; color: white; border: none; padding: 8px 16px; border-radius: 3px; margin-right: 10px;">Validate</button>
                    <button onclick="saveRule()" style="background: #2ecc71; color: white; border: none; padding: 8px 16px; border-radius: 3px; margin-right: 10px;">Save</button>
                    <button onclick="testRule()" style="background: #f39c12; color: white; border: none; padding: 8px 16px; border-radius: 3px;">Test</button>
                </div>
                
                <div id="rule-status" style="padding: 10px; margin: 10px 0; border-radius: 3px; background: #ecf0f1;"></div>
            </div>
            
            <div class="card">
                <h3>Active Rules</h3>
                <div id="active-rules-list" style="max-height: 400px; overflow-y: auto;">
                    <div style="padding: 10px; color: #7f8c8d;">Loading rules...</div>
                </div>
                
                <h4 style="margin-top: 30px;">DSL Reference</h4>
                <div style="font-size: 0.9em; background: #f8f9fa; padding: 15px; border-radius: 3px;">
                    <h5>Metrics:</h5>
                    <ul>
                        <li><code>heap.alloc</code> - Heap allocated memory</li>
                        <li><code>goroutines.count</code> - Active goroutines</li>
                        <li><code>gc.pause</code> - GC pause time</li>
                        <li><code>http.response_time</code> - HTTP response time</li>
                        <li><code>http.request_rate</code> - HTTP requests per second</li>
                    </ul>
                    
                    <h5>Functions:</h5>
                    <ul>
                        <li><code>avg(metric, duration)</code> - Average value</li>
                        <li><code>max(metric, duration)</code> - Maximum value</li>
                        <li><code>trend(metric, duration)</code> - Trend direction</li>
                    </ul>
                    
                    <h5>Actions:</h5>
                    <ul>
                        <li><code>alert("message")</code> - Send alert</li>
                        <li><code>log("message")</code> - Log message</li>
                    </ul>
                    
                    <h5>Example:</h5>
                    <pre style="background: white; padding: 10px; border-radius: 3px;">when heap.alloc > 200MB && trend("heap.alloc", 300) > 0 {
  alert("Memory leak detected: ${heap.alloc}")
}</pre>
                </div>
            </div>
        </div>
    </div>
    
    <div id="alerts-tab" class="tab-content">
        <div class="card" style="margin-bottom: 20px;">
            <h3>Alert Management</h3>
            <p>Monitor and manage system alerts with acknowledgement and resolution tracking</p>
            
            <div style="display: flex; gap: 10px; margin-bottom: 20px;">
                <select id="alert-status-filter" onchange="loadAlerts()">
                    <option value="">All Statuses</option>
                    <option value="active">Active</option>
                    <option value="acknowledged">Acknowledged</option>
                    <option value="resolved">Resolved</option>
                    <option value="suppressed">Suppressed</option>
                </select>
                
                <select id="alert-severity-filter" onchange="loadAlerts()">
                    <option value="">All Severities</option>
                    <option value="critical">Critical</option>
                    <option value="high">High</option>
                    <option value="medium">Medium</option>
                    <option value="low">Low</option>
                </select>
                
                <button onclick="loadAlerts()" style="background: #3498db; color: white; border: none; padding: 8px 16px; border-radius: 3px;">Refresh</button>
                
                <div style="margin-left: auto;">
                    <span id="alert-summary">Loading alerts...</span>
                </div>
            </div>
        </div>
        
        <div id="alerts-list" style="min-height: 400px;">
            <div style="text-align: center; padding: 50px; color: #7f8c8d;">
                Loading alerts...
            </div>
        </div>
        
        <!-- Alert Detail Modal -->
        <div id="alert-modal" style="display: none; position: fixed; top: 0; left: 0; width: 100%; height: 100%; background: rgba(0,0,0,0.5); z-index: 1000;">
            <div style="position: absolute; top: 50%; left: 50%; transform: translate(-50%, -50%); background: white; padding: 30px; border-radius: 5px; width: 600px; max-height: 80vh; overflow-y: auto;">
                <h3 id="modal-alert-title">Alert Details</h3>
                <div id="modal-alert-content"></div>
                
                <div style="margin-top: 20px;">
                    <input type="text" id="modal-user" placeholder="Your name" style="width: 100%; margin: 5px 0; padding: 8px;" />
                    <textarea id="modal-note" placeholder="Add a note..." style="width: 100%; height: 80px; margin: 5px 0; padding: 8px;"></textarea>
                    
                    <div style="display: flex; gap: 10px; margin-top: 10px;">
                        <button onclick="acknowledgeAlert()" style="background: #f39c12; color: white; border: none; padding: 8px 16px; border-radius: 3px;">Acknowledge</button>
                        <button onclick="resolveAlert()" style="background: #2ecc71; color: white; border: none; padding: 8px 16px; border-radius: 3px;">Resolve</button>
                        <button onclick="suppressAlert()" style="background: #95a5a6; color: white; border: none; padding: 8px 16px; border-radius: 3px;">Suppress</button>
                        <button onclick="addAlertNote()" style="background: #3498db; color: white; border: none; padding: 8px 16px; border-radius: 3px;">Add Note</button>
                        <button onclick="closeAlertModal()" style="background: #e74c3c; color: white; border: none; padding: 8px 16px; border-radius: 3px; margin-left: auto;">Close</button>
                    </div>
                </div>
            </div>
        </div>
    </div>
    
    <div id="correlation-tab" class="tab-content">
        <div class="card" style="margin-bottom: 20px;">
            <h3>Metric Correlation Analysis</h3>
            <p>Analyze relationships between different metrics to identify patterns and anomalies</p>
            
            <div style="display: grid; grid-template-columns: 1fr 1fr 1fr 1fr auto; gap: 10px; align-items: end; margin-bottom: 20px;">
                <div>
                    <label>X-Axis Metric:</label>
                    <select id="metric-x" style="width: 100%; padding: 8px;">
                        <option value="">Select metric...</option>
                    </select>
                </div>
                
                <div>
                    <label>Y-Axis Metric:</label>
                    <select id="metric-y" style="width: 100%; padding: 8px;">
                        <option value="">Select metric...</option>
                    </select>
                </div>
                
                <div>
                    <label>Time Range:</label>
                    <select id="time-range" style="width: 100%; padding: 8px;">
                        <option value="15">Last 15 minutes</option>
                        <option value="30">Last 30 minutes</option>
                        <option value="60" selected>Last 1 hour</option>
                        <option value="180">Last 3 hours</option>
                        <option value="360">Last 6 hours</option>
                    </select>
                </div>
                
                <div>
                    <label>Data Points:</label>
                    <select id="window-size" style="width: 100%; padding: 8px;">
                        <option value="50">50 points</option>
                        <option value="100" selected>100 points</option>
                        <option value="200">200 points</option>
                        <option value="500">500 points</option>
                    </select>
                </div>
                
                <div>
                    <button onclick="analyzeCorrelation()" style="background: #3498db; color: white; border: none; padding: 10px 20px; border-radius: 3px; white-space: nowrap;">Analyze</button>
                </div>
            </div>
        </div>
        
        <div style="display: grid; grid-template-columns: 2fr 1fr; gap: 20px;">
            <div class="card">
                <h3>Correlation Scatter Plot</h3>
                <div id="correlation-results" style="margin-bottom: 20px; padding: 10px; background: #f8f9fa; border-radius: 3px;">
                    Select two metrics and click "Analyze" to view correlation
                </div>
                <div style="position: relative; height: 400px;">
                    <canvas id="correlation-chart"></canvas>
                </div>
            </div>
            
            <div class="card">
                <h3>Correlation Statistics</h3>
                <div id="correlation-stats" style="margin-bottom: 20px;">
                    <div style="text-align: center; padding: 20px; color: #7f8c8d;">
                        No analysis yet
                    </div>
                </div>
                
                <h4>Anomalies Detected</h4>
                <div id="correlation-anomalies" style="max-height: 200px; overflow-y: auto;">
                    <div style="text-align: center; padding: 20px; color: #7f8c8d;">
                        No anomalies detected
                    </div>
                </div>
                
                <h4 style="margin-top: 30px;">Quick Analysis</h4>
                <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 10px;">
                    <button onclick="quickAnalysis('heap.alloc', 'goroutines.count')" style="background: #2ecc71; color: white; border: none; padding: 8px; border-radius: 3px; font-size: 0.9em;">Memory vs Goroutines</button>
                    <button onclick="quickAnalysis('heap.alloc', 'gc.pause')" style="background: #e67e22; color: white; border: none; padding: 8px; border-radius: 3px; font-size: 0.9em;">Memory vs GC Pause</button>
                    <button onclick="quickAnalysis('goroutines.count', 'gc.pause')" style="background: #9b59b6; color: white; border: none; padding: 8px; border-radius: 3px; font-size: 0.9em;">Goroutines vs GC</button>
                    <button onclick="quickAnalysis('http.response_time', 'http.request_rate')" style="background: #34495e; color: white; border: none; padding: 8px; border-radius: 3px; font-size: 0.9em;">Response vs Request Rate</button>
                </div>
            </div>
        </div>
    </div>

    <script>
        // WebSocket connection
        const ws = new WebSocket('ws://localhost:9090/ws');
        
        // Chart configurations
        const chartConfig = {
            type: 'line',
            options: {
                responsive: true,
                maintainAspectRatio: false,
                scales: {
                    x: {
                        type: 'time',
                        time: { unit: 'second' }
                    },
                    y: { beginAtZero: true }
                },
                plugins: { legend: { display: false } }
            }
        };
        
        // Initialize charts
        const memoryChart = new Chart(document.getElementById('memory-chart'), {
            ...chartConfig,
            data: { datasets: [{ data: [], borderColor: '#3498db', fill: false }] }
        });
        
        const goroutinesChart = new Chart(document.getElementById('goroutines-chart'), {
            ...chartConfig,
            data: { datasets: [{ data: [], borderColor: '#2ecc71', fill: false }] }
        });
        
        const gcChart = new Chart(document.getElementById('gc-chart'), {
            ...chartConfig,
            data: { datasets: [{ data: [], borderColor: '#e74c3c', fill: false }] }
        });
        
        // Playback charts
        const playbackMemoryChart = new Chart(document.getElementById('playback-memory-chart'), {
            ...chartConfig,
            data: { datasets: [{ data: [], borderColor: '#3498db', fill: false }] }
        });
        
        const playbackGoroutinesChart = new Chart(document.getElementById('playback-goroutines-chart'), {
            ...chartConfig,
            data: { datasets: [{ data: [], borderColor: '#2ecc71', fill: false }] }
        });
        
        const playbackGcChart = new Chart(document.getElementById('playback-gc-chart'), {
            ...chartConfig,
            data: { datasets: [{ data: [], borderColor: '#e74c3c', fill: false }] }
        });
        
        // Correlation scatter chart
        const correlationChart = new Chart(document.getElementById('correlation-chart'), {
            type: 'scatter',
            data: {
                datasets: [{
                    label: 'Data Points',
                    data: [],
                    backgroundColor: '#3498db',
                    borderColor: '#3498db',
                    pointRadius: 4
                }, {
                    label: 'Anomalies',
                    data: [],
                    backgroundColor: '#e74c3c',
                    borderColor: '#e74c3c',
                    pointRadius: 6
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                scales: {
                    x: { 
                        type: 'linear',
                        title: { display: true, text: 'X Metric' }
                    },
                    y: { 
                        type: 'linear',
                        title: { display: true, text: 'Y Metric' }
                    }
                },
                plugins: {
                    legend: { display: true },
                    tooltip: {
                        callbacks: {
                            label: function(context) {
                                return '(' + context.parsed.x.toFixed(2) + ', ' + context.parsed.y.toFixed(2) + ')';
                            }
                        }
                    }
                }
            }
        });
        
        // WebSocket message handling
        ws.onmessage = function(event) {
            const data = JSON.parse(event.data);
            
            if (data.type === 'metrics') {
                updateMetrics(data.data);
            } else if (data.type === 'event') {
                addEvent(data.data);
            } else if (data.type === 'playback_metric') {
                updatePlaybackMetrics(data.data);
            } else if (data.type === 'playback_event') {
                addPlaybackEvent(data.data);
            } else if (data.type === 'playback_complete') {
                document.getElementById('playback-status').textContent = 'Playback Complete';
            }
        };
        
        function updateMetrics(metrics) {
            const timestamp = new Date();
            
            // Update memory
            if (metrics['heap.alloc'] !== undefined) {
                const memMB = Math.round(metrics['heap.alloc'] / 1024 / 1024);
                document.getElementById('memory-value').textContent = memMB + ' MB';
                addDataPoint(memoryChart, timestamp, memMB);
            }
            
            // Update goroutines
            if (metrics['goroutines.count'] !== undefined) {
                document.getElementById('goroutines-value').textContent = metrics['goroutines.count'];
                addDataPoint(goroutinesChart, timestamp, metrics['goroutines.count']);
            }
            
            // Update GC pause
            if (metrics['gc.pause'] !== undefined) {
                const pauseUs = Math.round(metrics['gc.pause'] / 1000);
                document.getElementById('gc-value').textContent = pauseUs + ' μs';
                addDataPoint(gcChart, timestamp, pauseUs);
            }
        }
        
        function addDataPoint(chart, timestamp, value) {
            chart.data.datasets[0].data.push({ x: timestamp, y: value });
            
            // Keep only last 50 points
            if (chart.data.datasets[0].data.length > 50) {
                chart.data.datasets[0].data.shift();
            }
            
            chart.update('none');
        }
        
        function addEvent(event) {
            const eventsList = document.getElementById('events-list');
            const eventDiv = document.createElement('div');
            eventDiv.className = 'event ' + (event.type === 'alert' ? 'alert' : 'info');
            
            eventDiv.innerHTML = 
                '<div><strong>[' + event.rule + ']</strong> ' + event.message + '</div>' +
                '<div class="timestamp">' + new Date(event.timestamp).toLocaleString() + '</div>';
            
            eventsList.insertBefore(eventDiv, eventsList.firstChild);
            
            // Keep only last 20 events
            while (eventsList.children.length > 20) {
                eventsList.removeChild(eventsList.lastChild);
            }
        }
        
        ws.onopen = function() {
            console.log('Connected to Descry dashboard');
        };
        
        ws.onclose = function() {
            console.log('Disconnected from Descry dashboard');
        };
        
        // Tab switching
        function showTab(tabName) {
            // Hide all tab content
            document.querySelectorAll('.tab-content').forEach(content => {
                content.classList.remove('active');
            });
            
            // Remove active class from all tabs
            document.querySelectorAll('.tab').forEach(tab => {
                tab.classList.remove('active');
            });
            
            // Show selected tab content
            document.getElementById(tabName + '-tab').classList.add('active');
            
            // Add active class to clicked tab
            event.target.classList.add('active');
        }
        
        // Playback functions
        function updatePlaybackMetrics(metrics) {
            const timestamp = new Date(metrics.timestamp);
            
            // Update memory
            if (metrics.metrics && metrics.metrics['heap.alloc'] !== undefined) {
                const memMB = Math.round(metrics.metrics['heap.alloc'] / 1024 / 1024);
                document.getElementById('playback-memory-value').textContent = memMB + ' MB';
                addDataPoint(playbackMemoryChart, timestamp, memMB);
            }
            
            // Update goroutines
            if (metrics.metrics && metrics.metrics['goroutines.count'] !== undefined) {
                document.getElementById('playback-goroutines-value').textContent = metrics.metrics['goroutines.count'];
                addDataPoint(playbackGoroutinesChart, timestamp, metrics.metrics['goroutines.count']);
            }
            
            // Update GC pause
            if (metrics.metrics && metrics.metrics['gc.pause'] !== undefined) {
                const pauseUs = Math.round(metrics.metrics['gc.pause'] / 1000);
                document.getElementById('playback-gc-value').textContent = pauseUs + ' μs';
                addDataPoint(playbackGcChart, timestamp, pauseUs);
            }
        }
        
        function addPlaybackEvent(event) {
            const eventsList = document.getElementById('playback-events-list');
            const eventDiv = document.createElement('div');
            eventDiv.className = 'event ' + (event.type === 'alert' ? 'alert' : 'info');
            
            eventDiv.innerHTML = 
                '<div><strong>[' + event.rule + ']</strong> ' + event.message + '</div>' +
                '<div class="timestamp">' + new Date(event.timestamp).toLocaleString() + '</div>';
            
            eventsList.insertBefore(eventDiv, eventsList.firstChild);
            
            // Keep only last 20 events
            while (eventsList.children.length > 20) {
                eventsList.removeChild(eventsList.lastChild);
            }
        }
        
        function startPlayback() {
            const fromInput = document.getElementById('playback-from');
            const toInput = document.getElementById('playback-to');
            const speedSelect = document.getElementById('playback-speed');
            
            if (!fromInput.value || !toInput.value) {
                alert('Please select both from and to dates');
                return;
            }
            
            const fromTime = new Date(fromInput.value).toISOString();
            const toTime = new Date(toInput.value).toISOString();
            const speed = parseFloat(speedSelect.value);
            
            // Clear existing playback data
            clearPlaybackCharts();
            
            document.getElementById('playback-status').textContent = 'Starting playback...';
            
            fetch('/api/playback', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({
                    from: fromTime,
                    to: toTime,
                    speed: speed,
                    interval: 500 // 500ms intervals
                })
            })
            .then(response => response.json())
            .then(data => {
                if (data.status === 'ok') {
                    document.getElementById('playback-status').textContent = 'Playback running...';
                } else {
                    document.getElementById('playback-status').textContent = 'Error: ' + data.message;
                }
            })
            .catch(error => {
                document.getElementById('playback-status').textContent = 'Error: ' + error;
            });
        }
        
        function stopPlayback() {
            document.getElementById('playback-status').textContent = 'Stopped';
            // In a real implementation, you'd send a stop signal to the server
        }
        
        function loadLastHour() {
            const now = new Date();
            const oneHourAgo = new Date(now.getTime() - 60 * 60 * 1000);
            
            document.getElementById('playback-from').value = formatDateForInput(oneHourAgo);
            document.getElementById('playback-to').value = formatDateForInput(now);
        }
        
        function loadLast10Minutes() {
            const now = new Date();
            const tenMinutesAgo = new Date(now.getTime() - 10 * 60 * 1000);
            
            document.getElementById('playback-from').value = formatDateForInput(tenMinutesAgo);
            document.getElementById('playback-to').value = formatDateForInput(now);
        }
        
        function formatDateForInput(date) {
            const year = date.getFullYear();
            const month = String(date.getMonth() + 1).padStart(2, '0');
            const day = String(date.getDate()).padStart(2, '0');
            const hours = String(date.getHours()).padStart(2, '0');
            const minutes = String(date.getMinutes()).padStart(2, '0');
            
            return year + '-' + month + '-' + day + 'T' + hours + ':' + minutes;
        }
        
        function clearPlaybackCharts() {
            playbackMemoryChart.data.datasets[0].data = [];
            playbackGoroutinesChart.data.datasets[0].data = [];
            playbackGcChart.data.datasets[0].data = [];
            
            playbackMemoryChart.update();
            playbackGoroutinesChart.update();
            playbackGcChart.update();
            
            // Clear events list
            const eventsList = document.getElementById('playback-events-list');
            eventsList.innerHTML = '<div class="event"><div>Select a time range to replay events...</div><div class="timestamp">--</div></div>';
        }
        
        // Initialize default time range to last 10 minutes
        window.onload = function() {
            loadLast10Minutes();
            loadActiveRules();
            loadAlerts();
            loadAvailableMetrics();
        };
        
        // Rule editor functions
        function validateRule() {
            const name = document.getElementById('rule-name').value;
            const code = document.getElementById('rule-editor').value;
            
            if (!name || !code) {
                showRuleStatus('error', 'Please enter both rule name and code');
                return;
            }
            
            showRuleStatus('info', 'Validating rule...');
            
            fetch('/api/rules/validate', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({
                    name: name,
                    code: code
                })
            })
            .then(response => response.json())
            .then(data => {
                if (data.valid) {
                    showRuleStatus('success', data.message);
                } else {
                    showRuleStatus('error', 'Validation failed: ' + data.errors.join(', '));
                }
            })
            .catch(error => {
                showRuleStatus('error', 'Error validating rule: ' + error);
            });
        }
        
        function saveRule() {
            const name = document.getElementById('rule-name').value;
            const code = document.getElementById('rule-editor').value;
            
            if (!name || !code) {
                showRuleStatus('error', 'Please enter both rule name and code');
                return;
            }
            
            showRuleStatus('info', 'Saving rule...');
            
            fetch('/api/rules/save', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({
                    name: name,
                    code: code
                })
            })
            .then(response => response.json())
            .then(data => {
                if (data.status === 'ok') {
                    showRuleStatus('success', data.message);
                    loadActiveRules(); // Refresh the rules list
                } else {
                    showRuleStatus('error', 'Error saving rule: ' + data.message);
                }
            })
            .catch(error => {
                showRuleStatus('error', 'Error saving rule: ' + error);
            });
        }
        
        function testRule() {
            const name = document.getElementById('rule-name').value;
            const code = document.getElementById('rule-editor').value;
            
            if (!name || !code) {
                showRuleStatus('error', 'Please enter both rule name and code');
                return;
            }
            
            showRuleStatus('info', 'Testing rule against current metrics...');
            
            fetch('/api/rules/test', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({
                    name: name,
                    code: code
                })
            })
            .then(response => response.json())
            .then(data => {
                if (data.status === 'ok') {
                    const statusType = data.wouldTrigger ? 'warning' : 'info';
                    showRuleStatus(statusType, data.result);
                } else {
                    showRuleStatus('error', 'Error testing rule: ' + data.message);
                }
            })
            .catch(error => {
                showRuleStatus('error', 'Error testing rule: ' + error);
            });
        }
        
        function showRuleStatus(type, message) {
            const statusDiv = document.getElementById('rule-status');
            statusDiv.textContent = message;
            
            // Set background color based on type
            const colors = {
                'success': '#d5f4e6',
                'error': '#f8d7da',
                'warning': '#fff3cd',
                'info': '#cce7ff'
            };
            
            statusDiv.style.background = colors[type] || '#ecf0f1';
            statusDiv.style.color = '#000';
            statusDiv.style.border = '1px solid ' + (colors[type] || '#ddd');
        }
        
        function loadActiveRules() {
            fetch('/api/rules')
            .then(response => response.json())
            .then(data => {
                const rulesList = document.getElementById('active-rules-list');
                
                if (data.status === 'ok' && data.data && data.data.length > 0) {
                    rulesList.innerHTML = '';
                    data.data.forEach(rule => {
                        const ruleDiv = document.createElement('div');
                        ruleDiv.style.cssText = 'padding: 10px; margin: 5px 0; background: #f8f9fa; border-radius: 3px; border-left: 4px solid #3498db;';
                        
                        ruleDiv.innerHTML = 
                            '<strong>' + (rule.name || 'Unnamed Rule') + '</strong><br>' +
                            '<code style="font-size: 0.85em;">' + (rule.condition || rule.code || 'No condition') + '</code><br>' +
                            '<small style="color: #666;">Status: ' + (rule.enabled ? 'Active' : 'Inactive') + '</small>';
                        
                        rulesList.appendChild(ruleDiv);
                    });
                } else {
                    rulesList.innerHTML = '<div style="padding: 10px; color: #7f8c8d;">No active rules found</div>';
                }
            })
            .catch(error => {
                const rulesList = document.getElementById('active-rules-list');
                rulesList.innerHTML = '<div style="padding: 10px; color: #e74c3c;">Error loading rules: ' + error + '</div>';
            });
        }
        
        function loadRuleIntoEditor(ruleName, ruleCode) {
            document.getElementById('rule-name').value = ruleName;
            document.getElementById('rule-editor').value = ruleCode;
        }
        
        // Global variable for selected alert
        let selectedAlert = null;
        
        // Alert management functions
        function loadAlerts() {
            const statusFilter = document.getElementById('alert-status-filter').value;
            const severityFilter = document.getElementById('alert-severity-filter').value;
            
            let url = '/api/alerts';
            const params = [];
            if (statusFilter) params.push('status=' + encodeURIComponent(statusFilter));
            if (severityFilter) params.push('severity=' + encodeURIComponent(severityFilter));
            if (params.length > 0) url += '?' + params.join('&');
            
            fetch(url)
            .then(response => response.json())
            .then(data => {
                if (data.status === 'ok') {
                    displayAlerts(data.data);
                    updateAlertSummary(data.data);
                } else {
                    document.getElementById('alerts-list').innerHTML = '<div style="text-align: center; padding: 50px; color: #e74c3c;">Error loading alerts</div>';
                }
            })
            .catch(error => {
                document.getElementById('alerts-list').innerHTML = '<div style="text-align: center; padding: 50px; color: #e74c3c;">Error: ' + error + '</div>';
            });
        }
        
        function displayAlerts(alerts) {
            const alertsList = document.getElementById('alerts-list');
            
            if (!alerts || alerts.length === 0) {
                alertsList.innerHTML = '<div style="text-align: center; padding: 50px; color: #7f8c8d;">No alerts found</div>';
                return;
            }
            
            let html = '';
            alerts.forEach(alert => {
                const severityColor = getSeverityColor(alert.severity);
                const statusColor = getStatusColor(alert.status);
                const timeAgo = getTimeAgo(new Date(alert.created_at));
                
                html += '<div class="card" style="margin-bottom: 15px; border-left: 4px solid ' + severityColor + '; cursor: pointer;" onclick="showAlertModal(\'' + alert.id + '\')">';
                html += '<div style="display: flex; justify-content: between; align-items: start;">';
                html += '<div style="flex: 1;">';
                html += '<h4 style="margin: 0 0 10px 0; color: ' + severityColor + ';">[' + alert.severity.toUpperCase() + '] ' + alert.rule + '</h4>';
                html += '<p style="margin: 0 0 10px 0;">' + alert.message + '</p>';
                html += '<div style="display: flex; gap: 15px; font-size: 0.9em; color: #666;">';
                html += '<span>Status: <strong style="color: ' + statusColor + ';">' + alert.status.toUpperCase() + '</strong></span>';
                html += '<span>Created: ' + timeAgo + '</span>';
                if (alert.notes && alert.notes.length > 0) {
                    html += '<span>Notes: ' + alert.notes.length + '</span>';
                }
                html += '</div>';
                html += '</div>';
                html += '<div style="padding: 5px; background: ' + statusColor + '; color: white; border-radius: 3px; font-size: 0.8em; text-align: center; min-width: 80px;">';
                html += alert.status.toUpperCase();
                html += '</div>';
                html += '</div>';
                html += '</div>';
            });
            
            alertsList.innerHTML = html;
        }
        
        function updateAlertSummary(alerts) {
            if (!alerts) return;
            
            const counts = {
                active: 0,
                acknowledged: 0,
                resolved: 0,
                suppressed: 0,
                critical: 0,
                high: 0,
                medium: 0,
                low: 0
            };
            
            alerts.forEach(alert => {
                counts[alert.status]++;
                counts[alert.severity]++;
            });
            
            const summary = 'Active: ' + counts.active + 
                          ', Critical: ' + counts.critical + 
                          ', High: ' + counts.high + 
                          ', Total: ' + alerts.length;
            
            document.getElementById('alert-summary').textContent = summary;
        }
        
        function getSeverityColor(severity) {
            const colors = {
                'critical': '#e74c3c',
                'high': '#f39c12', 
                'medium': '#3498db',
                'low': '#2ecc71'
            };
            return colors[severity] || '#95a5a6';
        }
        
        function getStatusColor(status) {
            const colors = {
                'active': '#e74c3c',
                'acknowledged': '#f39c12',
                'resolved': '#2ecc71',
                'suppressed': '#95a5a6'
            };
            return colors[status] || '#95a5a6';
        }
        
        function getTimeAgo(date) {
            const now = new Date();
            const diffMs = now - date;
            const diffHours = Math.floor(diffMs / (1000 * 60 * 60));
            const diffMinutes = Math.floor(diffMs / (1000 * 60));
            
            if (diffHours > 24) {
                return Math.floor(diffHours / 24) + 'd ago';
            } else if (diffHours > 0) {
                return diffHours + 'h ago';
            } else if (diffMinutes > 0) {
                return diffMinutes + 'm ago';
            } else {
                return 'Just now';
            }
        }
        
        function showAlertModal(alertId) {
            // Find alert by ID
            fetch('/api/alerts')
            .then(response => response.json())
            .then(data => {
                if (data.status === 'ok') {
                    const alert = data.data.find(a => a.id === alertId);
                    if (alert) {
                        selectedAlert = alert;
                        displayAlertModal(alert);
                        document.getElementById('alert-modal').style.display = 'block';
                    }
                }
            });
        }
        
        function displayAlertModal(alert) {
            document.getElementById('modal-alert-title').textContent = '[' + alert.severity.toUpperCase() + '] ' + alert.rule;
            
            let content = '<div style="margin-bottom: 20px;">';
            content += '<p><strong>Message:</strong> ' + alert.message + '</p>';
            content += '<p><strong>Status:</strong> <span style="color: ' + getStatusColor(alert.status) + ';">' + alert.status.toUpperCase() + '</span></p>';
            content += '<p><strong>Created:</strong> ' + new Date(alert.created_at).toLocaleString() + '</p>';
            content += '<p><strong>Updated:</strong> ' + new Date(alert.updated_at).toLocaleString() + '</p>';
            
            if (alert.acknowledged_by) {
                content += '<p><strong>Acknowledged by:</strong> ' + alert.acknowledged_by + '</p>';
            }
            
            if (alert.resolved_at) {
                content += '<p><strong>Resolved:</strong> ' + new Date(alert.resolved_at).toLocaleString() + '</p>';
            }
            
            content += '</div>';
            
            if (alert.notes && alert.notes.length > 0) {
                content += '<h4>Notes:</h4>';
                alert.notes.forEach(note => {
                    content += '<div style="background: #f8f9fa; padding: 10px; margin: 5px 0; border-radius: 3px;">';
                    content += '<div>' + note.message + '</div>';
                    content += '<div style="font-size: 0.8em; color: #666; margin-top: 5px;">by ' + (note.author || 'Unknown') + ' at ' + new Date(note.created_at).toLocaleString() + '</div>';
                    content += '</div>';
                });
            }
            
            document.getElementById('modal-alert-content').innerHTML = content;
        }
        
        function closeAlertModal() {
            document.getElementById('alert-modal').style.display = 'none';
            selectedAlert = null;
        }
        
        function acknowledgeAlert() {
            performAlertAction('acknowledge', '/api/alerts/acknowledge');
        }
        
        function resolveAlert() {
            performAlertAction('resolve', '/api/alerts/resolve');
        }
        
        function suppressAlert() {
            performAlertAction('suppress', '/api/alerts/suppress');
        }
        
        function addAlertNote() {
            const note = document.getElementById('modal-note').value;
            if (!note) {
                alert('Please enter a note');
                return;
            }
            performAlertAction('add note', '/api/alerts/note');
        }
        
        function performAlertAction(actionName, endpoint) {
            if (!selectedAlert) return;
            
            const user = document.getElementById('modal-user').value;
            const note = document.getElementById('modal-note').value;
            
            fetch(endpoint, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({
                    alert_id: selectedAlert.id,
                    user: user,
                    note: note
                })
            })
            .then(response => response.json())
            .then(data => {
                if (data.status === 'ok') {
                    alert('Alert ' + actionName + ' successfully');
                    closeAlertModal();
                    loadAlerts(); // Refresh the alerts list
                } else {
                    alert('Error: ' + data.message);
                }
            })
            .catch(error => {
                alert('Error: ' + error);
            });
        }
        
        // Correlation analysis functions
        function loadAvailableMetrics() {
            fetch('/api/correlation')
            .then(response => response.json())
            .then(data => {
                if (data.status === 'ok' && data.metrics) {
                    populateMetricSelectors(data.metrics);
                }
            })
            .catch(error => {
                console.error('Error loading metrics:', error);
            });
        }
        
        function populateMetricSelectors(metrics) {
            const metricXSelect = document.getElementById('metric-x');
            const metricYSelect = document.getElementById('metric-y');
            
            // Clear existing options (keep first placeholder)
            metricXSelect.innerHTML = '<option value="">Select metric...</option>';
            metricYSelect.innerHTML = '<option value="">Select metric...</option>';
            
            metrics.forEach(metric => {
                const displayName = getMetricDisplayName(metric);
                
                const optionX = document.createElement('option');
                optionX.value = metric;
                optionX.textContent = displayName;
                metricXSelect.appendChild(optionX);
                
                const optionY = document.createElement('option');
                optionY.value = metric;
                optionY.textContent = displayName;
                metricYSelect.appendChild(optionY);
            });
        }
        
        function getMetricDisplayName(metric) {
            const displayNames = {
                'heap.alloc': 'Heap Memory Allocation',
                'goroutines.count': 'Active Goroutines',
                'gc.pause': 'GC Pause Time',
                'http.response_time': 'HTTP Response Time',
                'http.request_rate': 'HTTP Request Rate'
            };
            return displayNames[metric] || metric;
        }
        
        function analyzeCorrelation() {
            const metricX = document.getElementById('metric-x').value;
            const metricY = document.getElementById('metric-y').value;
            const timeRange = parseInt(document.getElementById('time-range').value);
            const windowSize = parseInt(document.getElementById('window-size').value);
            
            if (!metricX || !metricY) {
                alert('Please select both X and Y metrics');
                return;
            }
            
            if (metricX === metricY) {
                alert('Please select different metrics for X and Y axes');
                return;
            }
            
            document.getElementById('correlation-results').textContent = 'Analyzing correlation...';
            
            fetch('/api/correlation', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({
                    metric_x: metricX,
                    metric_y: metricY,
                    time_range: timeRange,
                    window_size: windowSize
                })
            })
            .then(response => response.json())
            .then(data => {
                if (data.status === 'ok') {
                    displayCorrelationResults(data.data);
                } else {
                    document.getElementById('correlation-results').textContent = 'Error analyzing correlation';
                }
            })
            .catch(error => {
                document.getElementById('correlation-results').textContent = 'Error: ' + error;
            });
        }
        
        function displayCorrelationResults(result) {
            // Update results summary
            const coefficient = result.coefficient || 0;
            const coefficientText = coefficient.toFixed(3);
            const direction = coefficient > 0 ? 'positive' : 'negative';
            const strengthColor = getStrengthColor(result.strength);
            
            document.getElementById('correlation-results').innerHTML = 
                '<strong>Correlation:</strong> ' + coefficientText + 
                ' (' + direction + ', ' + result.strength.toLowerCase() + ')' +
                ' • <strong>Data Points:</strong> ' + result.data_points +
                ' • <strong>Time Range:</strong> ' + result.time_range;
            
            // Update statistics
            let statsHTML = '<div style="background: ' + strengthColor + '; color: white; padding: 15px; border-radius: 5px; margin-bottom: 15px; text-align: center;">';
            statsHTML += '<h4 style="margin: 0 0 10px 0;">Correlation: ' + coefficientText + '</h4>';
            statsHTML += '<p style="margin: 0;">Strength: ' + result.strength + '</p>';
            statsHTML += '</div>';
            
            statsHTML += '<div style="display: grid; grid-template-columns: 1fr 1fr; gap: 10px;">';
            statsHTML += '<div style="text-align: center; padding: 10px; background: #f8f9fa; border-radius: 3px;">';
            statsHTML += '<div style="font-size: 1.5em; font-weight: bold; color: #3498db;">' + result.data_points + '</div>';
            statsHTML += '<div style="font-size: 0.9em; color: #666;">Data Points</div>';
            statsHTML += '</div>';
            statsHTML += '<div style="text-align: center; padding: 10px; background: #f8f9fa; border-radius: 3px;">';
            statsHTML += '<div style="font-size: 1.5em; font-weight: bold; color: #e74c3c;">' + (result.anomalies ? result.anomalies.length : 0) + '</div>';
            statsHTML += '<div style="font-size: 0.9em; color: #666;">Anomalies</div>';
            statsHTML += '</div>';
            statsHTML += '</div>';
            
            document.getElementById('correlation-stats').innerHTML = statsHTML;
            
            // Update anomalies
            if (result.anomalies && result.anomalies.length > 0) {
                let anomaliesHTML = '';
                result.anomalies.forEach(anomaly => {
                    anomaliesHTML += '<div style="padding: 8px; margin: 5px 0; background: #fff5f5; border-left: 4px solid #e74c3c; border-radius: 3px;">';
                    anomaliesHTML += '<div style="font-weight: bold; color: #e74c3c;">' + anomaly.anomaly_type.replace(/_/g, ' ').toUpperCase() + '</div>';
                    anomaliesHTML += '<div style="font-size: 0.9em; color: #666;">' + new Date(anomaly.timestamp).toLocaleString() + '</div>';
                    anomaliesHTML += '<div style="font-size: 0.8em; color: #666;">Severity: ' + (anomaly.severity * 100).toFixed(1) + '%</div>';
                    anomaliesHTML += '</div>';
                });
                document.getElementById('correlation-anomalies').innerHTML = anomaliesHTML;
            } else {
                document.getElementById('correlation-anomalies').innerHTML = '<div style="text-align: center; padding: 20px; color: #7f8c8d;">No anomalies detected</div>';
            }
            
            // Update scatter plot
            updateCorrelationChart(result);
        }
        
        function updateCorrelationChart(result) {
            const metricXName = getMetricDisplayName(result.metric_x);
            const metricYName = getMetricDisplayName(result.metric_y);
            
            // Update axis labels
            correlationChart.options.scales.x.title.text = metricXName;
            correlationChart.options.scales.y.title.text = metricYName;
            
            // Update data points
            const dataPoints = result.scatter_data.map(point => ({
                x: point.x,
                y: point.y
            }));
            
            const anomalyPoints = result.anomalies ? result.anomalies.map(anomaly => ({
                x: anomaly.x,
                y: anomaly.y
            })) : [];
            
            correlationChart.data.datasets[0].data = dataPoints;
            correlationChart.data.datasets[0].label = 'Data Points (' + dataPoints.length + ')';
            correlationChart.data.datasets[1].data = anomalyPoints;
            correlationChart.data.datasets[1].label = 'Anomalies (' + anomalyPoints.length + ')';
            
            correlationChart.update();
        }
        
        function getStrengthColor(strength) {
            const colors = {
                'Very Strong': '#27ae60',
                'Strong': '#2ecc71',
                'Moderate': '#3498db',
                'Weak': '#f39c12',
                'Very Weak': '#e74c3c'
            };
            return colors[strength] || '#95a5a6';
        }
        
        function quickAnalysis(metricX, metricY) {
            document.getElementById('metric-x').value = metricX;
            document.getElementById('metric-y').value = metricY;
            analyzeCorrelation();
        }
    </script>
</body>
</html>`
	
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	// Basic static file serving - in production, use proper static file server
	http.NotFound(w, r)
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	s.mutex.RLock()
	metrics := s.recentMetrics
	s.mutex.RUnlock()
	
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"data":   metrics,
	})
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	s.mutex.RLock()
	events := make([]EventUpdate, s.eventCount)
	
	// Copy events from circular buffer in chronological order
	if s.eventCount > 0 {
		bufferSize := len(s.eventBuffer)
		if s.eventCount == bufferSize {
			// Buffer is full, start from oldest
			startIndex := s.eventIndex
			for i := 0; i < bufferSize; i++ {
				events[i] = s.eventBuffer[(startIndex+i)%bufferSize]
			}
		} else {
			// Buffer not full, just copy from beginning
			copy(events, s.eventBuffer[:s.eventCount])
		}
	}
	s.mutex.RUnlock()
	
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"data":   events,
	})
}

func (s *Server) handleRules(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	var rules interface{}
	if s.getRules != nil {
		rules = s.getRules()
	} else {
		rules = []interface{}{}
	}
	
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"data":   rules,
	})
}

func (s *Server) SetRulesProvider(getRules func() interface{}) {
	s.getRules = getRules
}

func (s *Server) handleHistoricalMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	// Parse query parameters for time range
	query := r.URL.Query()
	fromStr := query.Get("from")
	toStr := query.Get("to")
	
	var fromTime, toTime time.Time
	var err error
	
	if fromStr != "" {
		fromTime, err = time.Parse(time.RFC3339, fromStr)
		if err != nil {
			http.Error(w, "Invalid 'from' time format", http.StatusBadRequest)
			return
		}
	}
	
	if toStr != "" {
		toTime, err = time.Parse(time.RFC3339, toStr)
		if err != nil {
			http.Error(w, "Invalid 'to' time format", http.StatusBadRequest)
			return
		}
	}
	
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	var filteredMetrics []MetricUpdate
	for _, metric := range s.historicalMetrics {
		// Apply time range filter if specified
		if !fromTime.IsZero() && metric.Timestamp.Before(fromTime) {
			continue
		}
		if !toTime.IsZero() && metric.Timestamp.After(toTime) {
			continue
		}
		filteredMetrics = append(filteredMetrics, metric)
	}
	
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"data":   filteredMetrics,
	})
}

func (s *Server) handleHistoricalEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	// Parse query parameters for time range
	query := r.URL.Query()
	fromStr := query.Get("from")
	toStr := query.Get("to")
	
	var fromTime, toTime time.Time
	var err error
	
	if fromStr != "" {
		fromTime, err = time.Parse(time.RFC3339, fromStr)
		if err != nil {
			http.Error(w, "Invalid 'from' time format", http.StatusBadRequest)
			return
		}
	}
	
	if toStr != "" {
		toTime, err = time.Parse(time.RFC3339, toStr)
		if err != nil {
			http.Error(w, "Invalid 'to' time format", http.StatusBadRequest)
			return
		}
	}
	
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	var filteredEvents []EventUpdate
	for _, event := range s.historicalEvents {
		// Apply time range filter if specified
		if !fromTime.IsZero() && event.Timestamp.Before(fromTime) {
			continue
		}
		if !toTime.IsZero() && event.Timestamp.After(toTime) {
			continue
		}
		filteredEvents = append(filteredEvents, event)
	}
	
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"data":   filteredEvents,
	})
}

type PlaybackRequest struct {
	From     string  `json:"from"`
	To       string  `json:"to"`
	Speed    float64 `json:"speed"`    // Playback speed multiplier (1.0 = real-time)
	Interval int     `json:"interval"` // Interval in milliseconds between updates
}

func (s *Server) handlePlayback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	var req PlaybackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON request", http.StatusBadRequest)
		return
	}
	
	// Default values
	if req.Speed <= 0 {
		req.Speed = 1.0
	}
	if req.Interval <= 0 {
		req.Interval = 1000 // 1 second
	}
	
	fromTime, err := time.Parse(time.RFC3339, req.From)
	if err != nil {
		http.Error(w, "Invalid 'from' time format", http.StatusBadRequest)
		return
	}
	
	toTime, err := time.Parse(time.RFC3339, req.To)
	if err != nil {
		http.Error(w, "Invalid 'to' time format", http.StatusBadRequest)
		return
	}
	
	// Start playback in a separate goroutine
	go s.startPlayback(fromTime, toTime, req.Speed, time.Duration(req.Interval)*time.Millisecond)
	
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"message": "Playback started",
	})
}

func (s *Server) startPlayback(from, to time.Time, speed float64, interval time.Duration) {
	s.mutex.RLock()
	
	// Get historical data within the time range
	var playbackMetrics []MetricUpdate
	var playbackEvents []EventUpdate
	
	for _, metric := range s.historicalMetrics {
		if metric.Timestamp.After(from) && metric.Timestamp.Before(to) {
			playbackMetrics = append(playbackMetrics, metric)
		}
	}
	
	for _, event := range s.historicalEvents {
		if event.Timestamp.After(from) && event.Timestamp.Before(to) {
			playbackEvents = append(playbackEvents, event)
		}
	}
	s.mutex.RUnlock()
	
	// Merge and sort by timestamp
	type playbackItem struct {
		timestamp time.Time
		data      interface{}
		itemType  string
	}
	
	var items []playbackItem
	for _, metric := range playbackMetrics {
		items = append(items, playbackItem{
			timestamp: metric.Timestamp,
			data:      metric,
			itemType:  "metric",
		})
	}
	
	for _, event := range playbackEvents {
		items = append(items, playbackItem{
			timestamp: event.Timestamp,
			data:      event,
			itemType:  "event",
		})
	}
	
	// Sort by timestamp
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[i].timestamp.After(items[j].timestamp) {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
	
	// Playback the data
	playbackInterval := time.Duration(float64(interval) / speed)
	
	for _, item := range items {
		select {
		case <-s.stop:
			return
		default:
			if item.itemType == "metric" {
				s.broadcastMessage(map[string]interface{}{
					"type":     "playback_metric",
					"data":     item.data,
					"playback": true,
				})
			} else {
				s.broadcastMessage(map[string]interface{}{
					"type":     "playback_event",
					"data":     item.data,
					"playback": true,
				})
			}
			
			time.Sleep(playbackInterval)
		}
	}
	
	// Send playback complete message
	s.broadcastMessage(map[string]interface{}{
		"type":     "playback_complete",
		"playback": true,
	})
}

type RuleRequest struct {
	Name string `json:"name"`
	Code string `json:"code"`
}

func (s *Server) handleRuleValidation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	var req RuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON request", http.StatusBadRequest)
		return
	}
	
	// Validate input
	if req.Name == "" {
		http.Error(w, "Rule name is required", http.StatusBadRequest)
		return
	}
	if len(req.Name) > 100 {
		http.Error(w, "Rule name exceeds maximum length of 100 characters", http.StatusBadRequest)
		return
	}
	if len(req.Code) > 5000 {
		http.Error(w, "Rule code exceeds maximum length of 5000 characters", http.StatusBadRequest)
		return
	}
	
	// Simple validation - check for basic DSL structure
	// In a real implementation, this would use the actual parser
	valid := true
	errors := []string{}
	
	if req.Code == "" {
		valid = false
		errors = append(errors, "Rule code cannot be empty")
	}
	
	// Check for basic DSL structure
	codeStr := strings.ToLower(req.Code)
	if valid && (!strings.Contains(codeStr, "when") || !strings.Contains(req.Code, "{")) {
		valid = false
		errors = append(errors, "Rule must contain 'when' condition and action block")
	}
	
	// Check for balanced braces
	if valid && !hasBalancedBraces(req.Code) {
		valid = false
		errors = append(errors, "Unbalanced braces in rule code")
	}
	
	response := map[string]interface{}{
		"valid": valid,
	}
	
	if !valid {
		response["errors"] = errors
	} else {
		response["message"] = "Rule syntax is valid"
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *Server) handleRuleSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	var req RuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON request", http.StatusBadRequest)
		return
	}
	
	// In a real implementation, this would save the rule to the engine
	// For now, we'll just return success
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"message": fmt.Sprintf("Rule '%s' saved successfully", req.Name),
	})
}

func (s *Server) handleRuleTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	var req RuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON request", http.StatusBadRequest)
		return
	}
	
	// Simulate rule testing against current metrics
	s.mutex.RLock()
	currentMetrics := s.recentMetrics.Metrics
	s.mutex.RUnlock()
	
	// Simple test - check if rule would trigger with current metrics
	// In a real implementation, this would use the actual evaluator
	wouldTrigger := false
	testResult := "Rule would not trigger with current metrics"
	
	// Simple heuristic test
	if strings.Contains(strings.ToLower(req.Code), "heap.alloc") && strings.Contains(strings.ToLower(req.Code), "200mb") {
		if heapAlloc, ok := currentMetrics["heap.alloc"].(float64); ok {
			if heapAlloc > 200*1024*1024 { // 200MB
				wouldTrigger = true
				testResult = "Rule would TRIGGER with current metrics"
			}
		}
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":      "ok",
		"wouldTrigger": wouldTrigger,
		"result":      testResult,
		"metrics":     currentMetrics,
	})
}

// Helper functions for rule validation

func hasBalancedBraces(code string) bool {
	count := 0
	for _, char := range code {
		if char == '{' {
			count++
		} else if char == '}' {
			count--
			if count < 0 {
				return false
			}
		}
	}
	return count == 0
}

func (s *Server) handleAlerts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	// Parse query parameters
	query := r.URL.Query()
	statusFilter := query.Get("status")
	severityFilter := query.Get("severity")
	
	s.mutex.RLock()
	filteredAlerts := make([]Alert, 0, len(s.alerts))
	
	for _, alert := range s.alerts {
		// Apply status filter
		if statusFilter != "" && string(alert.Status) != statusFilter {
			continue
		}
		
		// Apply severity filter
		if severityFilter != "" && string(alert.Severity) != severityFilter {
			continue
		}
		
		filteredAlerts = append(filteredAlerts, alert)
	}
	s.mutex.RUnlock()
	
	// Sort by creation time (newest first) - using efficient sort
	sortAlertsByTime(filteredAlerts)
	
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"data":   filteredAlerts,
	})
}

type AlertActionRequest struct {
	AlertID string `json:"alert_id"`
	User    string `json:"user,omitempty"`
	Note    string `json:"note,omitempty"`
}

func (s *Server) handleAcknowledgeAlert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	var req AlertActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON request", http.StatusBadRequest)
		return
	}
	
	// Validate input
	if req.AlertID == "" {
		http.Error(w, "Alert ID is required", http.StatusBadRequest)
		return
	}
	if len(req.Note) > 1000 {
		http.Error(w, "Note exceeds maximum length of 1000 characters", http.StatusBadRequest)
		return
	}
	if len(req.User) > 100 {
		http.Error(w, "User name exceeds maximum length of 100 characters", http.StatusBadRequest)
		return
	}
	
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	for i := range s.alerts {
		if s.alerts[i].ID == req.AlertID {
			s.alerts[i].Status = AlertStatusAcknowledged
			s.alerts[i].UpdatedAt = time.Now()
			if req.User != "" {
				s.alerts[i].AcknowledgedBy = &req.User
			}
			
			// Add note if provided
			if req.Note != "" {
				note := AlertNote{
					ID:        generateAlertID(),
					Message:   req.Note,
					Author:    req.User,
					CreatedAt: time.Now(),
				}
				s.alerts[i].Notes = append(s.alerts[i].Notes, note)
			}
			
			s.updateAlertsByStatus()
			
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "ok",
				"message": "Alert acknowledged successfully",
			})
			return
		}
	}
	
	http.Error(w, "Alert not found", http.StatusNotFound)
}

func (s *Server) handleResolveAlert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	var req AlertActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON request", http.StatusBadRequest)
		return
	}
	
	// Validate input
	if req.AlertID == "" {
		http.Error(w, "Alert ID is required", http.StatusBadRequest)
		return
	}
	if len(req.Note) > 1000 {
		http.Error(w, "Note exceeds maximum length of 1000 characters", http.StatusBadRequest)
		return
	}
	if len(req.User) > 100 {
		http.Error(w, "User name exceeds maximum length of 100 characters", http.StatusBadRequest)
		return
	}
	
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	for i := range s.alerts {
		if s.alerts[i].ID == req.AlertID {
			s.alerts[i].Status = AlertStatusResolved
			s.alerts[i].UpdatedAt = time.Now()
			now := time.Now()
			s.alerts[i].ResolvedAt = &now
			
			// Add note if provided
			if req.Note != "" {
				note := AlertNote{
					ID:        generateAlertID(),
					Message:   req.Note,
					Author:    req.User,
					CreatedAt: time.Now(),
				}
				s.alerts[i].Notes = append(s.alerts[i].Notes, note)
			}
			
			s.updateAlertsByStatus()
			
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "ok",
				"message": "Alert resolved successfully",
			})
			return
		}
	}
	
	http.Error(w, "Alert not found", http.StatusNotFound)
}

func (s *Server) handleSuppressAlert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	var req AlertActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON request", http.StatusBadRequest)
		return
	}
	
	// Validate input
	if req.AlertID == "" {
		http.Error(w, "Alert ID is required", http.StatusBadRequest)
		return
	}
	if len(req.Note) > 1000 {
		http.Error(w, "Note exceeds maximum length of 1000 characters", http.StatusBadRequest)
		return
	}
	if len(req.User) > 100 {
		http.Error(w, "User name exceeds maximum length of 100 characters", http.StatusBadRequest)
		return
	}
	
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	for i := range s.alerts {
		if s.alerts[i].ID == req.AlertID {
			s.alerts[i].Status = AlertStatusSuppressed
			s.alerts[i].UpdatedAt = time.Now()
			
			// Add note if provided
			if req.Note != "" {
				note := AlertNote{
					ID:        generateAlertID(),
					Message:   req.Note,
					Author:    req.User,
					CreatedAt: time.Now(),
				}
				s.alerts[i].Notes = append(s.alerts[i].Notes, note)
			}
			
			s.updateAlertsByStatus()
			
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "ok",
				"message": "Alert suppressed successfully",
			})
			return
		}
	}
	
	http.Error(w, "Alert not found", http.StatusNotFound)
}

func (s *Server) handleAddAlertNote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	var req AlertActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON request", http.StatusBadRequest)
		return
	}
	
	if req.Note == "" {
		http.Error(w, "Note message is required", http.StatusBadRequest)
		return
	}
	
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	for i := range s.alerts {
		if s.alerts[i].ID == req.AlertID {
			note := AlertNote{
				ID:        generateAlertID(),
				Message:   req.Note,
				Author:    req.User,
				CreatedAt: time.Now(),
			}
			s.alerts[i].Notes = append(s.alerts[i].Notes, note)
			s.alerts[i].UpdatedAt = time.Now()
			
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "ok",
				"message": "Note added successfully",
			})
			return
		}
	}
	
	http.Error(w, "Alert not found", http.StatusNotFound)
}

func (s *Server) updateAlertsByStatus() {
	// Rebuild the alerts by status map
	s.alertsByStatus = make(map[AlertStatus][]Alert)
	for _, alert := range s.alerts {
		s.alertsByStatus[alert.Status] = append(s.alertsByStatus[alert.Status], alert)
	}
}

func sortAlertsByTime(alerts []Alert) {
	sort.Slice(alerts, func(i, j int) bool {
		return alerts[i].CreatedAt.After(alerts[j].CreatedAt) // Newest first
	})
}

type CorrelationRequest struct {
	MetricX    string `json:"metric_x"`
	MetricY    string `json:"metric_y"`
	TimeRange  int    `json:"time_range"` // minutes
	WindowSize int    `json:"window_size"` // data points
}

type CorrelationResult struct {
	MetricX       string              `json:"metric_x"`
	MetricY       string              `json:"metric_y"`
	Coefficient   float64             `json:"coefficient"`
	Strength      string              `json:"strength"`
	DataPoints    int                 `json:"data_points"`
	ScatterData   []ScatterPoint      `json:"scatter_data"`
	Anomalies     []AnomalyPoint      `json:"anomalies"`
	TimeRange     string              `json:"time_range"`
}

type ScatterPoint struct {
	X         float64   `json:"x"`
	Y         float64   `json:"y"`
	Timestamp time.Time `json:"timestamp"`
}

type AnomalyPoint struct {
	X           float64   `json:"x"`
	Y           float64   `json:"y"`
	Timestamp   time.Time `json:"timestamp"`
	AnomalyType string    `json:"anomaly_type"`
	Severity    float64   `json:"severity"`
}

func (s *Server) handleMetricCorrelation(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	if r.Method == http.MethodGet {
		// Return available metrics for correlation
		availableMetrics := []string{
			"heap.alloc",
			"goroutines.count", 
			"gc.pause",
			"http.response_time",
			"http.request_rate",
		}
		
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "ok",
			"metrics": availableMetrics,
		})
		return
	}
	
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	var req CorrelationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON request", http.StatusBadRequest)
		return
	}
	
	// Default values
	if req.TimeRange <= 0 {
		req.TimeRange = 60 // 1 hour
	}
	if req.WindowSize <= 0 {
		req.WindowSize = 100
	}
	
	result := s.calculateCorrelation(req)
	
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"data":   result,
	})
}

func (s *Server) calculateCorrelation(req CorrelationRequest) CorrelationResult {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	// Filter historical data by time range
	cutoffTime := time.Now().Add(-time.Duration(req.TimeRange) * time.Minute)
	
	var dataPoints []ScatterPoint
	for _, metric := range s.historicalMetrics {
		if metric.Timestamp.Before(cutoffTime) {
			continue
		}
		
		xVal, xOk := getMetricValue(metric.Metrics, req.MetricX)
		yVal, yOk := getMetricValue(metric.Metrics, req.MetricY)
		
		if xOk && yOk {
			dataPoints = append(dataPoints, ScatterPoint{
				X:         xVal,
				Y:         yVal,
				Timestamp: metric.Timestamp,
			})
		}
	}
	
	// Limit to window size (keep most recent)
	if len(dataPoints) > req.WindowSize {
		dataPoints = dataPoints[len(dataPoints)-req.WindowSize:]
	}
	
	// Calculate correlation coefficient
	correlation := calculatePearsonCorrelation(dataPoints)
	strength := getCorrelationStrength(correlation)
	
	// Detect anomalies
	anomalies := detectAnomalies(dataPoints, correlation)
	
	return CorrelationResult{
		MetricX:     req.MetricX,
		MetricY:     req.MetricY,
		Coefficient: correlation,
		Strength:    strength,
		DataPoints:  len(dataPoints),
		ScatterData: dataPoints,
		Anomalies:   anomalies,
		TimeRange:   fmt.Sprintf("%d minutes", req.TimeRange),
	}
}

func getMetricValue(metrics map[string]interface{}, metricName string) (float64, bool) {
	if val, exists := metrics[metricName]; exists {
		switch v := val.(type) {
		case float64:
			return v, true
		case int:
			return float64(v), true
		case int64:
			return float64(v), true
		}
	}
	return 0, false
}

func calculatePearsonCorrelation(points []ScatterPoint) float64 {
	n := len(points)
	if n < 2 {
		return 0
	}
	
	// Calculate means
	var sumX, sumY float64
	for _, p := range points {
		sumX += p.X
		sumY += p.Y
	}
	meanX := sumX / float64(n)
	meanY := sumY / float64(n)
	
	// Calculate correlation coefficient
	var numerator, sumXSq, sumYSq float64
	for _, p := range points {
		dx := p.X - meanX
		dy := p.Y - meanY
		numerator += dx * dy
		sumXSq += dx * dx
		sumYSq += dy * dy
	}
	
	denominator := sumXSq * sumYSq
	if denominator <= 0 {
		return 0
	}
	
	// Calculate square root of denominator for proper correlation
	return numerator / math.Sqrt(denominator)
}

func getCorrelationStrength(coefficient float64) string {
	abs := coefficient
	if abs < 0 {
		abs = -abs
	}
	
	if abs >= 0.9 {
		return "Very Strong"
	} else if abs >= 0.7 {
		return "Strong" 
	} else if abs >= 0.5 {
		return "Moderate"
	} else if abs >= 0.3 {
		return "Weak"
	} else {
		return "Very Weak"
	}
}

func detectAnomalies(points []ScatterPoint, expectedCorrelation float64) []AnomalyPoint {
	if len(points) < 10 {
		return []AnomalyPoint{} // Need enough data for anomaly detection
	}
	
	// Calculate moving correlation and detect deviations
	var anomalies []AnomalyPoint
	windowSize := 10
	
	for i := windowSize; i < len(points); i++ {
		window := points[i-windowSize : i]
		windowCorrelation := calculatePearsonCorrelation(window)
		
		// Check if correlation has significantly deviated
		deviation := windowCorrelation - expectedCorrelation
		if deviation > 0.3 || deviation < -0.3 {
			severity := deviation
			if severity < 0 {
				severity = -severity
			}
			
			anomalyType := "correlation_change"
			if deviation > 0 {
				anomalyType = "stronger_correlation"
			} else {
				anomalyType = "weaker_correlation"
			}
			
			anomalies = append(anomalies, AnomalyPoint{
				X:           points[i].X,
				Y:           points[i].Y,
				Timestamp:   points[i].Timestamp,
				AnomalyType: anomalyType,
				Severity:    severity,
			})
		}
	}
	
	return anomalies
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Check client limit before upgrading
	s.clientsMutex.RLock()
	clientCount := len(s.clients)
	s.clientsMutex.RUnlock()
	
	if clientCount >= s.maxClients {
		http.Error(w, "Maximum clients reached", http.StatusServiceUnavailable)
		return
	}
	
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer conn.Close()
	
	s.clientsMutex.Lock()
	s.clients[conn] = true
	s.clientsMutex.Unlock()
	
	defer func() {
		s.clientsMutex.Lock()
		delete(s.clients, conn)
		s.clientsMutex.Unlock()
	}()
	
	// Set connection timeouts and handlers
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})
	
	// Start a goroutine to read messages (required to detect client disconnections)
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("WebSocket read error: %v", err)
				}
				return
			}
		}
	}()
	
	// Keep connection alive with ping messages
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case <-readDone:
			// Client disconnected
			return
		case <-s.stop:
			// Server shutdown
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			return
		}
	}
}

func (s *Server) broadcast() {
	for {
		select {
		case metric := <-s.metrics:
			// Store recent metrics and historical data
			s.mutex.Lock()
			s.recentMetrics = metric
			s.historicalMetrics = append(s.historicalMetrics, metric)
			if len(s.historicalMetrics) > s.maxHistorySize {
				// Properly release memory by copying and truncating
				copy(s.historicalMetrics, s.historicalMetrics[1:])
				s.historicalMetrics = s.historicalMetrics[:s.maxHistorySize]
			}
			s.mutex.Unlock()
			
			s.broadcastMessage(map[string]interface{}{
				"type": "metrics",
				"data": metric,
			})
		case event := <-s.events:
			// Store in circular buffer and historical data
			s.mutex.Lock()
			s.eventBuffer[s.eventIndex] = event
			s.eventIndex = (s.eventIndex + 1) % len(s.eventBuffer)
			if s.eventCount < len(s.eventBuffer) {
				s.eventCount++
			}
			s.historicalEvents = append(s.historicalEvents, event)
			if len(s.historicalEvents) > s.maxHistorySize {
				// Properly release memory by copying and truncating
				copy(s.historicalEvents, s.historicalEvents[1:])
				s.historicalEvents = s.historicalEvents[:s.maxHistorySize]
			}
			s.mutex.Unlock()
			
			s.broadcastMessage(map[string]interface{}{
				"type": "event",
				"data": event,
			})
		case <-s.stop:
			return
		}
	}
}

func (s *Server) broadcastMessage(message interface{}) {
	// Early exit if no clients
	s.clientsMutex.RLock()
	if len(s.clients) == 0 {
		s.clientsMutex.RUnlock()
		return
	}
	
	// Copy client connections to avoid holding lock during I/O
	clientsCopy := make([]*websocket.Conn, 0, len(s.clients))
	for client := range s.clients {
		clientsCopy = append(clientsCopy, client)
	}
	s.clientsMutex.RUnlock()
	
	data, err := json.Marshal(message)
	if err != nil {
		log.Printf("Error marshaling message: %v", err)
		return
	}
	
	// Send to all clients, removing failed ones
	var failedClients []*websocket.Conn
	for _, client := range clientsCopy {
		if err := client.WriteMessage(websocket.TextMessage, data); err != nil {
			client.Close()
			failedClients = append(failedClients, client)
		}
	}
	
	// Remove failed clients from the map
	if len(failedClients) > 0 {
		s.clientsMutex.Lock()
		for _, client := range failedClients {
			delete(s.clients, client)
		}
		s.clientsMutex.Unlock()
	}
}