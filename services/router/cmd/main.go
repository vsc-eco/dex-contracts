package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	// vscdex "github.com/vsc-eco/vsc-dex-mapping/sdk/go" // Temporarily commented out due to dependency issues
	"github.com/vsc-eco/vsc-dex-mapping/services/router"
)

// mockDEXExecutor implements DEXExecutor for when SDK is not available
type mockDEXExecutor struct{}

func (m *mockDEXExecutor) ExecuteDexOperation(ctx context.Context, operationType string, payload string) error {
	log.Printf("Mock DEXExecutor: Executing %s with payload %s", operationType, payload)
	return nil
}

func (m *mockDEXExecutor) ExecuteDexOperationWithIntents(ctx context.Context, operationType string, payload string, intents []router.Intent) error {
	log.Printf("Mock DEXExecutor: Executing %s with payload %s and %d intents", operationType, payload, len(intents))
	for i, intent := range intents {
		log.Printf("  Intent %d: %s with args %v", i, intent.Type, intent.Args)
	}
	return nil
}

func (m *mockDEXExecutor) ExecuteDexSwap(ctx context.Context, amountOut int64, route []string, fee int64) error {
	log.Printf("Mock DEXExecutor: Executing swap with amountOut %d, route %v, fee %d", amountOut, route, fee)
	return nil
}

func main() {
	var (
		vscNode         = flag.String("vsc-node", "http://localhost:4000", "VSC node GraphQL endpoint")
		vscKey          = flag.String("vsc-key", "", "VSC active key for transactions")
		vscUsername     = flag.String("vsc-username", "", "VSC username")
		port            = flag.String("port", "8080", "HTTP server port")
		indexerEndpoint = flag.String("indexer-endpoint", "http://localhost:8081", "Indexer service HTTP endpoint")
		dexRouter       = flag.String("dex-router-contract", "", "DEX router contract ID (router-v2)")
	)
	flag.Parse()

	config := router.VSCConfig{
		Endpoint:          *vscNode,
		Key:               *vscKey,
		Username:          *vscUsername,
		DexRouterContract: *dexRouter,
	}

	// Create SDK client to use as DEXExecutor
	// sdkClient := vscdex.NewClient(vscdex.Config{
	// 	Endpoint: *vscNode,
	// 	Username: *vscUsername,
	// 	ActiveKey: *vscKey,
	// 	Contracts: vscdex.ContractAddresses{
	// 		DexRouter: *dexRouter,
	// 	},
	// })

	// For now, create a mock executor since SDK has dependency issues
	mockExecutor := &mockDEXExecutor{}

	svc := router.NewService(config, mockExecutor)

	// Connect router to indexer for real-time pool data
	if *indexerEndpoint != "" {
		// Indexer integration can be added here when needed
		log.Printf("Indexer endpoint provided: %s (integration pending)", *indexerEndpoint)
	} else {
		log.Printf("Warning: No indexer endpoint provided, router will use hardcoded fallback pools")
	}

	server := router.NewServer(svc, *port)

	// Handle graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("Starting router service on port %s...", *port)
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Server failed to start:", err)
		}
	}()

	<-c
	log.Println("Shutting down router service...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Stop(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	log.Println("Router service stopped")
}
