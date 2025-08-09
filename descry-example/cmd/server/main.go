package main

import (
	"log"
	"net/http"
	"time"

	"github.com/chosenoffset/descry/descry-example/internal/ledger"
)

func main() {
	l := ledger.NewLedger()

	mux := http.NewServeMux()
	mux.HandleFunc("/account", l.HandleCreateAccount)
	mux.HandleFunc("/balance", l.HandleGetBalance)
	mux.HandleFunc("/transfer", l.HandleTransfer)

	server := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Println("HTTP server listening on :8080")
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
