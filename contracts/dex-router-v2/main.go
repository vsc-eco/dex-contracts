package main

import (
	sdk "dex-router-v2/sdk"
	"encoding/json"
	"strconv"
	"strings"

	. "dex-router-v2/router-internal"

	tinyjson "github.com/CosmWasm/tinyjson"
)

func main() {}

// Contract initialization
// Payload: version string (e.g. "1.0.0")
//
//go:wasmexport init
func Init(payload *string) *string {
	if payload == nil || *payload == "" {
		setStr(keyVersion, "1.0.0")
	} else {
		setStr(keyVersion, *payload)
	}
	return nil
}

// Register a token in the token registry. MUST be called before any pool can use the token.
// Payload: JSON {"name": "HBD", "chain": "HIVE"} or {"name": "BTC", "chain": "BTC"}
// Chains: HIVE (for HIVE/HBD native), MAGI (for MAGI native tokens), BTC, ETH, etc. (for mapped assets)
//
//go:wasmexport register_token
func RegisterToken(payload *string) *string {
	if payload == nil {
		return &[]string{"error", "payload required"}[1]
	}

	var params RegisterTokenParams
	if err := tinyjson.Unmarshal([]byte(*payload), &params); err != nil {
		return &[]string{"error", "invalid payload"}[1]
	}

	name := params.Name
	if name == "" {
		return &[]string{"error", "name required"}[1]
	}
	if params.Chain == "" {
		return &[]string{"error", "chain required"}[1]
	}

	// Validate chain - support HIVE, MAGI, and common mapped chains
	validChains := map[string]bool{
		chainHIVE: true, chainMAGI: true,
		"BTC": true, "ETH": true, "SOL": true, "SUI": true,
	}
	if !validChains[params.Chain] {
		return &[]string{"error", "unsupported chain: " + params.Chain}[1]
	}

	// Normalize name to uppercase for storage key consistency
	name = strings.ToUpper(name)

	if isAssetRegistered(name) {
		return &[]string{"error", "asset already registered"}[1]
	}

	info, err := json.Marshal(params.TokenInfo)
	if err != nil {
		return &[]string{"error", "error marshaling token info"}[1]
	}
	setStr(assetKey(name), string(info))
	updateChainsList(params.Chain)
	return nil
}

// Register a DEX contract for a pool. Both assets MUST be registered via register_token first.
// Payload: JSON {"asset0": "HBD", "asset1": "HIVE", "dex_contract_id": "dex-hbd-hive-123"}
//
//go:wasmexport register_pool
func RegisterPool(payload *string) *string {
	if payload == nil {
		return &[]string{"error", "payload required"}[1]
	}

	var params RegisterPoolParams
	if err := tinyjson.Unmarshal([]byte(*payload), &params); err != nil {
		return &[]string{"error", "invalid payload"}[1]
	}

	// Validate assets are different
	if params.Asset0 == params.Asset1 {
		return &[]string{"error", "assets must be different"}[1]
	}

	// Enforce: both assets must be registered before pool can use them
	if !isAssetRegistered(params.Asset0) {
		return &[]string{"error", "asset " + params.Asset0 + " not registered - call register_token first"}[1]
	}
	if !isAssetRegistered(params.Asset1) {
		return &[]string{"error", "asset " + params.Asset1 + " not registered - call register_token first"}[1]
	}

	// Normalize asset order (alphabetical)
	if params.Asset0 > params.Asset1 {
		params.Asset0, params.Asset1 = params.Asset1, params.Asset0
	}

	// Store pool mapping
	poolKey := poolKeyForAssets(params.Asset0, params.Asset1)
	setStr(poolKey, params.DexContractId)

	return nil
}

// Execute swap through router (2-step routing)
// Payload: JSON instruction as defined in schema
// {"type": "swap", "version": "1.0.0", "asset_in": "BTC", "asset_out": "HIVE", "amount_in": 100000, "min_amount_out": 90000, "recipient": "user123"}
//
//go:wasmexport execute
func Execute(payload *string) *string {
	if payload == nil {
		return &[]string{"error", "payload required"}[1]
	}

	var instruction DexInstruction
	if err := tinyjson.Unmarshal([]byte(*payload), &instruction); err != nil {
		return &[]string{"error", "invalid json payload"}[1]
	}

	// Validate required fields
	if instruction.Type == "" || instruction.Version == "" ||
		instruction.AssetIn == "" || instruction.AssetOut == "" ||
		instruction.Recipient == "" {
		return &[]string{"error", "missing required fields"}[1]
	}

	switch instruction.Type {
	case "swap":
		return executeSwap(instruction)
	case "deposit":
		return executeDeposit(instruction)
	case "withdrawal":
		return executeWithdrawal(instruction)
	default:
		return &[]string{"error", "unknown instruction type"}[1]
	}
}

