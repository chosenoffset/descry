package ledger

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

type Ledger struct {
	mu       sync.RWMutex
	accounts map[string]float64
}

func NewLedger() *Ledger {
	return &Ledger{
		accounts: make(map[string]float64),
	}
}

// CreateAccountRequest is the input for /account
type CreateAccountRequest struct {
	ID      string  `json:"id"`
	Balance float64 `json:"balance"`
}

func (l *Ledger) HandleCreateAccount(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CreateAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if _, exists := l.accounts[req.ID]; exists {
		http.Error(w, "account already exists", http.StatusConflict)
		return
	}

	l.accounts[req.ID] = req.Balance
	w.WriteHeader(http.StatusCreated)
}

func (l *Ledger) HandleGetBalance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	l.mu.RLock()
	defer l.mu.RUnlock()

	balance, ok := l.accounts[id]
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	fmt.Fprintf(w, "%.2f", balance)
}

type TransferRequest struct {
	From   string  `json:"from"`
	To     string  `json:"to"`
	Amount float64 `json:"amount"`
}

func (l *Ledger) HandleTransfer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req TransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if req.Amount <= 0 {
		http.Error(w, "invalid amount", http.StatusBadRequest)
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	fromBal, fromOk := l.accounts[req.From]
	toBal, toOk := l.accounts[req.To]

	if !fromOk || !toOk {
		http.Error(w, "invalid account(s)", http.StatusNotFound)
		return
	}

	if fromBal < req.Amount {
		http.Error(w, "insufficient funds", http.StatusBadRequest)
		return
	}

	l.accounts[req.From] = fromBal - req.Amount
	l.accounts[req.To] = toBal + req.Amount

	w.WriteHeader(http.StatusOK)
}
