package main

import (
	"encoding/json"
	"math/big"
	"strings"

	tinyjson "github.com/CosmWasm/tinyjson"

	ce "github.com/vsc-eco/dex-contracts/contracterrors"
	"github.com/vsc-eco/dex-contracts/contracts/asset"
	"github.com/vsc-eco/dex-contracts/contracts/types"
	"github.com/vsc-eco/dex-contracts/sdk"
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
		ce.CustomAbort(ce.NewContractError(ce.ErrInput, "payload required"))
	}

	var params types.RegisterTokenParams
	if err := tinyjson.Unmarshal([]byte(*payload), &params); err != nil {
		ce.CustomAbort(ce.WrapContractError(ce.ErrJson, err, "invalid payload"))
	}

	name := params.Name
	if name == "" {
		ce.CustomAbort(ce.NewContractError(ce.ErrInput, "name required"))
	}
	if params.Chain == "" {
		ce.CustomAbort(ce.NewContractError(ce.ErrInput, "chain required"))
	}

	// Validate chain - support HIVE, MAGI, and common mapped chains
	validChains := map[string]bool{
		chainHIVE: true, chainMAGI: true,
		"BTC": true, "ETH": true, "SOL": true, "SUI": true,
	}
	if !validChains[params.Chain] {
		ce.CustomAbort(ce.NewContractError(ce.ErrInput, "unsupported chain: "+params.Chain))
	}

	// Normalize name to lowercase for storage key consistency
	name = strings.ToLower(name)

	if isAssetRegistered(name) {
		ce.CustomAbort(ce.NewContractError(ce.ErrInput, "asset already registered"))
	}

	info, err := json.Marshal(params.TokenInfo)
	if err != nil {
		ce.CustomAbort(ce.NewContractError(ce.ErrJson, "error marshaling token info"))
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
		ce.CustomAbort(ce.NewContractError(ce.ErrInput, "payload required"))
	}

	var params types.RegisterPoolParams
	if err := tinyjson.Unmarshal([]byte(*payload), &params); err != nil {
		ce.CustomAbort(ce.WrapContractError(ce.ErrJson, err, "invalid payload"))
	}

	// Validate assets are different
	if params.Asset0 == params.Asset1 {
		ce.CustomAbort(ce.NewContractError(ce.ErrInput, "assets must be different"))
	}

	// Enforce: both assets must be registered before pool can use them
	if !isAssetRegistered(params.Asset0) {
		ce.CustomAbort(
			ce.NewContractError(
				ce.ErrInitialization,
				"asset "+params.Asset0+" not registered - call register_token first",
			),
		)
	}
	if !isAssetRegistered(params.Asset1) {
		ce.CustomAbort(
			ce.NewContractError(
				ce.ErrInitialization,
				"asset "+params.Asset1+" not registered - call register_token first",
			),
		)
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
		ce.CustomAbort(ce.NewContractError(ce.ErrInput, "payload required"))
	}

	var instruction types.DexInstruction
	if err := tinyjson.Unmarshal([]byte(*payload), &instruction); err != nil {
		ce.CustomAbort(ce.WrapContractError(ce.ErrJson, err, "invalid payload"))
	}

	// Validate required fields
	if instruction.Type == "" || instruction.Version == "" ||
		instruction.AssetIn == "" || instruction.AssetOut == "" ||
		instruction.Recipient == "" {
		ce.CustomAbort(ce.NewContractError(ce.ErrInput, "missing required fields"))
	}

	switch instruction.Type {
	case "swap":
		return executeSwap(instruction)
	case "deposit":
		return executeDeposit(instruction)
	case "withdrawal":
		return executeWithdrawal(instruction)
	default:
		ce.CustomAbort(ce.NewContractError(ce.ErrInput, "unknown instruction type"))
		return nil
	}
}

// Execute swap operation with 2-step routing
func executeSwap(instruction types.DexInstruction) *string {
	// Try direct pool first
	directPoolId := findPool(instruction.AssetIn, instruction.AssetOut)
	if directPoolId != "" {
		return executeDirectSwap(directPoolId, instruction)
	}

	// Try two-hop swap via HBD
	if strings.ToLower(instruction.AssetIn) != sdk.AssetHbd.String() &&
		strings.ToLower(instruction.AssetOut) != sdk.AssetHbd.String() {
		return executeTwoHopSwap(instruction)
	}

	ce.CustomAbort(
		ce.NewContractError(ce.ErrTransaction, "no suitable pool found"),
	)
	return nil
}

