package indexer

import (
	"encoding/json"
	"sync"
)

// TransactionInfo represents a DEX transaction
type TransactionInfo struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"` // "swap", "deposit", "withdrawal"
	PoolID      string                 `json:"pool_id"`
	User        string                 `json:"user"`
	BlockHeight uint64                 `json:"block_height"`
	Timestamp   string                 `json:"timestamp"`
	Details     map[string]interface{} `json:"details"`
}

// LiquidityPosition represents a user's liquidity position in a pool
type LiquidityPosition struct {
	User   string  `json:"user"`
	PoolID string  `json:"pool_id"`
	Amount uint64  `json:"amount"`
	Share  float64 `json:"share"` // Percentage of total pool liquidity
}

// DexReadModel implements read model for DEX operations
type DexReadModel struct {
	mu                sync.RWMutex
	pools             map[string]PoolInfo
	transactions      []TransactionInfo
	positions         map[string][]LiquidityPosition // pool_id -> []positions
	dexContractToPool map[string]string              // dex_contract_id -> pool_id
	assets            map[string]string              // asset -> chain mapping for schema
	chains            map[string]bool               // set of supported chains
}

// NewDexReadModel creates a new DEX read model
func NewDexReadModel() *DexReadModel {
	// Initialize with only VSC native chain
	// Other chains will be added dynamically as pools are registered
	chains := make(map[string]bool)
	chains["HIVE"] = true // Only VSC native chain is known at init

	return &DexReadModel{
		pools:             make(map[string]PoolInfo),
		transactions:       make([]TransactionInfo, 0),
		positions:          make(map[string][]LiquidityPosition),
		dexContractToPool:  make(map[string]string),
		assets:             make(map[string]string),
		chains:             chains,
	}
}

// HandleEvent processes VSC events and updates read models
func (dm *DexReadModel) HandleEvent(event VSCEvent) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	// Handle events from router-v2, individual DEX contracts, or legacy dex-router
	switch {
	case event.Contract == "dex-router-v2" || event.Contract == "dex-router":
		return dm.handleRouterEvent(event)
	default:
		// Check if this is a DEX contract (contracts starting with "dex-")
		// or we can track it by checking if it has DEX methods
		return dm.handleDexContractEvent(event)
	}
}

// handleRouterEvent processes router-v2 events (pool registrations)
func (dm *DexReadModel) handleRouterEvent(event VSCEvent) error {
	// Extract common transaction info
	txInfo := TransactionInfo{
		ID:          event.TxID,
		BlockHeight: event.BlockHeight,
		Timestamp:   "", // Would need to be populated from block data
	}

	switch event.Method {
		case "register_pool":
		var args struct {
			Asset0        string  `json:"asset0"`
			Asset1        string  `json:"asset1"`
			DexContractId string  `json:"dex_contract_id"`
			Asset0Chain   *string `json:"asset0_chain,omitempty"` // Optional: from mapping contract
			Asset1Chain   *string `json:"asset1_chain,omitempty"` // Optional: from mapping contract
		}
		if err := json.Unmarshal(event.Args, &args); err != nil {
			return err
		}

		// Normalize asset order for pool ID
		asset0, asset1 := args.Asset0, args.Asset1
		if asset0 > asset1 {
			asset0, asset1 = asset1, asset0
		}
		poolID := asset0 + "-" + asset1

		// Register pool with DEX contract ID
		dm.pools[poolID] = PoolInfo{
			ID:          poolID,
			Asset0:      asset0,
			Asset1:      asset1,
			Fee:         0, // Will be updated when we query the DEX contract
			Reserve0:    0,
			Reserve1:    0,
			TotalSupply: 0,
		}

		// Store mapping from DEX contract ID to pool ID
		dm.dexContractToPool[args.DexContractId] = poolID

		// Register assets for schema generation
		// If chains provided in registration, use them; otherwise try to determine
		if args.Asset0Chain != nil {
			dm.assets[asset0] = *args.Asset0Chain
			dm.chains[*args.Asset0Chain] = true
		} else {
			dm.registerAsset(asset0)
		}

		if args.Asset1Chain != nil {
			dm.assets[asset1] = *args.Asset1Chain
			dm.chains[*args.Asset1Chain] = true
		} else {
			dm.registerAsset(asset1)
		}

		txInfo.Type = "pool_created"
		txInfo.PoolID = poolID
		txInfo.Details = map[string]interface{}{
			"asset0":         asset0,
			"asset1":         asset1,
			"dex_contract_id": args.DexContractId,
		}

		// Add transaction to history
		dm.transactions = append(dm.transactions, txInfo)
		if len(dm.transactions) > 1000 {
			dm.transactions = dm.transactions[1:]
		}

		return nil
	}

	// Fallback to legacy handler for old dex-router contract
	return dm.handleLegacyRouterEvent(event)
}

