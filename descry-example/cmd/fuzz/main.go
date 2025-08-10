// Package main provides a comprehensive load testing client for the Descry example application.
// It generates realistic traffic patterns to demonstrate monitoring capabilities under various conditions.
//
// The fuzzing client implements 9 different load scenarios:
//   1. Normal Operations: Regular account operations with typical load
//   2. Sustained Load: Continuous high-volume operations
//   3. Spike Load: Sudden traffic bursts to test response under pressure
//   4. Memory Pressure: Operations that create temporary memory allocation spikes
//   5. High Error Rate: Mix of valid and invalid operations
//   6. Concurrent Transfers: Heavy concurrent transfer operations
//   7. Large Payloads: Operations with oversized request bodies
//   8. Rapid Fire: Very high-frequency small operations
//   9. Mixed Workload: Combination of all patterns
//
// Each scenario is designed to trigger different types of monitoring rules
// and demonstrate the effectiveness of Descry's rule-based alerting system.
//
// Usage:
//   go run descry-example/cmd/fuzz/main.go
//
// The client will run all scenarios sequentially, each for 30 seconds,
// generating realistic load patterns that stress-test the monitored application.
// Monitor the results using the Descry dashboard at http://localhost:9090
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

type LoadPattern struct {
	Name        string
	Description string
	Execute     func(ctx context.Context, client *http.Client, baseURL string)
}

type AccountRequest struct {
	ID      string  `json:"id"`
	Balance float64 `json:"balance"`
}

type TransferRequest struct {
	From   string  `json:"from"`
	To     string  `json:"to"`
	Amount float64 `json:"amount"`
}

const maxAccounts = 1000

var (
	accountCounter = 0
	accountMutex   sync.Mutex
	createdAccounts = make([]string, 0, maxAccounts)
)

func main() {
	client := &http.Client{Timeout: 5 * time.Second}
	baseURL := "http://localhost:8080"
	ctx := context.Background()

	// Define load patterns that stress different aspects of the system
	patterns := []LoadPattern{
		{
			Name:        "Normal Operations",
			Description: "Regular account operations with typical load",
			Execute:     normalOperations,
		},
		{
			Name:        "Account Creation Burst",
			Description: "Rapid account creation to test memory allocation",
			Execute:     accountCreationBurst,
		},
		{
			Name:        "High Frequency Transfers",
			Description: "Many concurrent transfers to test performance",
			Execute:     highFrequencyTransfers,
		},
		{
			Name:        "Large Transfer Amounts",
			Description: "Transfers with large amounts to test precision",
			Execute:     largeTransfers,
		},
		{
			Name:        "Concurrent Balance Checks",
			Description: "Many simultaneous balance queries",
			Execute:     concurrentBalanceChecks,
		},
		{
			Name:        "Error Generation",
			Description: "Deliberately trigger error conditions",
			Execute:     errorGeneration,
		},
		{
			Name:        "Memory Pressure",
			Description: "Operations designed to increase memory usage",
			Execute:     memoryPressure,
		},
		{
			Name:        "Sustained Load",
			Description: "Consistent medium load to test stability",
			Execute:     sustainedLoad,
		},
		{
			Name:        "Spike Load",
			Description: "Sudden bursts of activity to test scalability",
			Execute:     spikeLoad,
		},
	}

	rand.Seed(time.Now().UnixNano())
	
	// Pre-create some accounts for testing
	log.Println("Pre-creating test accounts...")
	for i := 0; i < 20; i++ {
		createTestAccount(ctx, client, baseURL)
	}
	log.Printf("Created %d test accounts", len(createdAccounts))

	log.Println("Starting load generation...")
	log.Println("This will generate realistic load patterns to demonstrate Descry monitoring capabilities")
	log.Printf("Available patterns: %d", len(patterns))
	for i, pattern := range patterns {
		log.Printf("  %d. %s - %s", i+1, pattern.Name, pattern.Description)
	}
	
	// Run different load patterns in cycles
	for {
		pattern := patterns[rand.Intn(len(patterns))]
		log.Printf("Running pattern: %s - %s", pattern.Name, pattern.Description)
		
		// Run the pattern for a random duration
		duration := time.Duration(rand.Intn(30)+10) * time.Second
		timeout, cancel := context.WithTimeout(ctx, duration)
		
		pattern.Execute(timeout, client, baseURL)
		cancel()
		
		// Brief pause between patterns
		time.Sleep(time.Duration(rand.Intn(5)+2) * time.Second)
	}
}

