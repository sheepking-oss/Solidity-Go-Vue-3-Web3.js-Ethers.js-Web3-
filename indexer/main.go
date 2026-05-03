package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"supply-chain-indexer/config"
	"supply-chain-indexer/database"
	"supply-chain-indexer/ethereum"
	"supply-chain-indexer/routes"
)

func main() {
	cfg := config.LoadConfig()

	if err := database.InitDB(cfg); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	indexer, err := ethereum.NewIndexer(cfg)
	if err != nil {
		log.Fatalf("Failed to create indexer: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := indexer.Start(ctx); err != nil {
			log.Printf("Indexer error: %v", err)
		}
	}()

	router := routes.SetupRouter()

	go func() {
		addr := fmt.Sprintf(":%d", cfg.ServerPort)
		log.Printf("Starting HTTP server on %s", addr)
		if err := router.Run(addr); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Println("Shutting down...")
	cancel()

	log.Println("Server stopped")
}