// handleDexContractEvent processes events from individual DEX contracts
func (dm *DexReadModel) handleDexContractEvent(event VSCEvent) error {
	// Find pool ID for this DEX contract
	poolID, exists := dm.dexContractToPool[event.Contract]
	if !exists {
		// Try to infer pool ID from contract ID (e.g., "dex-hbd-hive-123" -> "HBD-HIVE")
		// This is a fallback if we haven't seen a register_pool event yet
		return nil
	}

	// Extract common transaction info
	txInfo := TransactionInfo{
		ID:          event.TxID,
		BlockHeight: event.BlockHeight,
		Timestamp:   "",
		PoolID:      poolID,
	}

	switch event.Method {
	case "swap":
		var args struct {
			AmountIn  uint64 `json:"amount_in"`
			AmountOut uint64 `json:"amount_out"`
			AssetIn   string `json:"asset_in"`
			AssetOut   string `json:"asset_out"`
			Recipient string `json:"recipient"`
		}
		if err := json.Unmarshal(event.Args, &args); err != nil {
			return err
		}

		if pool, exists := dm.pools[poolID]; exists {
			// Update reserves based on swap direction
			if args.AssetIn == pool.Asset0 {
				pool.Reserve0 += args.AmountIn
				if pool.Reserve1 >= args.AmountOut {
					pool.Reserve1 -= args.AmountOut
				}
			} else {
				pool.Reserve1 += args.AmountIn
				if pool.Reserve0 >= args.AmountOut {
					pool.Reserve0 -= args.AmountOut
				}
			}
			dm.pools[poolID] = pool
		}

		txInfo.Type = "swap"
		txInfo.User = args.Recipient
		txInfo.Details = map[string]interface{}{
			"amount_in":  args.AmountIn,
			"amount_out": args.AmountOut,
			"asset_in":   args.AssetIn,
			"asset_out":  args.AssetOut,
		}

	case "add_liquidity":
		var args struct {
			Amount0   uint64 `json:"amount0"`
			Amount1   uint64 `json:"amount1"`
			Recipient string `json:"recipient"`
			LPTokens  uint64 `json:"lp_tokens,omitempty"`
		}
		if err := json.Unmarshal(event.Args, &args); err != nil {
			return err
		}

		if pool, exists := dm.pools[poolID]; exists {
			pool.Reserve0 += args.Amount0
			pool.Reserve1 += args.Amount1
			if args.LPTokens > 0 {
				pool.TotalSupply += args.LPTokens
			}
			dm.pools[poolID] = pool

			if args.Recipient != "" && args.LPTokens > 0 {
				dm.updateLiquidityPosition(poolID, args.Recipient, args.LPTokens, true)
			}
		}

		txInfo.Type = "deposit"
		txInfo.User = args.Recipient
		txInfo.Details = map[string]interface{}{
			"amount0":   args.Amount0,
			"amount1":   args.Amount1,
			"lp_tokens": args.LPTokens,
		}

	case "remove_liquidity":
		var args struct {
			LpAmount  uint64 `json:"lp_amount"`
			Recipient string `json:"recipient"`
			Amount0   uint64 `json:"amount0,omitempty"`
			Amount1   uint64 `json:"amount1,omitempty"`
		}
		if err := json.Unmarshal(event.Args, &args); err != nil {
			return err
		}

		if pool, exists := dm.pools[poolID]; exists {
			if args.Amount0 > 0 {
				if pool.Reserve0 >= args.Amount0 {
					pool.Reserve0 -= args.Amount0
				}
			}
			if args.Amount1 > 0 {
				if pool.Reserve1 >= args.Amount1 {
					pool.Reserve1 -= args.Amount1
				}
			}
			if pool.TotalSupply >= args.LpAmount {
				pool.TotalSupply -= args.LpAmount
			}
			dm.pools[poolID] = pool

			if args.Recipient != "" {
				dm.updateLiquidityPosition(poolID, args.Recipient, args.LpAmount, false)
			}
		}

		txInfo.Type = "withdrawal"
		txInfo.User = args.Recipient
		txInfo.Details = map[string]interface{}{
			"lp_amount": args.LpAmount,
			"amount0":   args.Amount0,
			"amount1":   args.Amount1,
		}
	}

	// Add transaction to history
	dm.transactions = append(dm.transactions, txInfo)
	if len(dm.transactions) > 1000 {
		dm.transactions = dm.transactions[1:]
	}

	return nil
}