// Find pool by assets
func findPool(assetA, assetB string) string {
	// Normalize asset order
	astA := strings.ToLower(assetA)
	astB := strings.ToLower(assetB)
	if astA > astB {
		astA, astB = astB, astA
	}
	poolKey := poolKeyForAssets(astA, astB)
	poolVal := getStr(poolKey)
	return poolVal
}

// Execute direct swap within a single DEX pool
func executeDirectSwap(dexContractId string, instruction types.DexInstruction) *string {
	amountIn, ok := new(big.Int).SetString(instruction.AmountIn, 10)
	if !ok {
		ce.CustomAbort(ce.NewContractError(ce.ErrInput, "invalid input amount"))
	}
	poolAsset0, err := getPoolAsset(dexContractId, types.KeyAsset0)
	if err != nil {
		ce.CustomAbort(ce.WrapContractError(ce.ErrStateAccess, err, "could not retrieve pool asset0"))
	}
	poolAsset1, err := getPoolAsset(dexContractId, types.KeyAsset0)
	if err != nil {
		ce.CustomAbort(ce.WrapContractError(ce.ErrStateAccess, err, "could not retrieve pool asset1"))
	}
	var assetIn asset.Asset
	var mEnv types.MaybeEnv
	env := mEnv.UseEnv()
	switch instruction.AssetIn {
	case poolAsset0.Name():
		assetIn = poolAsset0
	case poolAsset1.Name():
		assetIn = poolAsset1
	default:
		ce.CustomAbort(ce.NewContractError(ce.ErrStateAccess, "cannot find input asset in pool"))
	}

	assetIn.DrawAssetFrom(amountIn, env.Caller, mEnv)

	// Prepare swap parameters for DEX contract
	swapParams := types.SwapParams{
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
		ce.CustomAbort(
			ce.WrapContractError(ce.ErrJson, err, "failed to marshal swap params"),
		)
	}

	options := sdk.ContractCallOptions{
		Intents: []sdk.Intent{
			{
				Type: types.IntentTransferType,
				Args: map[string]string{
					types.IntentAmountKey:     instruction.AmountIn,
					types.IntentContractIdKey: assetIn.MappingContract(),
					types.IntentTokenKey:      assetIn.Name(),
				},
			},
		},
	}

	// Call DEX contract's swap method
	result := sdk.ContractCall(dexContractId, "swap", string(swapPayload), &options)
	if result == nil {
		ce.CustomAbort(
			ce.NewContractError(ce.ErrTransaction, "unknown swap failure"),
		)
	}

	// Parse result to update cache
	var swapResult types.SwapResult
	if err := tinyjson.Unmarshal([]byte(*result), &swapResult); err == nil {
		// Update cached pool state
		// setPoolState(dexContractId, swapResult.PoolState)
	}

	return result
}