// Execute swap operation with 2-step routing
func executeSwap(instruction DexInstruction) *string {
	// Try direct pool first
	directPoolId := findPool(instruction.AssetIn, instruction.AssetOut)
	if directPoolId != "" {
		return executeDirectSwap(directPoolId, instruction)
	}

	// Try two-hop swap via HBD
	if instruction.AssetIn != "HBD" && instruction.AssetOut != "HBD" {
		return executeTwoHopSwap(instruction)
	}

	return &[]string{"error", "no suitable pool found"}[1]
}

// Find pool by assets
func findPool(assetA, assetB string) string {
	// Normalize asset order
	if assetA > assetB {
		assetA, assetB = assetB, assetA
	}
	poolKey := poolKeyForAssets(assetA, assetB)
	return getStr(poolKey)
}

// Execute direct swap within a single DEX pool
func executeDirectSwap(dexContractId string, instruction DexInstruction) *string {
	// Prepare swap parameters for DEX contract
	swapParams := SwapParams{
		AssetIn:      instruction.AssetIn,
		AmountIn:     instruction.AmountIn,
		AssetOut:     instruction.AssetOut,
		MinAmountOut: instruction.MinAmountOut,
		Recipient:    instruction.Recipient,
		Beneficiary:  instruction.Beneficiary,
		RefBps:       instruction.RefBps,
	}

	swapPayload, err := tinyjson.Marshal(&swapParams)
	if err != nil {
		return &[]string{"error", "failed to marshal swap params"}[1]
	}

	// Clone user's intents into the call - intents don't pass through inter-contract calls by default,
	// so the dex can't spend user money without this. Temporary workaround until VSC passes intents.
	swapOpts := contractCallOptionsWithUserIntents()

	// Call DEX contract's swap method
	result := sdk.ContractCall(dexContractId, "swap", string(swapPayload), swapOpts)
	if result == nil {
		return &[]string{"error", "swap failed"}[1]
	}

	// Parse result to update cache
	var swapResult SwapResult
	if err := tinyjson.Unmarshal([]byte(*result), &swapResult); err == nil {
		// Update cached pool state
		setPoolState(dexContractId, swapResult.PoolState)
	}

	return result
}