// handleLegacyRouterEvent processes events from the old unified dex-router contract
func (dm *DexReadModel) handleLegacyRouterEvent(event VSCEvent) error {
	// Extract common transaction info
	txInfo := TransactionInfo{
		ID:          event.TxID,
		BlockHeight: event.BlockHeight,
		Timestamp:   "", // Would need to be populated from block data
	}

	// Handle pool creation, liquidity changes, and swaps from unified contract
	switch event.Method {
	case "pool_created":
		var args struct {
			PoolID string  `json:"pool_id"`
			Asset0 string  `json:"asset0"`
			Asset1 string  `json:"asset1"`
			Fee    float64 `json:"fee"`
		}
		if err := json.Unmarshal(event.Args, &args); err != nil {
			return err
		}

		dm.pools[args.PoolID] = PoolInfo{
			ID:       args.PoolID,
			Asset0:   args.Asset0,
			Asset1:   args.Asset1,
			Fee:      args.Fee,
			Reserve0: 0,
			Reserve1: 0,
		}

		txInfo.Type = "pool_created"
		txInfo.PoolID = args.PoolID
		txInfo.Details = map[string]interface{}{
			"asset0": args.Asset0,
			"asset1": args.Asset1,
			"fee":    args.Fee,
		}

	case "liquidity_added":
		var args struct {
			PoolID   string `json:"pool_id"`
			User     string `json:"user,omitempty"`
			Amount0  uint64 `json:"amount0"`
			Amount1  uint64 `json:"amount1"`
			LPTokens uint64 `json:"lp_tokens,omitempty"`
		}
		if err := json.Unmarshal(event.Args, &args); err != nil {
			return err
		}

		if pool, exists := dm.pools[args.PoolID]; exists {
			pool.Reserve0 += args.Amount0
			pool.Reserve1 += args.Amount1
			// Backward compatibility: if no lp_tokens specified, use amount0 as before
			lpTokens := args.LPTokens
			if lpTokens == 0 {
				lpTokens = args.Amount0 // Maintain old test behavior
			}
			pool.TotalSupply += lpTokens
			dm.pools[args.PoolID] = pool

			// Update liquidity position only if user is specified
			if args.User != "" {
				dm.updateLiquidityPosition(args.PoolID, args.User, lpTokens, true)
			}
		}

		txInfo.Type = "deposit"
		txInfo.PoolID = args.PoolID
		txInfo.User = args.User
		txInfo.Details = map[string]interface{}{
			"amount0":   args.Amount0,
			"amount1":   args.Amount1,
			"lp_tokens": args.LPTokens,
		}

	case "liquidity_removed":
		var args struct {
			PoolID   string `json:"pool_id"`
			User     string `json:"user"`
			Amount0  uint64 `json:"amount0"`
			Amount1  uint64 `json:"amount1"`
			LPTokens uint64 `json:"lp_tokens"`
		}
		if err := json.Unmarshal(event.Args, &args); err != nil {
			return err
		}

		if pool, exists := dm.pools[args.PoolID]; exists {
			pool.Reserve0 -= args.Amount0
			pool.Reserve1 -= args.Amount1
			pool.TotalSupply -= args.LPTokens
			dm.pools[args.PoolID] = pool

			// Update liquidity position
			dm.updateLiquidityPosition(args.PoolID, args.User, args.LPTokens, false)
		}

		txInfo.Type = "withdrawal"
		txInfo.PoolID = args.PoolID
		txInfo.User = args.User
		txInfo.Details = map[string]interface{}{
			"amount0":   args.Amount0,
			"amount1":   args.Amount1,
			"lp_tokens": args.LPTokens,
		}

	case "swap_executed":
		var args struct {
			PoolID    string `json:"pool_id"`
			User      string `json:"user,omitempty"`
			Amount0   int64  `json:"amount0,omitempty"` // Reserve delta for asset0 (backward compatibility)
			Amount1   int64  `json:"amount1,omitempty"` // Reserve delta for asset1 (backward compatibility)
			AmountIn  uint64 `json:"amount_in,omitempty"`
			AmountOut uint64 `json:"amount_out,omitempty"`
			AssetIn   string `json:"asset_in,omitempty"`
			AssetOut  string `json:"asset_out,omitempty"`
		}
		if err := json.Unmarshal(event.Args, &args); err != nil {
			return err
		}

		if pool, exists := dm.pools[args.PoolID]; exists {
			// Handle backward compatibility: if amount0/amount1 are provided, treat as deltas
			if args.Amount0 != 0 || args.Amount1 != 0 {
				pool.Reserve0 = uint64(int64(pool.Reserve0) + args.Amount0)
				pool.Reserve1 = uint64(int64(pool.Reserve1) + args.Amount1)
			} else {
				// New format: update reserves based on swap direction
				if args.AssetIn == pool.Asset0 {
					pool.Reserve0 += args.AmountIn
					pool.Reserve1 -= args.AmountOut
				} else {
					pool.Reserve1 += args.AmountIn
					pool.Reserve0 -= args.AmountOut
				}
			}
			dm.pools[args.PoolID] = pool
		}

		txInfo.Type = "swap"
		txInfo.PoolID = args.PoolID
		txInfo.User = args.User
		txInfo.Details = map[string]interface{}{
			"amount0":    args.Amount0,
			"amount1":    args.Amount1,
			"amount_in":  args.AmountIn,
			"amount_out": args.AmountOut,
			"asset_in":   args.AssetIn,
			"asset_out":  args.AssetOut,
		}
	}

	// Add transaction to history (keep last 1000 transactions)
	dm.transactions = append(dm.transactions, txInfo)
	if len(dm.transactions) > 1000 {
		dm.transactions = dm.transactions[1:]
	}

	return nil
}

