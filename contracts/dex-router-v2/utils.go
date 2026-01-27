package main

import (
	sdk "dex-router-v2/sdk"
	"slices"
	"strconv"
	"strings"

	. "dex-router-v2/router-internal"

	"github.com/CosmWasm/tinyjson"
)

// Keys for state storage
const (
	keyVersion       = "version"
	keyPoolPrefix    = "pool/"           // pool/{asset0}/{asset1}
	keyStatePrefix   = "state/"          // state/{pool_id} - cached pool state
	keyFailurePrefix = "failure_log/"    // failure_log/{tx_id}
	keyReturnPrefix  = "return_request/" // return_request/{tx_id}
	keyAssetPrefix   = "asset/"          // asset/{symbol} -> AssetInfo
	keyChainsList    = "chains"          // comma-separated list of supported chains
)

// State helpers
func getStr(key string) string {
	v := sdk.StateGetObject(key)
	if v == nil {
		return ""
	}
	return *v
}

func setStr(key string, val string) {
	sdk.StateSetObject(key, val)
}

func getUint(key string) uint64 {
	v := sdk.StateGetObject(key)
	if v == nil {
		return 0
	}
	n, _ := strconv.ParseUint(*v, 10, 64)
	return n
}

func setUint(key string, val uint64) {
	sdk.StateSetObject(key, strconv.FormatUint(val, 10))
}

// Pool key helpers
func poolKeyForAssets(asset0, asset1 string) string {
	return keyPoolPrefix + strings.ToUpper(asset0) + "/" + strings.ToUpper(asset1)
}

func poolStateKey(poolId string) string {
	return keyStatePrefix + poolId
}

// Pool state cache helpers
func getPoolState(poolId string) PoolInfo {
	stateKey := poolStateKey(poolId)
	stateStr := getStr(stateKey)
	if stateStr == "" {
		return PoolInfo{} // Empty state
	}

	// Parse pool state from stored string
	// In a real implementation, we'd use tinyjson to unmarshal
	// For now, return empty - will be populated from swap results
	return PoolInfo{}
}

func setPoolState(poolId string, state PoolInfo) {
	stateKey := poolStateKey(poolId)
	// In a real implementation, we'd marshal state to JSON
	// For now, store as placeholder
	setStr(stateKey, "cached")
}

// Asset registry helpers for schema generation
func assetKey(asset string) string {
	return keyAssetPrefix + strings.ToUpper(asset)
}

// registerAssetForSchema registers an asset and its chain for schema generation
// Chain should be provided from mapping contracts or token registry
// This function stores the chain info but doesn't determine it
func registerAssetForSchema(asset string) {
	// If asset already registered, skip
	if getStr(assetKey(asset)) != "" {
		return
	}

	// Try to get chain - if not found, attempt to determine
	// In production, chain should come from:
	// 1. Mapping contract registration (utxo-mapping pattern)
	// 2. Token registry with chain metadata
	// 3. Pool registration payload with explicit chain
	chain := getAssetChain(asset)

	// Only register if we have a valid chain
	// Unknown assets will need to be registered via mapping contracts first
	if chain != "" {
		setStr(assetKey(asset), chain)
		updateChainsList(chain)
	}
	// If chain is empty, asset needs to be registered via mapping contract first
}

// updateChainsList adds a chain to the supported chains list if not already present
func updateChainsList(chain string) {
	chainsStr := getStr(keyChainsList)

	// Check if chain already in list
	// Simple check - in production, use proper parsing
	if len(chainsStr) > 0 {
		chains := strings.Split(chainsStr, ",")

		if !slices.Contains(chains, chain) {
			newChains := chainsStr + "," + chain
			setStr(keyChainsList, newChains)
		}
	} else {
		setStr(keyChainsList, chain)
	}
}

// getAssetChain returns the blockchain chain for a given asset symbol
// Chains are determined dynamically from registered assets, not hardcoded
// For cross-chain assets, chain info comes from mapping contracts (utxo-mapping pattern)
// For VSC native tokens, chain is "HIVE" (determined by checking if ContractId == nil in token registry)
// If asset not found, returns empty string (caller should handle)
func getAssetChain(asset string) string {
	// First, check if we've stored chain info for this asset
	infoStr := getStr(assetKey(asset))
	if infoStr == "" {
		return ""
	}
	var tokenInfo TokenInfo
	err := tinyjson.Unmarshal([]byte(infoStr), &tokenInfo)
	if err != nil {
		return ""
	}
	if tokenInfo.Chain != "" {
		return tokenInfo.Chain
	}

	// TODO: Query token registry to check if asset is native VSC token
	// In WASM contract, we could:
	// 1. Call token registry contract to check if ContractId == nil
	// 2. Store native asset list in contract state
	// 3. Receive chain info from pool registration

	// For now, return empty - chain must be provided via:
	// 1. Pool registration with explicit chain (asset0_chain, asset1_chain)
	// 2. Previous registration from mapping contract
	// 3. Token registry query (if we add contract call capability)

	// Unknown asset - return empty to indicate it needs registration
	// Caller should provide chain via pool registration or query token registry
	return ""
}