// Execute two-hop swap via HBD with intent-based bag checking and proper failure handling
func executeTwoHopSwap(instruction DexInstruction) *string {
	// Find first pool: AssetIn -> HBD
	pool1Id := findPool(instruction.AssetIn, "HBD")
	if pool1Id == "" {
		return handleSwapFailure(
			instruction,
			0,
			"no pool found for first hop",
			instruction.AssetIn,
			uint64(instruction.AmountIn),
		)
	}

	// Find second pool: HBD -> AssetOut
	pool2Id := findPool("HBD", instruction.AssetOut)
	if pool2Id == "" {
		return handleSwapFailure(
			instruction,
			0,
			"no pool found for second hop",
			instruction.AssetIn,
			uint64(instruction.AmountIn),
		)
	}

	// Execute first swap: AssetIn -> HBD
	// Route intermediate HBD to this router contract
	firstSwapParams := SwapParams{
		AssetIn:      instruction.AssetIn,
		AmountIn:     instruction.AmountIn,
		AssetOut:     "HBD",
		Recipient:    sdk.GetEnv().ContractId, // Route to router
		MinAmountOut: nil,                     // Let DEX calculate
	}

	firstSwapPayload, err := tinyjson.Marshal(&firstSwapParams)
	if err != nil {
		return handleSwapFailure(
			instruction,
			0,
			"failed to marshal first swap params",
			instruction.AssetIn,
			uint64(instruction.AmountIn),
		)
	}

	// Clone user's intents - dex needs them to spend user funds (user->router->dex flow)
	opts := contractCallOptionsWithUserIntents()

	// Call first DEX contract - VSC will validate intents
	// If swap fails (slippage, insufficient liquidity, etc.), VSC rolls back automatically
	firstResult := sdk.ContractCall(pool1Id, "swap", string(firstSwapPayload), opts)
	if firstResult == nil {
		// First swap failed - VSC rolled back, return original asset via return_address
		return handleSwapFailure(
			instruction,
			1,
			"first hop swap failed",
			instruction.AssetIn,
			uint64(instruction.AmountIn),
		)
	}

	// Check if result is an error
	if len(*firstResult) > 0 {
		// Try to parse as error
		if len(*firstResult) >= 6 && (*firstResult)[:6] == `{"error"` {
			// First swap failed - return original asset
			return handleSwapFailure(
				instruction,
				1,
				"first hop failed: "+*firstResult,
				instruction.AssetIn,
				uint64(instruction.AmountIn),
			)
		}
	}

	// Parse swap result (includes pool state)
	var swapResult1 SwapResult
	if err := tinyjson.Unmarshal([]byte(*firstResult), &swapResult1); err != nil {
		// Parsing failed - might be error, return original asset
		return handleSwapFailure(
			instruction,
			1,
			"first hop failed: "+*firstResult,
			instruction.AssetIn,
			uint64(instruction.AmountIn),
		)
	}

	// Update cached pool state from result
	setPoolState(pool1Id, swapResult1.PoolState)

	// Execute second swap: HBD -> AssetOut
	// Use actual HBD received from first swap
	secondSwapParams := SwapParams{
		AssetIn:      "HBD",
		AmountIn:     int64(swapResult1.AmountOut),
		AssetOut:     instruction.AssetOut,
		Recipient:    instruction.Recipient,
		MinAmountOut: instruction.MinAmountOut, // User's slippage protection
	}

	secondSwapPayload, err := tinyjson.Marshal(&secondSwapParams)
	if err != nil {
		// First swap succeeded but second failed to marshal
		// Return intermediate HBD via return_address
		return handlePartialSwap(swapResult1.AmountOut, "HBD", instruction)
	}

	// Call second DEX contract (intents cloned above, reuse for second hop)
	secondResult := sdk.ContractCall(pool2Id, "swap", string(secondSwapPayload), opts)
	if secondResult == nil {
		// Second swap failed - return intermediate HBD
		return handlePartialSwap(swapResult1.AmountOut, "HBD", instruction)
	}

	// Check for error
	if len(*secondResult) > 0 && len(*secondResult) >= 6 && (*secondResult)[:6] == `{"error"` {
		// Second swap failed - return intermediate HBD
		return handlePartialSwap(swapResult1.AmountOut, "HBD", instruction)
	}

	// Parse second swap result
	var swapResult2 SwapResult
	if err := tinyjson.Unmarshal([]byte(*secondResult), &swapResult2); err != nil {
		return handlePartialSwap(swapResult1.AmountOut, "HBD", instruction)
	}

	// Update cached pool state
	setPoolState(pool2Id, swapResult2.PoolState)

	// Success - both swaps completed
	return nil
}

// Handle swap failure - return original asset via return_address
func handleSwapFailure(
	instruction DexInstruction,
	failedAtHop int,
	reason string,
	asset string,
	amount uint64,
) *string {
	// Create failure log
	failureLog := FailureLog{
		Reason:         reason,
		FailedAtHop:    failedAtHop,
		OriginalAsset:  asset,
		OriginalAmount: amount,
		ReturnAddress:  instruction.ReturnAddress,
		Timestamp:      sdk.GetEnv().Timestamp,
	}

	// Store failure log (for debugging/auditing)
	logFailure(failureLog)

	// If return_address is specified, return asset to that address
	if instruction.ReturnAddress != nil {
		return returnAssetToAddress(asset, amount, *instruction.ReturnAddress, failureLog)
	}

	// If no return_address, try to return to recipient (might be on Hive)
	// This handles the case where user didn't specify return_address but is on Hive
	return returnAssetToRecipient(asset, amount, instruction.Recipient, failureLog)
}