// QueryPools returns all indexed pools
func (dm *DexReadModel) QueryPools() ([]PoolInfo, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	pools := make([]PoolInfo, 0, len(dm.pools))
	for _, pool := range dm.pools {
		pools = append(pools, pool)
	}

	return pools, nil
}

// GetPool returns a specific pool by ID
func (dm *DexReadModel) GetPool(poolID string) (PoolInfo, bool) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	pool, exists := dm.pools[poolID]
	return pool, exists
}

// updateLiquidityPosition updates a user's liquidity position
func (dm *DexReadModel) updateLiquidityPosition(poolID, user string, amount uint64, isAdd bool) {
	positions := dm.positions[poolID]
	found := false

	for i, pos := range positions {
		if pos.User == user {
			if isAdd {
				pos.Amount += amount
			} else {
				if pos.Amount > amount {
					pos.Amount -= amount
				} else {
					pos.Amount = 0
				}
			}
			positions[i] = pos
			found = true
			break
		}
	}

	if !found && isAdd && amount > 0 {
		positions = append(positions, LiquidityPosition{
			User:   user,
			PoolID: poolID,
			Amount: amount,
		})
	}

	// Update shares for all positions in this pool
	totalLP := dm.pools[poolID].TotalSupply
	for i := range positions {
		if totalLP > 0 {
			positions[i].Share = float64(positions[i].Amount) / float64(totalLP) * 100
		} else {
			positions[i].Share = 0
		}
	}

	dm.positions[poolID] = positions
}

// QueryTransactions returns recent transactions with optional filtering
func (dm *DexReadModel) QueryTransactions(poolID string, txType string, limit int) ([]TransactionInfo, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	var filtered []TransactionInfo

	for i := len(dm.transactions) - 1; i >= 0; i-- {
		tx := dm.transactions[i]

		if poolID != "" && tx.PoolID != poolID {
			continue
		}
		if txType != "" && tx.Type != txType {
			continue
		}

		filtered = append(filtered, tx)
		if len(filtered) >= limit {
			break
		}
	}

	return filtered, nil
}

// GetTransaction returns a specific transaction by ID
func (dm *DexReadModel) GetTransaction(txID string) (TransactionInfo, bool) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	for _, tx := range dm.transactions {
		if tx.ID == txID {
			return tx, true
		}
	}
	return TransactionInfo{}, false
}

// QueryLiquidityPositions returns liquidity positions for a pool
func (dm *DexReadModel) QueryLiquidityPositions(poolID string) ([]LiquidityPosition, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	positions, exists := dm.positions[poolID]
	if !exists {
		return []LiquidityPosition{}, nil
	}

	// Return copy to avoid external modification
	result := make([]LiquidityPosition, len(positions))
	copy(result, positions)
	return result, nil
}

