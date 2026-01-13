package main

import (
	sdk "dex/sdk"
	"math/bits"
	"strconv"

	tinyjson "github.com/CosmWasm/tinyjson"
)

func main() {}

// Contract initialization
// Payload: JSON with pool parameters
// {"asset0": "HBD", "asset1": "HIVE", "fee_bps": 8}
//
//go:wasmexport init
func Init(payload *string) *string {
	if payload == nil {
		return &[]string{"error", "payload required"}[1]
	}

	var params InitParams
	if err := tinyjson.Unmarshal([]byte(*payload), &params); err != nil {
		return &[]string{"error", "invalid payload"}[1]
	}

	// Validate assets are different
	if params.Asset0 == params.Asset1 {
		return &[]string{"error", "assets must be different"}[1]
	}

	// Default fee if not specified
	if params.FeeBps == 0 {
		params.FeeBps = defaultBaseFeeBps
	}

	// Initialize pool state
	setAsset0(params.Asset0)
	setAsset1(params.Asset1)
	setReserve0(0)
	setReserve1(0)
	setFee(params.FeeBps)
	setTotalLp(0)
	setUint(keyFee0, 0)
	setUint(keyFee1, 0)
	setStr(keyFeeLastClaim, sdk.GetEnv().Timestamp)

	return nil
}

// Swap tokens in this pool
// Payload: JSON with swap parameters
// {"asset_in": "HBD", "amount_in": 1000000, "asset_out": "HIVE", "min_amount_out": 900000, "recipient": "user123"}
//
//go:wasmexport swap
func Swap(payload *string) *string {
	if payload == nil {
		return &[]string{"error", "payload required"}[1]
	}

	var params SwapParams
	if err := tinyjson.Unmarshal([]byte(*payload), &params); err != nil {
		return &[]string{"error", "invalid payload"}[1]
	}

	// Validate required fields
	if params.AssetIn == "" || params.AssetOut == "" || params.Recipient == "" {
		return &[]string{"error", "missing required fields"}[1]
	}

	asset0 := getAsset0()
	asset1 := getAsset1()

	if asset0 == "" {
		return &[]string{"error", "pool not initialized"}[1]
	}

	r0 := getReserve0()
	r1 := getReserve1()
	feeBps := getFee()

	if r0 == 0 || r1 == 0 {
		return &[]string{"error", "pool has zero reserves"}[1]
	}

	// Determine swap direction and calculate output
	var amountOut uint64
	var inputAsset, outputAsset string
	var feeReserveKey string

	if asset0 == params.AssetIn && asset1 == params.AssetOut {
		// asset0 -> asset1
		inputAsset = asset0
		outputAsset = asset1
		feeReserveKey = keyFee0

		// Calculate output: dy = r1 - (r0 * r1) / (r0 + dx)
		dx := uint64(params.AmountIn) * (10000 - feeBps) / 10000 // Apply fee
		if dx == 0 {
			dx = 1
		}
		k := r0 * r1
		newR0 := r0 + dx
		amountOut = r1 - (k / newR0)

		// Update reserves
		setReserve0(newR0)
		setReserve1(r1 - amountOut)

	} else if asset1 == params.AssetIn && asset0 == params.AssetOut {
		// asset1 -> asset0
		inputAsset = asset1
		outputAsset = asset0
		feeReserveKey = keyFee1

		// Calculate output: dx = r0 - (r0 * r1) / (r1 + dy)
		dy := uint64(params.AmountIn)
		k := r0 * r1
		newR1 := r1 + dy
		amountOut = r0 - (k / newR1)

		// Update reserves
		setReserve1(newR1)
		setReserve0(r0 - amountOut)

	} else {
		return &[]string{"error", "invalid asset pair for pool"}[1]
	}

	// Apply slippage protection if specified
	if params.MinAmountOut != nil {
		if amountOut < uint64(*params.MinAmountOut) {
			return &[]string{"error", "slippage tolerance exceeded"}[1]
		}
	}

	// Draw input asset and transfer output asset
	drawAsset(int64(params.AmountIn), inputAsset)

	// Handle referral fees
	if params.Beneficiary != nil && params.RefBps != nil {
		refOut := amountOut * uint64(*params.RefBps) / 10000
		if refOut > 0 {
			if refOut >= amountOut {
				refOut = amountOut - 1
			}
			amountOut -= refOut
			transferAsset(*params.Beneficiary, int64(refOut), outputAsset)
		}
	}

	transferAsset(params.Recipient, int64(amountOut), outputAsset)

	// Accumulate fees
	if inputAsset == "HBD" {
		fee := uint64(params.AmountIn) - (uint64(params.AmountIn) * (10000 - feeBps) / 10000)
		if fee > 0 {
			currentFee := getUint(feeReserveKey)
			setUint(feeReserveKey, currentFee+fee)
		}
	}

	// Return swap result with current pool state
	result := SwapResult{
		AmountOut: amountOut,
		PoolState: PoolInfo{
			Asset0:   getAsset0(),
			Asset1:   getAsset1(),
			Reserve0: getReserve0(),
			Reserve1: getReserve1(),
			Fee:      getFee(),
			TotalLp:  getTotalLp(),
		},
	}

	resultBytes, err := tinyjson.Marshal(&result)
	if err != nil {
		return &[]string{"error", "serialization failed"}[1]
	}

	resultStr := string(resultBytes)
	return &resultStr
}