// Handle partial swap failure - return intermediate asset via return_address
func handlePartialSwap(intermediateAmount uint64, intermediateAsset string, instruction DexInstruction) *string {
	// Create failure log
	failureLog := FailureLog{
		Reason:             "second hop failed",
		FailedAtHop:        2,
		OriginalAsset:      instruction.AssetIn,
		OriginalAmount:     uint64(instruction.AmountIn),
		IntermediateAsset:  intermediateAsset,
		IntermediateAmount: intermediateAmount,
		ReturnAddress:      instruction.ReturnAddress,
		Timestamp:          sdk.GetEnv().Timestamp,
	}

	// Store failure log
	logFailure(failureLog)

	// Determine what asset to return based on return_address requirements
	assetToReturn := intermediateAsset
	amountToReturn := intermediateAmount

	// If return_address is specified, check if we need to convert back to original asset
	if instruction.ReturnAddress != nil {
		originalAssetChain := getAssetChain(instruction.AssetIn)
		returnChain := instruction.ReturnAddress.Chain

		// If the original asset is from a non-Hive chain, and return address is on that same chain,
		// we should try to swap HBD back to the original asset
		// This handles cases like: BTC -> HBD (success) -> HIVE (failed), return to BTC address
		if originalAssetChain != "HIVE" && returnChain != "" && returnChain == originalAssetChain {
			// Try to swap HBD back to original asset
			swapBackResult := trySwapBackToOriginal(
				intermediateAsset,
				intermediateAmount,
				instruction.AssetIn,
				instruction,
			)
			if swapBackResult != nil && swapBackResult.Success {
				// Successfully swapped back - we now have original asset in router contract
				assetToReturn = swapBackResult.Asset
				amountToReturn = swapBackResult.AmountOut
			}
			// If swap back failed, we'll return the intermediate asset (HBD) as fallback
			// The returnAssetToAddress function will handle cross-chain bridging if needed
		}
		// If return address is on Hive or empty, we can return HBD directly
		// If return address is on a different chain than the original asset, we return HBD
		// and let the bridge system handle cross-chain transfer
	}

	// Return asset via return_address
	if instruction.ReturnAddress != nil {
		return returnAssetToAddress(assetToReturn, amountToReturn, *instruction.ReturnAddress, failureLog)
	}

	// If no return_address, return to recipient
	return returnAssetToRecipient(assetToReturn, amountToReturn, instruction.Recipient, failureLog)
}

// Return asset to specified return address (handles cross-chain)
func returnAssetToAddress(asset string, amount uint64, returnAddr ReturnAddress, log FailureLog) *string {
	// Check if return address is on Hive
	if returnAddr.Chain == "HIVE" || returnAddr.Chain == "" {
		// Direct transfer on Hive
		transferAsset(returnAddr.Address, int64(amount), asset)
		return &[]string{"error", "swap failed, returned " + strconv.FormatUint(amount, 10) + " " + asset + " to " + returnAddr.Address}[1]
	}

	// Cross-chain return - need bridge integration
	// For now, store the return request for bridge to process
	// In production, this would trigger a bridge transaction
	storeCrossChainReturn(returnAddr, asset, amount, log)

	return &[]string{"error", "swap failed, cross-chain return initiated to " + returnAddr.Chain + ":" + returnAddr.Address}[1]
}

// Return asset to recipient (fallback if no return_address)
func returnAssetToRecipient(asset string, amount uint64, recipient string, log FailureLog) *string {
	// Try to transfer to recipient (might fail if recipient is not on Hive)
	transferAsset(recipient, int64(amount), asset)
	return &[]string{"error", "swap failed, returned " + strconv.FormatUint(amount, 10) + " " + asset + " to " + recipient}[1]
}

// Log failure for auditing
func logFailure(log FailureLog) {
	// Store failure log in contract state
	// Key: failure_log/{tx_id}
	env := sdk.GetEnv()
	logKey := "failure_log/" + env.TxId

	logBytes, err := tinyjson.Marshal(&log)
	if err == nil {
		setStr(logKey, string(logBytes))
	}
}

// Store cross-chain return request (for bridge to process)
func storeCrossChainReturn(returnAddr ReturnAddress, asset string, amount uint64, log FailureLog) {
	// Store return request in contract state
	// Bridge service will process these
	returnKey := "return_request/" + sdk.GetEnv().TxId

	returnReq := ReturnRequest{
		Chain:   returnAddr.Chain,
		Address: returnAddr.Address,
		Asset:   asset,
		Amount:  amount,
		Log:     log,
	}

	reqBytes, err := tinyjson.Marshal(&returnReq)
	if err == nil {
		setStr(returnKey, string(reqBytes))
	}
}

