// Package actions provides a pluggable action system for Descry rule triggers.
// When monitoring rules evaluate to true, they can execute actions like alerts,
// logging, dashboard events, or custom handlers.
//
// The action system is built around the ActionHandler interface which allows
// for extensible handling of different action types. Built-in handlers include:
//   - ConsoleAlertHandler: Prints alerts to stdout
//   - LogHandler: Writes to Go's standard logger
//   - DashboardHandler: Sends events to the web dashboard
//
// Example usage:
//
//	registry := actions.NewActionRegistry()
//	registry.RegisterHandler(actions.AlertAction, &actions.ConsoleAlertHandler{})
//
//	action := actions.Action{
//		Type: actions.AlertAction,
//		Message: "High memory usage detected",
//		RuleName: "memory_check",
//		Timestamp: time.Now(),
//	}
//	registry.ExecuteAction(action)
//
// Custom action handlers can be created by implementing the ActionHandler interface.
package actions

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// ActionType represents the different kinds of actions that can be triggered
type ActionType string

const (
	AlertAction     ActionType = "alert"
	LogAction       ActionType = "log"
	DashboardAction ActionType = "dashboard"
)

// Action represents an action to be executed when a rule triggers
type Action struct {
	// Type specifies which kind of action this is
	Type      ActionType
	// Message contains the action content (e.g., alert text)
	Message   string
	// Timestamp indicates when the action was triggered
	Timestamp time.Time
	// RuleName identifies which rule triggered this action
	RuleName  string
}

// ActionHandler is the interface that action processors must implement
// to handle specific types of actions when rules trigger
type ActionHandler interface {
	// Handle processes the given action and returns any error
	Handle(action Action) error
}

// ConsoleAlertHandler prints alert messages to stdout with timestamps
type ConsoleAlertHandler struct{}

func (h *ConsoleAlertHandler) Handle(action Action) error {
	timestamp := action.Timestamp.Format("15:04:05")
	fmt.Printf("[%s] ALERT [%s]: %s\n", timestamp, action.RuleName, action.Message)
	return nil
}

// LogHandler writes log messages using Go's standard logger
type LogHandler struct {
	logger *log.Logger
}

// NewLogHandler creates a new log handler with an optional custom logger.
// If logger is nil, the standard log package will be used.
func NewLogHandler(logger *log.Logger) *LogHandler {
	return &LogHandler{logger: logger}
}

func (h *LogHandler) Handle(action Action) error {
	if h.logger == nil {
		log.Printf("LOG [%s]: %s", action.RuleName, action.Message)
	} else {
		h.logger.Printf("LOG [%s]: %s", action.RuleName, action.Message)
	}
	return nil
}

// ActionRegistry manages action handlers and executes actions when triggered.
// Multiple handlers can be registered for the same action type.
type ActionRegistry struct {
	mu       sync.RWMutex
	handlers map[ActionType][]ActionHandler
}

func NewActionRegistry() *ActionRegistry {
	return &ActionRegistry{
		handlers: make(map[ActionType][]ActionHandler),
	}
}

func (r *ActionRegistry) RegisterHandler(actionType ActionType, handler ActionHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[actionType] = append(r.handlers[actionType], handler)
}

func (r *ActionRegistry) ExecuteAction(action Action) error {
	r.mu.RLock()
	handlers, exists := r.handlers[action.Type]
	if !exists {
		r.mu.RUnlock()
		return fmt.Errorf("no handlers registered for action type: %s", action.Type)
	}
	
	// Copy handlers to release lock quickly
	handlersCopy := make([]ActionHandler, len(handlers))
	copy(handlersCopy, handlers)
	r.mu.RUnlock()

	for _, handler := range handlersCopy {
		if err := handler.Handle(action); err != nil {
			return fmt.Errorf("handler error for %s: %w", action.Type, err)
		}
	}

	return nil
}

type DashboardHandler struct {
	sendEvent func(eventType, message, rule string, data interface{})
}

func NewDashboardHandler(sendEvent func(eventType, message, rule string, data interface{})) *DashboardHandler {
	return &DashboardHandler{sendEvent: sendEvent}
}

func (h *DashboardHandler) Handle(action Action) error {
	if h.sendEvent != nil {
		eventType := "alert"
		if action.Type == LogAction {
			eventType = "log"
		}
		h.sendEvent(eventType, action.Message, action.RuleName, nil)
	}
	return nil
}

func (r *ActionRegistry) CreateAction(actionType ActionType, message, ruleName string) Action {
	return Action{
		Type:      actionType,
		Message:   message,
		Timestamp: time.Now(),
		RuleName:  ruleName,
	}
}