package actions

import (
	"fmt"
	"log"
	"sync"
	"time"
)

type ActionType string

const (
	AlertAction     ActionType = "alert"
	LogAction       ActionType = "log"
	DashboardAction ActionType = "dashboard"
)

type Action struct {
	Type      ActionType
	Message   string
	Timestamp time.Time
	RuleName  string
}

type ActionHandler interface {
	Handle(action Action) error
}

type ConsoleAlertHandler struct{}

func (h *ConsoleAlertHandler) Handle(action Action) error {
	timestamp := action.Timestamp.Format("15:04:05")
	fmt.Printf("[%s] ALERT [%s]: %s\n", timestamp, action.RuleName, action.Message)
	return nil
}

type LogHandler struct {
	logger *log.Logger
}

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

func (r *ActionRegistry) CreateAction(actionType ActionType, message, ruleName string) Action {
	return Action{
		Type:      actionType,
		Message:   message,
		Timestamp: time.Now(),
		RuleName:  ruleName,
	}
}