// Helper to transfer assets
func transferAsset(to string, amount int64, asset string) {
	sdk.HiveTransfer(sdk.Address(to), amount, sdk.Asset(asset))
}

// getAssetChain returns the blockchain chain for a given asset symbol
// This function is in utils.go - using it here for consistency
// Chains are determined dynamically from registrations, not hardcoded

// SwapBackResult contains the result of attempting to swap back to original asset
type SwapBackResult struct {
	Success   bool
	AmountOut uint64
	Asset     string
	Error     string
}

// trySwapBackToOriginal attempts to swap the intermediate asset back to the original asset
// Returns SwapBackResult with success status and amount received
func trySwapBackToOriginal(
	intermediateAsset string,
	intermediateAmount uint64,
	originalAsset string,
	instruction DexInstruction,
) *SwapBackResult {
	// Find pool for reverse swap: intermediateAsset -> originalAsset
	reversePoolId := findPool(intermediateAsset, originalAsset)
	if reversePoolId == "" {
		// No pool available for reverse swap
		return &SwapBackResult{
			Success: false,
			Error:   "no pool found to swap back to " + originalAsset,
		}
	}

	// Prepare reverse swap parameters
	reverseSwapParams := SwapParams{
		AssetIn:      intermediateAsset,
		AmountIn:     int64(intermediateAmount),
		AssetOut:     originalAsset,
		Recipient:    sdk.GetEnv().ContractId, // Route back to router temporarily
		MinAmountOut: nil,                     // No minimum - we're trying to recover
	}

	reverseSwapPayload, err := tinyjson.Marshal(&reverseSwapParams)
	if err != nil {
		return &SwapBackResult{
			Success: false,
			Error:   "failed to marshal reverse swap params",
		}
	}

	// Execute reverse swap (clone intents for dex to spend)
	reverseOpts := contractCallOptionsWithUserIntents()
	reverseResult := sdk.ContractCall(reversePoolId, "swap", string(reverseSwapPayload), reverseOpts)
	if reverseResult == nil {
		// Reverse swap failed
		return &SwapBackResult{
			Success: false,
			Error:   "reverse swap failed",
		}
	}

	// Check for error in result
	if len(*reverseResult) > 0 && len(*reverseResult) >= 6 && (*reverseResult)[:6] == `{"error"` {
		return &SwapBackResult{
			Success: false,
			Error:   "reverse swap failed: " + *reverseResult,
		}
	}

	// Parse swap result to update cache
	var swapResult SwapResult
	if err := tinyjson.Unmarshal([]byte(*reverseResult), &swapResult); err != nil {
		return &SwapBackResult{
			Success: false,
			Error:   "failed to parse reverse swap result",
		}
	}

	// Update cached pool state
	setPoolState(reversePoolId, swapResult.PoolState)

	// Success - we now have the original asset in the router contract
	return &SwapBackResult{
		Success:   true,
		AmountOut: swapResult.AmountOut,
		Asset:     originalAsset,
	}
}

// Execute deposit (add liquidity)
func executeDeposit(instruction DexInstruction) *string {
	// Find the pool
	poolId := findPool(instruction.AssetIn, instruction.AssetOut)
	if poolId == "" {
		return &[]string{"error", "pool not found"}[1]
	}

	// For deposit, we need amounts from metadata
	if instruction.Metadata == nil {
		return &[]string{"error", "deposit amounts required in metadata"}[1]
	}

	amt0Interface, ok := instruction.Metadata["amount0"]
	if !ok {
		return &[]string{"error", "amount0 required in metadata"}[1]
	}
	amt1Interface, ok := instruction.Metadata["amount1"]
	if !ok {
		return &[]string{"error", "amount1 required in metadata"}[1]
	}

	amt0Float, err := strconv.ParseFloat(amt0Interface, 64)
	if err != nil {
		return &[]string{"error", "amount0 must be number"}[1]
	}
	amt1Float, err := strconv.ParseFloat(amt1Interface, 64)
	if err != nil {
		return &[]string{"error", "amount1 must be number"}[1]
	}

	amt0U := uint64(amt0Float)
	amt1U := uint64(amt1Float)

	// Prepare add liquidity parameters
	addLiqParams := AddLiquidityParams{
		Amount0:   amt0U,
		Amount1:   amt1U,
		Recipient: instruction.Recipient,
	}

	addLiqPayload, err := tinyjson.Marshal(&addLiqParams)
	if err != nil {
		return &[]string{"error", "failed to marshal add liquidity params"}[1]
	}

	// Call DEX contract's add_liquidity method (clone intents for dex to spend user funds)
	addLiqOpts := contractCallOptionsWithUserIntents()
	result := sdk.ContractCall(poolId, "add_liquidity", string(addLiqPayload), addLiqOpts)
	if result == nil {
		return &[]string{"error", "add liquidity failed"}[1]
	}

	return result
}