func createTestAccount(ctx context.Context, client *http.Client, baseURL string) {
	accountMutex.Lock()
	accountCounter++
	accountID := fmt.Sprintf("account-%d", accountCounter)
	accountMutex.Unlock()
	
	// Generate and validate balance
	balance := float64(rand.Intn(10000) + 1000) // Random balance between 1000-11000
	balance = math.Max(0, balance) // Ensure non-negative
	if math.IsInf(balance, 0) || math.IsNaN(balance) {
		balance = 1000 // Fallback value
	}
	
	account := AccountRequest{
		ID:      accountID,
		Balance: balance,
	}
	
	if createAccount(ctx, client, baseURL, account) {
		accountMutex.Lock()
		if len(createdAccounts) >= maxAccounts {
			// Remove oldest 10% of accounts to prevent memory leak
			removeCount := maxAccounts / 10
			if removeCount < 1 {
				removeCount = 1
			}
			copy(createdAccounts, createdAccounts[removeCount:])
			createdAccounts = createdAccounts[:len(createdAccounts)-removeCount]
		}
		createdAccounts = append(createdAccounts, accountID)
		accountMutex.Unlock()
	}
}

func normalOperations(ctx context.Context, client *http.Client, baseURL string) {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			switch rand.Intn(4) {
			case 0:
				createTestAccount(ctx, client, baseURL)
			case 1:
				performRandomTransfer(ctx, client, baseURL)
			case 2, 3:
				checkRandomBalance(ctx, client, baseURL)
			}
		}
	}
}

func accountCreationBurst(ctx context.Context, client *http.Client, baseURL string) {
	// Create many accounts rapidly to stress memory allocation
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			createTestAccount(ctx, client, baseURL)
		}
	}
}

func highFrequencyTransfers(ctx context.Context, client *http.Client, baseURL string) {
	// Rapid transfers to test HTTP performance and response times
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			performRandomTransfer(ctx, client, baseURL)
		}
	}
}

func largeTransfers(ctx context.Context, client *http.Client, baseURL string) {
	// Transfers with large amounts
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			performLargeTransfer(ctx, client, baseURL)
		}
	}
}

func concurrentBalanceChecks(ctx context.Context, client *http.Client, baseURL string) {
	// Many concurrent balance checks to test goroutine management
	var wg sync.WaitGroup
	
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()
			
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					checkRandomBalance(ctx, client, baseURL)
				}
			}
		}()
	}
	
	wg.Wait()
}

func errorGeneration(ctx context.Context, client *http.Client, baseURL string) {
	// Deliberately generate errors to test error rate monitoring
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			switch rand.Intn(4) {
			case 0:
				// Try to create duplicate account
				if len(createdAccounts) > 0 {
					existing := createdAccounts[rand.Intn(len(createdAccounts))]
					account := AccountRequest{ID: existing, Balance: 1000}
					createAccount(ctx, client, baseURL, account)
				}
			case 1:
				// Try to transfer from non-existent account
				transfer := TransferRequest{
					From:   "non-existent-account",
					To:     "another-non-existent",
					Amount: 100,
				}
				performTransfer(ctx, client, baseURL, transfer)
			case 2:
				// Check balance of non-existent account
				checkBalance(ctx, client, baseURL, "non-existent-account")
			case 3:
				// Try insufficient funds transfer
				performInsufficientFundsTransfer(ctx, client, baseURL)
			}
		}
	}
}

func memoryPressure(ctx context.Context, client *http.Client, baseURL string) {
	// Create operations that use more memory
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Create accounts with very long IDs and large amounts
			accountMutex.Lock()
			accountCounter++
			longID := fmt.Sprintf("very-long-account-id-with-lots-of-data-%d-%s", 
				accountCounter, randomString(50))
			accountMutex.Unlock()
			
			account := AccountRequest{
				ID:      longID,
				Balance: float64(rand.Intn(1000000) + 100000),
			}
			createAccount(ctx, client, baseURL, account)
		}
	}
}

