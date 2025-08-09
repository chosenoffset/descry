package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
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
		clients:     make(map[*websocket.Conn]bool),
		maxClients:  100, // Limit concurrent WebSocket connections
		metrics:     make(chan MetricUpdate, 100),
		events:      make(chan EventUpdate, 100),
		stop:        make(chan struct{}),
		eventBuffer: make([]EventUpdate, 50), // Fixed-size circular buffer
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
	select {
	case s.events <- EventUpdate{
		Timestamp: time.Now(),
		Type:      eventType,
		Message:   message,
		Rule:      rule,
		Data:      data,
	}:
	default:
		// Drop if channel is full
	}
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
    </style>
</head>
<body>
    <div class="header">
        <h1>Descry Dashboard</h1>
        <p>Real-time application monitoring and rule engine</p>
    </div>
    
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
        
        // WebSocket message handling
        ws.onmessage = function(event) {
            const data = JSON.parse(event.data);
            
            if (data.type === 'metrics') {
                updateMetrics(data.data);
            } else if (data.type === 'event') {
                addEvent(data.data);
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
			// Store recent metrics
			s.mutex.Lock()
			s.recentMetrics = metric
			s.mutex.Unlock()
			
			s.broadcastMessage(map[string]interface{}{
				"type": "metrics",
				"data": metric,
			})
		case event := <-s.events:
			// Store in circular buffer
			s.mutex.Lock()
			s.eventBuffer[s.eventIndex] = event
			s.eventIndex = (s.eventIndex + 1) % len(s.eventBuffer)
			if s.eventCount < len(s.eventBuffer) {
				s.eventCount++
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