// Execute withdrawal (remove liquidity)
func executeWithdrawal(instruction DexInstruction) *string {
	// Find the pool
	poolId := findPool(instruction.AssetIn, instruction.AssetOut)
	if poolId == "" {
		return &[]string{"error", "pool not found"}[1]
	}

	// For withdrawal, we need LP amount from metadata
	if instruction.Metadata == nil {
		return &[]string{"error", "lp_amount required in metadata"}[1]
	}

	lpAmountInterface, ok := instruction.Metadata["lp_amount"]
	if !ok {
		return &[]string{"error", "lp_amount required in metadata"}[1]
	}

	lpAmountFloat, err := strconv.ParseFloat(lpAmountInterface, 64)
	if err != nil {
		return &[]string{"error", "lp_amount must be number"}[1]
	}

	lpAmountU := uint64(lpAmountFloat)

	// Prepare remove liquidity parameters
	removeLiqParams := RemoveLiquidityParams{
		LpAmount:  lpAmountU,
		Recipient: instruction.Recipient,
	}

	removeLiqPayload, err := tinyjson.Marshal(&removeLiqParams)
	if err != nil {
		return &[]string{"error", "failed to marshal remove liquidity params"}[1]
	}

	// Call DEX contract's remove_liquidity method (clone intents for dex to spend)
	removeLiqOpts := contractCallOptionsWithUserIntents()
	result := sdk.ContractCall(poolId, "remove_liquidity", string(removeLiqPayload), removeLiqOpts)
	if result == nil {
		return &[]string{"error", "remove liquidity failed"}[1]
	}

	return result
}

// Query pool information via router
// Payload: JSON with assets {"asset0": "HBD", "asset1": "HIVE"}
//
//go:wasmexport get_pool
func GetPool(payload *string) *string {
	if payload == nil {
		return &[]string{"error", "payload required"}[1]
	}

	var params GetPoolParams
	if err := tinyjson.Unmarshal([]byte(*payload), &params); err != nil {
		return &[]string{"error", "invalid payload"}[1]
	}

	// Find pool
	poolId := findPool(params.Asset0, params.Asset1)
	if poolId == "" {
		return &[]string{"error", "pool not found"}[1]
	}

	// Query DEX contract for pool info
	result := sdk.ContractCall(poolId, "get_pool", "", nil)
	if result == nil {
		return &[]string{"error", "failed to query pool"}[1]
	}

	return result
}

// Query schema information - returns supported assets and chains
// Payload: empty or {"type": "schema"}
//
//go:wasmexport get_schema
func GetSchema(payload *string) *string {
	// Get all registered assets and their chains
	// For now, return a simplified schema
	// In production, this would query all registered assets

	chainsStr := getStr(keyChainsList)
	chains := []string{"BTC", "ETH", "SOL", chainHIVE, chainMAGI} // Default chains (HIVE, MAGI for native tokens)
	if chainsStr != "" {
		parts := splitChains(chainsStr)
		if len(parts) > 0 {
			chains = parts
		}
	}

	// Build schema response
	schema := SchemaReturn{
		SupportedChains:     chains,
		ReturnAddressChains: chains,
		Note:                "Schema is dynamically generated from registered pools. Use indexer API for complete schema.",
	}

	schemaBytes, err := tinyjson.Marshal(&schema)
	if err != nil {
		return &[]string{"error", "failed to marshal schema"}[1]
	}

	result := string(schemaBytes)
	return &result
}
