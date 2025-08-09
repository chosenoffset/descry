package main

import (
	"bytes"
	"context"
	"log"
	"math/rand"
	"net/http"
	"path/filepath"
	"plugin"
	"time"

	"github.com/chosenoffset/descry/descry-example/internal/scenario"
)

func loadPlugins(path string) []scenario.Scenario {
	var scenarios []scenario.Scenario
	files, err := filepath.Glob(filepath.Join(path, "*.so"))
	if err != nil {
		log.Fatalf("Failed to scan plugins: %v", err)
	}

	for _, f := range files {
		p, err := plugin.Open(f)
		if err != nil {
			log.Printf("Failed to load plugin %s: %v", f, err)
			continue
		}

		sym, err := p.Lookup("ScenarioInstance")
		if err != nil {
			log.Printf("Failed to find symbol in %s: %v", f, err)
			continue
		}

		sc, ok := sym.(scenario.Scenario)
		if !ok {
			log.Printf("Invalid type in %s", f)
			continue
		}

		log.Printf("Loaded scenario plugin: %s", sc.Name())
		scenarios = append(scenarios, sc)
	}

	return scenarios
}

func main() {
	client := &http.Client{Timeout: 5 * time.Second}
	baseURL := "http://localhost:8080"
	ctx := context.Background()

	scenarios := loadPlugins("./plugins")
	if len(scenarios) == 0 {
		log.Fatal("No scenarios loaded. Exiting.")
	}

	rand.Seed(time.Now().UnixNano())

	for {
		if rand.Intn(10) < 8 {
			sendRandomTransaction(ctx, client, baseURL)
		} else {
			sc := scenarios[rand.Intn(len(scenarios))]
			log.Printf("Running scenario: %s", sc.Name())
			sc.Run(ctx, client, baseURL)
		}

		time.Sleep(100 * time.Millisecond)
	}
}

func sendRandomTransaction(ctx context.Context, client *http.Client, baseURL string) {
	body := []byte(`{"txid": "tx-123", "amount": 100}`)
	req, _ := http.NewRequestWithContext(ctx, "POST", baseURL+"/ledger", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Normal transaction failed: %v", err)
		return
	}
	resp.Body.Close()
}