// Execute two-hop swap via HBD with intent-based bag checking and proper failure handling
func executeTwoHopSwap(instruction types.DexInstruction) *string {
	pool1Id := findPool(instruction.AssetIn, sdk.AssetHbd.String())
	if pool1Id == "" {
		ce.CustomAbort(ce.NewContractError(ce.ErrInitialization, "no pool found for first hop"))
	}

	// Find second pool: HBD -> AssetOut
	pool2Id := findPool(sdk.AssetHbd.String(), instruction.AssetOut)
	if pool2Id == "" {
		ce.CustomAbort(ce.NewContractError(ce.ErrInitialization, "no pool found for first hop"))
	}

	amountIn, ok := new(big.Int).SetString(instruction.AmountIn, 10)
	if !ok {
		ce.CustomAbort(ce.NewContractError(ce.ErrInput, "invalid input amount"))
	}
	poolAsset0, err := getPoolAsset(pool1Id, types.KeyAsset0)
	if err != nil {
		ce.CustomAbort(ce.WrapContractError(ce.ErrStateAccess, err, "could not retrieve pool asset 0"))
	}
	poolAsset1, err := getPoolAsset(pool1Id, types.KeyAsset1)
	if err != nil {
		ce.CustomAbort(ce.WrapContractError(ce.ErrStateAccess, err, "could not retrieve pool asset 1"))
	}
	var assetIn asset.Asset
	var mEnv types.MaybeEnv
	env := mEnv.UseEnv()
	switch instruction.AssetIn {
	case poolAsset0.Name():
		assetIn = poolAsset0
	case poolAsset1.Name():
		assetIn = poolAsset1
	default:
		ce.CustomAbort(ce.NewContractError(ce.ErrStateAccess, "cannot find input asset in pool"))
	}

	assetIn.DrawAssetFrom(amountIn, env.Caller, mEnv)

	// Execute first swap: AssetIn -> HBD
	// Route intermediate HBD to this router contract
	firstSwapParams := types.SwapParams{
		AssetIn:      instruction.AssetIn,
		AmountIn:     instruction.AmountIn,
		AssetOut:     sdk.AssetHbd.String(),
		Recipient:    "contract:" + mEnv.UseEnv().ContractId, // Route to router
		MinAmountOut: nil,                                    // Let DEX calculate
	}

	firstSwapPayload, err := tinyjson.Marshal(&firstSwapParams)
	if err != nil {
		ce.CustomAbort(ce.WrapContractError(ce.ErrJson, err, "error marshalling first hop params"))
	}

	options1 := sdk.ContractCallOptions{
		Intents: []sdk.Intent{
			{
				Type: types.IntentTransferType,
				Args: map[string]string{
					types.IntentAmountKey:     instruction.AmountIn,
					types.IntentContractIdKey: assetIn.MappingContract(),
					types.IntentTokenKey:      assetIn.Name(),
				},
			},
		},
	}

	// Call first DEX contract - VSC will validate intents
	// If swap fails (slippage, insufficient liquidity, etc.), VSC rolls back automatically
	firstResult := sdk.ContractCall(pool1Id, "swap", string(firstSwapPayload), &options1)
	if firstResult == nil {
		ce.CustomAbort(ce.NewContractError(ce.ErrTransaction, "first hop returned no result"))
	}

	// Parse swap result (includes pool state)
	var swapResult1 types.SwapResult
	if err := tinyjson.Unmarshal([]byte(*firstResult), &swapResult1); err != nil {
		ce.CustomAbort(ce.WrapContractError(ce.ErrJson, err, "error unmarshalling first hop result"))
	}
	res1AmountOut, ok := new(big.Int).SetString(swapResult1.AmountOut, 10)
	if !ok {
		ce.CustomAbort(ce.NewContractError(ce.ErrTransaction, "first hop returned non-integer amount out"))
	}

	// Update cached pool state from result
	// setPoolState(pool1Id, swapResult1.PoolState)

	// Execute second swap: HBD -> AssetOut
	// Use actual HBD received from first swap
	secondSwapParams := types.SwapParams{
		AssetIn:      sdk.AssetHbd.String(),
		AmountIn:     res1AmountOut.String(),
		AssetOut:     instruction.AssetOut,
		Recipient:    instruction.Recipient,
		MinAmountOut: instruction.MinAmountOut, // User's slippage protection
	}

	secondSwapPayload, err := tinyjson.Marshal(&secondSwapParams)
	if err != nil {
		ce.CustomAbort(ce.WrapContractError(ce.ErrJson, err, "error marshalling first hop params"))
	}

	options2 := sdk.ContractCallOptions{
		Intents: []sdk.Intent{
			{
				Type: types.IntentTransferType,
				Args: map[string]string{
					types.IntentAmountKey: instruction.AmountIn,
					types.IntentTokenKey:  sdk.AssetHbd.String(),
				},
			},
		},
	}

	// Call second DEX contract
	secondResult := sdk.ContractCall(pool2Id, "swap", string(secondSwapPayload), &options2)
	if secondResult == nil {
		// Second swap failed - return intermediate HBD
		ce.CustomAbort(ce.NewContractError(ce.ErrTransaction, "first hop returned no result"))
	}

	// Parse second swap result
	var swapResult2 types.SwapResult
	if err := tinyjson.Unmarshal([]byte(*secondResult), &swapResult2); err != nil {
		ce.CustomAbort(ce.WrapContractError(ce.ErrJson, err, "error unmarshalling first hop result"))
	}

	// Update cached pool state
	// setPoolState(pool2Id, swapResult2.PoolState)

	// Success - both swaps completed
	return nil
}

// getAssetChain returns the blockchain chain for a given asset symbol
// This function is in utils.go - using it here for consistency
// Chains are determined dynamically from registrations, not hardcoded

// SwapBackResult contains the result of attempting to swap back to original asset
type SwapBackResult struct {
	Success   bool
	AmountOut *big.Int
	Asset     string
	Error     string
}

