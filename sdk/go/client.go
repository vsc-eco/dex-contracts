package vscdex

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	contracts "vsc-node/modules/db/vsc/contracts"
	transactionpool "vsc-node/modules/transaction-pool"

	"github.com/hasura/go-graphql-client"
)

// Intent represents a VSC transaction intent
type Intent struct {
	Type string            `json:"type"`
	Args map[string]string `json:"args"`
}

// Client provides SDK methods for VSC DEX mapping operations
type Client struct {
	config             Config
	transactionCreator *transactionpool.TransactionCrafter
}

type Config struct {
	Endpoint  string
	Username  string
	ActiveKey string
	Contracts ContractAddresses
}

type ContractAddresses struct {
	DexRouter string
}

// NewClient creates a new VSC DEX client
func NewClient(config Config) *Client {
	return &Client{
		config:             config,
		transactionCreator: nil, // TODO: Initialize TransactionCrafter if needed
	}
}

// ComputeDexRoute computes an optimal DEX swap route
func (c *Client) ComputeDexRoute(ctx context.Context, fromAsset, toAsset string, amount int64) (*RouteResult, error) {
	// Call router service HTTP API
	routerURL := fmt.Sprintf("%s/router/route", c.config.Endpoint)

	payload := map[string]interface{}{
		"fromAsset": fromAsset,
		"toAsset":   toAsset,
		"amount":    amount,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", routerURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to query router for route: %v", err)
		return nil, fmt.Errorf("failed to compute route: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("router returned status %d", resp.StatusCode)
	}

	// Parse response
	var result RouteResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse route response: %w", err)
	}

	return &result, nil
}

type RouteResult struct {
	AmountOut   int64    `json:"amount_out"`
	Route       []string `json:"route"`
	PriceImpact float64  `json:"price_impact"`
	Fee         int64    `json:"fee"`
}

// ExecuteDexSwap executes a computed DEX swap
func (c *Client) ExecuteDexSwap(ctx context.Context, route *RouteResult) error {
	// For the new unified contract, we need to construct a proper swap instruction
	// This is a simplified implementation - in practice, more parameters would be needed
	payload := fmt.Sprintf(`{
		"contract": "%s",
		"method": "execute",
		"args": {
			"type": "swap",
			"version": "1.0.0",
			"asset_in": "HBD",
			"asset_out": "HIVE",
			"recipient": "%s",
			"min_amount_out": %d
		}
	}`, c.config.Contracts.DexRouter, c.config.Username, route.AmountOut)

	return c.broadcastTx(ctx, payload)
}

// ExecuteDexOperation implements the router.DEXExecutor interface
// This executes operations on the unified DEX router contract
func (c *Client) ExecuteDexOperation(ctx context.Context, operationType string, payload string) error {
	return c.ExecuteDexOperationWithIntents(ctx, operationType, payload, nil)
}

// ExecuteDexOperationWithIntents implements the router.DEXExecutor interface
// This executes operations on the unified DEX router contract with intents
func (c *Client) ExecuteDexOperationWithIntents(ctx context.Context, operationType string, payload string, intents []Intent) error {
	// Call the unified DEX router contract
	payloadJSON := fmt.Sprintf(`{
		"contract": "%s",
		"method": "%s",
		"args": %s
	}`, c.config.Contracts.DexRouter, operationType, payload)

	return c.broadcastTxWithIntents(ctx, payloadJSON, intents)
}

// ExecuteDexSwapRouter implements the router.DEXExecutor interface
// This allows the SDK client to be injected into the router service
func (c *Client) ExecuteDexSwapRouter(ctx context.Context, amountOut int64, route []string, fee int64) error {
	// Create SDK RouteResult from parameters
	sdkRoute := &RouteResult{
		AmountOut:   amountOut,
		Route:       route,
		PriceImpact: 0, // TODO: Calculate price impact
		Fee:         fee,
	}

	return c.ExecuteDexSwap(ctx, sdkRoute)
}

// broadcastTx broadcasts a transaction to VSC
func (c *Client) broadcastTx(ctx context.Context, payload string) error {
	return c.broadcastTxWithIntents(ctx, payload, nil)
}