// Add liquidity to the pool
// Payload: JSON with deposit parameters
// {"amount0": 1000000, "amount1": 1000000, "recipient": "user123"}
//
//go:wasmexport add_liquidity
func AddLiquidity(payload *string) *string {
	if payload == nil {
		return &[]string{"error", "payload required"}[1]
	}

	var params AddLiquidityParams
	if err := tinyjson.Unmarshal([]byte(*payload), &params); err != nil {
		return &[]string{"error", "invalid payload"}[1]
	}

	if params.Amount0 == 0 || params.Amount1 == 0 || params.Recipient == "" {
		return &[]string{"error", "missing required fields"}[1]
	}

	return executeAddLiquidity(params.Amount0, params.Amount1, params.Recipient)
}

// Remove liquidity from the pool
// Payload: JSON with withdrawal parameters
// {"lp_amount": 500000, "recipient": "user123"}
//
//go:wasmexport remove_liquidity
func RemoveLiquidity(payload *string) *string {
	if payload == nil {
		return &[]string{"error", "payload required"}[1]
	}

	var params RemoveLiquidityParams
	if err := tinyjson.Unmarshal([]byte(*payload), &params); err != nil {
		return &[]string{"error", "invalid payload"}[1]
	}

	if params.LpAmount == 0 || params.Recipient == "" {
		return &[]string{"error", "missing required fields"}[1]
	}

	return executeRemoveLiquidity(params.LpAmount, params.Recipient)
}

// Execute add liquidity operation
func executeAddLiquidity(amt0U, amt1U uint64, provider string) *string {
	asset0 := getAsset0()
	asset1 := getAsset1()

	// Pull funds from user intents into contract
	if amt0U > 0 {
		drawAsset(int64(amt0U), asset0)
	}
	if amt1U > 0 {
		drawAsset(int64(amt1U), asset1)
	}

	// Update reserves and mint LP
	r0 := getReserve0()
	r1 := getReserve1()
	totalLP := getTotalLp()

	var minted uint64
	if totalLP == 0 {
		// Geometric mean using 128-bit product for first liquidity
		hi, lo := bits.Mul64(amt0U, amt1U)
		minted = sqrt128(hi, lo)
	} else {
		// Proportional minting
		m0 := amt0U * totalLP / r0
		m1 := amt1U * totalLP / r1
		minted = min64(m0, m1)
	}
	contractAssert(minted > 0)

	// Update state
	setReserve0(r0 + amt0U)
	setReserve1(r1 + amt1U)
	setTotalLp(totalLP + minted)

	// Mint LP tokens to provider
	providerAddr := sdk.Address(provider)
	currentLP := getLp(providerAddr.String())
	setLp(providerAddr.String(), currentLP+minted)

	return nil
}

// Execute remove liquidity operation
func executeRemoveLiquidity(lpAmountU uint64, provider string) *string {
	providerAddr := sdk.Address(provider)
	userLP := getLp(providerAddr.String())
	totalLP := getTotalLp()

	contractAssert(lpAmountU > 0 && lpAmountU <= userLP && totalLP > 0)

	r0 := getReserve0()
	r1 := getReserve1()

	// Calculate proportional amounts
	amt0 := int64(r0 * lpAmountU / totalLP)
	amt1 := int64(r1 * lpAmountU / totalLP)

	// Update state first
	setLp(providerAddr.String(), userLP-lpAmountU)
	setTotalLp(totalLP - lpAmountU)
	setReserve0(r0 - uint64(amt0))
	setReserve1(r1 - uint64(amt1))

	// Transfer assets out
	asset0 := getAsset0()
	asset1 := getAsset1()
	if amt0 > 0 {
		transferAsset(provider, amt0, asset0)
	}
	if amt1 > 0 {
		transferAsset(provider, amt1, asset1)
	}

	return nil
}

// Query pool information
//
//go:wasmexport get_pool
func GetPool(payload *string) *string {
	asset0 := getAsset0()
	if asset0 == "" {
		return &[]string{"error", "pool not initialized"}[1]
	}

	poolInfo := PoolInfo{
		Asset0:   asset0,
		Asset1:   getAsset1(),
		Reserve0: getReserve0(),
		Reserve1: getReserve1(),
		Fee:      getFee(),
		TotalLp:  getTotalLp(),
	}

	resultBytes, err := tinyjson.Marshal(&poolInfo)
	if err != nil {
		return &[]string{"error", "serialization failed"}[1]
	}

	result := string(resultBytes)
	return &result
}

// Claim fees (system only)
//
//go:wasmexport claim_fees
func ClaimFees(payload *string) *string {
	if !isSystemSender() {
		return &[]string{"error", "system only"}[1]
	}

	asset0 := getAsset0()
	asset1 := getAsset1()
	dao := sdk.Address("system:fr_balance")

	f0 := getUint(keyFee0)
	f1 := getUint(keyFee1)

	if f0 > 0 && isHbd(asset0) {
		setUint(keyFee0, 0)
		sdk.HiveWithdraw(dao, int64(f0), sdk.Asset(asset0))
	}
	if f1 > 0 && isHbd(asset1) {
		setUint(keyFee1, 0)
		sdk.HiveWithdraw(dao, int64(f1), sdk.Asset(asset1))
	}

	setStr(keyFeeLastClaim, sdk.GetEnv().Timestamp)
	return nil
}