func performRandomTransfer(ctx context.Context, client *http.Client, baseURL string) {
	accountMutex.Lock()
	if len(createdAccounts) < 2 {
		accountMutex.Unlock()
		return
	}
	
	fromIdx := rand.Intn(len(createdAccounts))
	toIdx := rand.Intn(len(createdAccounts))
	for toIdx == fromIdx {
		toIdx = rand.Intn(len(createdAccounts))
	}
	from := createdAccounts[fromIdx]
	to := createdAccounts[toIdx]
	accountMutex.Unlock()
	
	transfer := TransferRequest{
		From:   from,
		To:     to,
		Amount: float64(rand.Intn(500) + 1),
	}
	
	performTransfer(ctx, client, baseURL, transfer)
}

func performLargeTransfer(ctx context.Context, client *http.Client, baseURL string) {
	accountMutex.Lock()
	if len(createdAccounts) < 2 {
		accountMutex.Unlock()
		return
	}
	
	from := createdAccounts[rand.Intn(len(createdAccounts))]
	to := createdAccounts[rand.Intn(len(createdAccounts))]
	accountMutex.Unlock()
	
	transfer := TransferRequest{
		From:   from,
		To:     to,
		Amount: float64(rand.Intn(10000) + 5000), // Large amounts
	}
	
	performTransfer(ctx, client, baseURL, transfer)
}

func performInsufficientFundsTransfer(ctx context.Context, client *http.Client, baseURL string) {
	accountMutex.Lock()
	if len(createdAccounts) < 2 {
		accountMutex.Unlock()
		return
	}
	
	from := createdAccounts[rand.Intn(len(createdAccounts))]
	to := createdAccounts[rand.Intn(len(createdAccounts))]
	accountMutex.Unlock()
	
	// Try to transfer an impossibly large amount
	transfer := TransferRequest{
		From:   from,
		To:     to,
		Amount: 1000000000, // Very large amount likely to cause insufficient funds
	}
	
	performTransfer(ctx, client, baseURL, transfer)
}

func checkRandomBalance(ctx context.Context, client *http.Client, baseURL string) {
	accountMutex.Lock()
	if len(createdAccounts) == 0 {
		accountMutex.Unlock()
		return
	}
	
	account := createdAccounts[rand.Intn(len(createdAccounts))]
	accountMutex.Unlock()
	
	checkBalance(ctx, client, baseURL, account)
}

func createAccount(ctx context.Context, client *http.Client, baseURL string, account AccountRequest) bool {
	data, err := json.Marshal(account)
	if err != nil {
		return false
	}
	
	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/account", bytes.NewReader(data))
	if err != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	
	return resp.StatusCode == http.StatusCreated
}

func performTransfer(ctx context.Context, client *http.Client, baseURL string, transfer TransferRequest) {
	data, err := json.Marshal(transfer)
	if err != nil {
		return
	}
	
	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/transfer", bytes.NewReader(data))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
}

func checkBalance(ctx context.Context, client *http.Client, baseURL string, accountID string) {
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/balance?id="+accountID, nil)
	if err != nil {
		return
	}
	
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
}

func sustainedLoad(ctx context.Context, client *http.Client, baseURL string) {
	// Consistent medium load to test long-term stability
	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Mix of operations: 40% transfers, 30% balance checks, 20% account creation, 10% errors
			switch rand.Intn(10) {
			case 0, 1, 2, 3: // 40% transfers
				performRandomTransfer(ctx, client, baseURL)
			case 4, 5, 6: // 30% balance checks
				checkRandomBalance(ctx, client, baseURL)
			case 7, 8: // 20% account creation
				createTestAccount(ctx, client, baseURL)
			case 9: // 10% error generation
				// Generate a single error
				if len(createdAccounts) > 0 {
					checkBalance(ctx, client, baseURL, "non-existent-account")
				}
			}
		}
	}
}

func spikeLoad(ctx context.Context, client *http.Client, baseURL string) {
	// Sudden bursts of activity to test scalability and goroutine management
	for {
		select {
		case <-ctx.Done():
			return
		default:
			// Create a sudden spike of concurrent requests
			concurrency := rand.Intn(50) + 20 // 20-70 concurrent requests
			var wg sync.WaitGroup
			
			for i := 0; i < concurrency; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					switch rand.Intn(3) {
					case 0:
						performRandomTransfer(ctx, client, baseURL)
					case 1:
						checkRandomBalance(ctx, client, baseURL)
					case 2:
						createTestAccount(ctx, client, baseURL)
					}
				}()
			}
			
			// Wait for all requests to complete
			wg.Wait()
			
			// Pause between spikes
			pauseDuration := time.Duration(rand.Intn(3000)+1000) * time.Millisecond
			time.Sleep(pauseDuration)
		}
	}
}

func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}