// broadcastTxWithIntents broadcasts a transaction to VSC with intents
func (c *Client) broadcastTxWithIntents(ctx context.Context, payload string, intents []Intent) error {
	// Parse the payload to extract contract call parameters
	var contractCall map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &contractCall); err != nil {
		return fmt.Errorf("failed to parse contract call payload: %w", err)
	}

	contractID, _ := contractCall["contract"].(string)
	method, _ := contractCall["method"].(string)
	args, _ := contractCall["args"].(map[string]interface{})

	// Serialize args to JSON string (VscContractCall.Payload is string, not map)
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return fmt.Errorf("failed to marshal contract call args: %w", err)
	}

	// Convert SDK intents to contracts intents
	var contractIntents []contracts.Intent
	if intents != nil {
		contractIntents = make([]contracts.Intent, len(intents))
		for i, intent := range intents {
			contractIntents[i] = contracts.Intent{
				Type: intent.Type,
				Args: intent.Args,
			}
		}
	}

	// Create VSC contract call transaction
	vscCall := &transactionpool.VscContractCall{
		Caller:     c.config.Username, // Use configured username as caller
		ContractId: contractID,
		RcLimit:    1000, // Default RC limit
		Action:     method,
		Payload:    string(argsJSON), // Payload must be JSON string
		Intents:    contractIntents,  // Add intents for protocol-level protection
		NetId:      "vsc-mainnet",
	}

	// Serialize the contract call
	op, err := vscCall.SerializeVSC()
	if err != nil {
		return fmt.Errorf("failed to serialize contract call: %w", err)
	}

	// Create VSC transaction
	tx := transactionpool.VSCTransaction{
		Ops:   []transactionpool.VSCTransactionOp{op},
		Nonce: 0, // TODO: Implement proper nonce management
	}

	// For now, create mock signed transaction
	// TODO: Implement proper transaction signing with active key
	mockTxBytes, _ := json.Marshal(tx)
	mockSigBytes := []byte("mock_signature_" + string(mockTxBytes))

	txStr := base64.StdEncoding.EncodeToString(mockTxBytes)
	sigStr := base64.StdEncoding.EncodeToString(mockSigBytes)

	// Create GraphQL client
	gqlClient := graphql.NewClient(c.config.Endpoint+"/api/v1/graphql", nil)

	// Execute GraphQL mutation
	var mutation struct {
		SubmitTransactionV1 struct {
			Id graphql.String `graphql:"id"`
		} `graphql:"submitTransactionV1(tx: $tx, sig: $sig)"`
	}

	err = gqlClient.Query(ctx, &mutation, map[string]interface{}{
		"tx":  graphql.String(txStr),
		"sig": graphql.String(sigStr),
	})

	if err != nil {
		log.Printf("Failed to broadcast transaction: %v", err)
		return fmt.Errorf("failed to broadcast transaction: %w", err)
	}

	log.Printf("Transaction broadcasted successfully, ID: %s", mutation.SubmitTransactionV1.Id)
	return nil
}

// GetPools queries available liquidity pools from indexer
func (c *Client) GetPools(ctx context.Context) ([]PoolInfo, error) {
	// Make HTTP call to indexer service
	indexerURL := fmt.Sprintf("%s/indexer/pools", c.config.Endpoint)

	req, err := http.NewRequestWithContext(ctx, "GET", indexerURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to query indexer for pools: %v", err)
		return nil, fmt.Errorf("failed to query pools: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("indexer returned status %d", resp.StatusCode)
	}

	// Parse response
	var pools []PoolInfo
	if err := json.NewDecoder(resp.Body).Decode(&pools); err != nil {
		return nil, fmt.Errorf("failed to parse pools response: %w", err)
	}

	return pools, nil
}

type PoolInfo struct {
	ID       string  `json:"id"`
	Asset0   string  `json:"asset0"`
	Asset1   string  `json:"asset1"`
	Reserve0 uint64  `json:"reserve0"`
	Reserve1 uint64  `json:"reserve1"`
	Fee      float64 `json:"fee"`
}