// QueryRichList returns top liquidity holders for a pool with pagination
func (dm *DexReadModel) QueryRichList(poolID string, offset, limit int) ([]LiquidityPosition, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	positions, exists := dm.positions[poolID]
	if !exists {
		return []LiquidityPosition{}, nil
	}

	// Sort by amount descending (simple sort for demo)
	for i := 0; i < len(positions)-1; i++ {
		for j := i + 1; j < len(positions); j++ {
			if positions[i].Amount < positions[j].Amount {
				positions[i], positions[j] = positions[j], positions[i]
			}
		}
	}

	// Apply pagination
	start := offset
	end := offset + limit
	if start >= len(positions) {
		return []LiquidityPosition{}, nil
	}
	if end > len(positions) {
		end = len(positions)
	}

	result := make([]LiquidityPosition, end-start)
	copy(result, positions[start:end])
	return result, nil
}

// registerAsset registers an asset and determines its chain for schema generation
// Only registers if chain can be determined (from existing registry or VSC native)
// Cross-chain assets should be registered via mapping contracts first
func (dm *DexReadModel) registerAsset(asset string) {
	// Determine chain from asset symbol
	chain := dm.getAssetChain(asset)
	// Only register if we have a valid chain
	if chain != "" {
		dm.assets[asset] = chain
		dm.chains[chain] = true
	}
	// If chain is empty, asset needs to be registered via mapping contract first
}

// getAssetChain returns the blockchain chain for a given asset symbol
// Chains are determined dynamically from registered assets, not hardcoded
// For cross-chain assets, chain info should come from mapping contracts (utxo-mapping pattern)
// For VSC native tokens, chain is "HIVE" (determined by checking if ContractId == nil in token registry)
// If asset not found in registry, returns empty string
func (dm *DexReadModel) getAssetChain(asset string) string {
	// Check if we've already registered this asset
	if chain, exists := dm.assets[asset]; exists {
		return chain
	}

	// TODO: Query token registry to check if asset is native (ContractId == nil)
	// For now, we can't access token registry from indexer read model
	// In production, the indexer should query the token registry or receive this info
	// from pool registration events that include chain information
	
	// Unknown asset - return empty to indicate it needs registration
	// Chain info should come from:
	// 1. Pool registration with explicit chain
	// 2. Token registry query (if asset is native, ContractId == nil means chain = "HIVE")
	// 3. Mapping contract registration
	return ""
}

// GetSchema returns the current instruction schema based on registered assets
func (dm *DexReadModel) GetSchema() map[string]interface{} {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	// Collect all unique chains
	chains := make([]string, 0, len(dm.chains))
	for chain := range dm.chains {
		chains = append(chains, chain)
	}

	// Collect all registered assets
	assets := make([]string, 0, len(dm.assets))
	for asset := range dm.assets {
		assets = append(assets, asset)
	}

	return map[string]interface{}{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type":    "object",
		"required": []string{"type", "version", "asset_in", "asset_out", "recipient"},
		"properties": map[string]interface{}{
			"type": map[string]interface{}{
				"type": "string",
				"enum": []string{"swap", "deposit", "withdrawal"},
			},
			"version": map[string]interface{}{
				"type":    "string",
				"pattern": "^\\d+\\.\\d+\\.\\d+$",
			},
			"asset_in": map[string]interface{}{
				"type": "string",
				// Note: In production, could list registered assets here
			},
			"asset_out": map[string]interface{}{
				"type": "string",
				// Note: In production, could list registered assets here
			},
			"recipient": map[string]interface{}{
				"type": "string",
			},
			"slippage_bps": map[string]interface{}{
				"type":    "integer",
				"minimum": 0,
				"maximum": 10000,
			},
			"min_amount_out": map[string]interface{}{
				"type":    "integer",
				"minimum": 0,
			},
			"beneficiary": map[string]interface{}{
				"type": "string",
			},
			"ref_bps": map[string]interface{}{
				"type":    "integer",
				"minimum": 0,
				"maximum": 10000,
			},
			"return_address": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"chain": map[string]interface{}{
						"type": "string",
						"enum": chains, // Dynamically generated from registered pools
					},
					"address": map[string]interface{}{
						"type": "string",
					},
				},
				"required": []string{"chain", "address"},
			},
			"metadata": map[string]interface{}{
				"type": "object",
			},
		},
		"registered_assets": assets,
		"supported_chains": chains,
		"note": "This schema is dynamically generated based on registered DEX pools. Chains are automatically added when pools are registered.",
	}
}
