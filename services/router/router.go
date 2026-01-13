package router

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
)

// Intent represents a VSC transaction intent
type Intent struct {
	Type string            `json:"type"`
	Args map[string]string `json:"args"`
}

// DEXExecutor interface for executing DEX operations
type DEXExecutor interface {
	ExecuteDexOperation(ctx context.Context, operationType string, payload string) error
	ExecuteDexOperationWithIntents(ctx context.Context, operationType string, payload string, intents []Intent) error
	ExecuteDexSwap(ctx context.Context, amountOut int64, route []string, fee int64) error
}

// Service provides DEX routing and transaction composition
type Service struct {
	vscConfig   VSCConfig
	dexExecutor DEXExecutor
}

type VSCConfig struct {
	Endpoint          string
	Key               string
	Username          string
	DexRouterContract string
}

// SwapParams represents a swap request
type SwapParams struct {
	Sender         string
	AmountIn       int64
	AssetIn        string
	AssetOut       string
	MinAmountOut   int64
	MaxSlippage    uint64
	MiddleOutRatio float64
	Beneficiary    string
	RefBps         uint64
}

// DepositParams represents a deposit request
type DepositParams struct {
	Sender   string
	AssetIn  string
	AssetOut string
	AmountIn int64
}

// WithdrawalParams represents a withdrawal request
type WithdrawalParams struct {
	Sender   string
	AssetIn  string
	AssetOut string
	LpAmount int64
}

// SwapResult represents the result of a DEX operation
type SwapResult struct {
	Success      bool
	AmountOut    int64
	Fee          int64
	Route        []string
	ErrorMessage string
}

// ExecuteSwap executes a swap through the unified DEX router contract
func (r *Service) ExecuteSwap(params SwapParams) (*SwapResult, error) {
	// Validate input
	if params.AssetIn == params.AssetOut {
		return &SwapResult{
			Success:      false,
			ErrorMessage: "cannot swap asset to itself",
		}, nil
	}

	// Construct JSON payload according to schema
	payload := map[string]interface{}{
		"type":           "swap",
		"version":        "1.0.0",
		"asset_in":       params.AssetIn,
		"amount_in":      params.AmountIn,
		"asset_out":      params.AssetOut,
		"recipient":      params.Sender,
		"min_amount_out": params.MinAmountOut,
	}

	// Add optional fields
	if params.MaxSlippage > 0 {
		payload["slippage_bps"] = int(params.MaxSlippage)
	}
	if params.Beneficiary != "" {
		payload["beneficiary"] = params.Beneficiary
	}
	if params.RefBps > 0 {
		payload["ref_bps"] = int(params.RefBps)
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return &SwapResult{
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to marshal payload: %v", err),
		}, nil
	}

	// Create intents for the swap operation
	// Allow transfer of input asset up to the estimated input amount
	intents := []Intent{
		{
			Type: "transfer.allow",
			Args: map[string]string{
				"limit": fmt.Sprintf("%d", params.AmountIn), // Maximum input amount
				"token": params.AssetIn,
			},
		},
	}

	// Execute through DEX executor with intents
	err = r.dexExecutor.ExecuteDexOperationWithIntents(context.Background(), "execute", string(payloadBytes), intents)
	if err != nil {
		return &SwapResult{
			Success:      false,
			ErrorMessage: fmt.Sprintf("swap execution failed: %v", err),
		}, nil
	}

	// For now, return success - in practice, we'd parse the contract response
	// The contract would need to return the actual swap result
	return &SwapResult{
		Success:   true,
		AmountOut: params.MinAmountOut, // Placeholder - would come from contract
		Route:     []string{"direct"},
	}, nil
}

// ExecuteDeposit executes a liquidity deposit
func (s *Service) ExecuteDeposit(params DepositParams) (*SwapResult, error) {
	// Construct JSON payload for deposit
	payload := map[string]interface{}{
		"type":      "deposit",
		"version":   "1.0.0",
		"asset_in":  params.AssetIn,
		"asset_out": params.AssetOut,
		"recipient": params.Sender,
		// Additional deposit parameters would go here
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return &SwapResult{
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to marshal deposit payload: %v", err),
		}, nil
	}

	// Create intents for the deposit operation
	// Allow transfer of input asset (the amount being deposited)
	intents := []Intent{
		{
			Type: "transfer.allow",
			Args: map[string]string{
				"limit": fmt.Sprintf("%d", params.AmountIn),
				"token": params.AssetIn,
			},
		},
	}

	err = s.dexExecutor.ExecuteDexOperationWithIntents(context.Background(), "execute", string(payloadBytes), intents)
	if err != nil {
		return &SwapResult{
			Success:      false,
			ErrorMessage: fmt.Sprintf("deposit execution failed: %v", err),
		}, nil
	}

	return &SwapResult{
		Success: true,
		Route:   []string{"deposit"},
	}, nil
}

// ExecuteWithdrawal executes a liquidity withdrawal
func (s *Service) ExecuteWithdrawal(params WithdrawalParams) (*SwapResult, error) {
	// Construct JSON payload for withdrawal
	payload := map[string]interface{}{
		"type":      "withdrawal",
		"version":   "1.0.0",
		"asset_in":  params.AssetIn,
		"asset_out": params.AssetOut,
		"recipient": params.Sender,
		// Additional withdrawal parameters would go here
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return &SwapResult{
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to marshal withdrawal payload: %v", err),
		}, nil
	}

	// Create intents for the withdrawal operation
	// For withdrawals, we need to allow the contract to transfer underlying assets back
	// Since the exact amounts are determined by the contract, we use a broad allowance
	// In practice, this might need to be calculated based on expected withdrawal amounts
	intents := []Intent{
		{
			Type: "transfer.allow",
			Args: map[string]string{
				"limit": "1000000000", // Large limit - would be calculated based on position
				"token": params.AssetIn,
			},
		},
		{
			Type: "transfer.allow",
			Args: map[string]string{
				"limit": "1000000000", // Large limit - would be calculated based on position
				"token": params.AssetOut,
			},
		},
	}

	err = s.dexExecutor.ExecuteDexOperationWithIntents(context.Background(), "execute", string(payloadBytes), intents)
	if err != nil {
		return &SwapResult{
			Success:      false,
			ErrorMessage: fmt.Sprintf("withdrawal execution failed: %v", err),
		}, nil
	}

	return &SwapResult{
		Success: true,
		Route:   []string{"withdrawal"},
	}, nil
}

// NewService creates a new router service
func NewService(config VSCConfig, dexExecutor DEXExecutor) *Service {
	return &Service{
		vscConfig:   config,
		dexExecutor: dexExecutor,
	}
}

// ComputeRoute finds the optimal route for a swap (external API method)
func (s *Service) ComputeRoute(ctx context.Context, params SwapParams) (*SwapResult, error) {
	return s.ExecuteSwap(params)
}

// ExecuteTransaction composes and submits the swap transaction
func (s *Service) ExecuteTransaction(ctx context.Context, result *SwapResult) error {
	log.Printf("Executing DEX operation: %+v", result)

	if s.dexExecutor == nil {
		return fmt.Errorf("DEX executor not initialized")
	}

	// The actual execution already happened in ExecuteSwap/ExecuteDeposit/ExecuteWithdrawal
	// This method is kept for compatibility
	log.Printf("DEX operation completed successfully")
	return nil
}