// Execute deposit (add liquidity)
func executeDeposit(instruction types.DexInstruction) *string {
	// Find the pool
	poolId := findPool(instruction.AssetIn, instruction.AssetOut)
	if poolId == "" {
		ce.CustomAbort(ce.NewContractError(ce.ErrTransaction, "pool not found"))
	}

	// For deposit, we need amounts from metadata
	if instruction.Metadata == nil {
		ce.CustomAbort(ce.NewContractError(ce.ErrInput, "deposit amounts required in metadata"))
	}

	amt0, ok := instruction.Metadata["amount0"]
	if !ok {
		ce.CustomAbort(ce.NewContractError(ce.ErrInput, "amount0 required in metadata"))
	}
	amt1, ok := instruction.Metadata["amount1"]
	if !ok {
		ce.CustomAbort(ce.NewContractError(ce.ErrInput, "amount1 required in metadata"))
	}

	// amt0, ok := new(big.Int).SetString(amt0Interface, 10)
	// if !ok {
	// 	ce.CustomAbort(ce.NewContractError(ce.ErrInput, "amount0 must be number"))
	// }
	// amt1, ok := new(big.Int).SetString(amt1Interface, 10)
	// if !ok {
	// 	ce.CustomAbort(ce.NewContractError(ce.ErrInput, "amount1 must be number"))
	// }

	// Prepare add liquidity parameters
	addLiqParams := types.AddLiquidityParams{
		Amount0:   amt0,
		Amount1:   amt1,
		Recipient: instruction.Recipient,
	}

	addLiqPayload, err := tinyjson.Marshal(&addLiqParams)
	if err != nil {
		ce.CustomAbort(ce.NewContractError(ce.ErrJson, "failed to marshal add liquidity params"))
	}

	// Call DEX contract's add_liquidity method (clone intents for dex to spend user funds)
	result := sdk.ContractCall(poolId, "add_liquidity", string(addLiqPayload), &sdk.ContractCallOptions{})
	if result == nil {
		ce.CustomAbort(ce.NewContractError(ce.ErrTransaction, "add liquidity returned no result"))
	}

	return result
}

// Execute withdrawal (remove liquidity)
func executeWithdrawal(instruction types.DexInstruction) *string {
	// Find the pool
	poolId := findPool(instruction.AssetIn, instruction.AssetOut)
	if poolId == "" {
		ce.CustomAbort(ce.NewContractError(ce.ErrInitialization, "pool not found"))
	}

	// For withdrawal, we need LP amount from metadata
	if instruction.Metadata == nil {
		ce.CustomAbort(ce.NewContractError(ce.ErrInput, "lp_amount required in metadata"))
	}

	lpAmount, ok := instruction.Metadata["lp_amount"]
	if !ok {
		ce.CustomAbort(ce.NewContractError(ce.ErrInput, "lp_amount required in metadata"))
	}

	// Prepare remove liquidity parameters
	removeLiqParams := types.RemoveLiquidityParams{
		LpAmount:  lpAmount,
		Recipient: instruction.Recipient,
	}

	removeLiqPayload, err := tinyjson.Marshal(&removeLiqParams)
	if err != nil {
		ce.CustomAbort(ce.NewContractError(ce.ErrJson, "failed to marshal remove liquidity params"))
	}

	// Call DEX contract's remove_liquidity method (clone intents for dex to spend)
	result := sdk.ContractCall(poolId, "remove_liquidity", string(removeLiqPayload), &sdk.ContractCallOptions{})
	if result == nil {
		ce.CustomAbort(ce.NewContractError(ce.ErrTransaction, "remove liquidity failed"))
	}

	return result
}

// Query pool information via router
// Payload: JSON with assets {"asset0": "HBD", "asset1": "HIVE"}
//
//go:wasmexport get_pool
func GetPool(payload *string) *string {
	if payload == nil {
		ce.CustomAbort(ce.NewContractError(ce.ErrInput, "payload required"))
	}

	var params types.GetPoolParams
	if err := tinyjson.Unmarshal([]byte(*payload), &params); err != nil {
		ce.CustomAbort(ce.NewContractError(ce.ErrJson, "invalid payload"))
	}

	// Find pool
	poolId := findPool(params.Asset0, params.Asset1)
	if poolId == "" {
		ce.CustomAbort(ce.NewContractError(ce.ErrTransaction, "pool not found"))
	}

	// Query DEX contract for pool info
	result := sdk.ContractCall(poolId, "get_pool", "", nil)
	if result == nil {
		ce.CustomAbort(ce.NewContractError(ce.ErrTransaction, "failed to query pool"))
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
	schema := types.SchemaReturn{
		SupportedChains:     chains,
		ReturnAddressChains: chains,
		Note:                "Schema is dynamically generated from registered pools. Use indexer API for complete schema.",
	}

	schemaBytes, err := tinyjson.Marshal(&schema)
	if err != nil {
		ce.CustomAbort(ce.NewContractError(ce.ErrJson, "failed to marshal schema"))
	}

	result := string(schemaBytes)
	return &result
}
