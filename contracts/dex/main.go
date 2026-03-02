package main

import (
	sdk "dex/sdk"
	"errors"
	"fmt"
	"math/big"
	"math/bits"
	"strings"

	. "dex/dex-internal"

	ce "dex/contracterrors"

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
		sdk.Abort("payload required")
	}

	var params InitParams
	if err := tinyjson.Unmarshal([]byte(*payload), &params); err != nil {
		sdk.Revert("invalid payload", JSON_ERROR)
	}

	// Validate assets are different
	if params.Asset0 == params.Asset1 {
		sdk.Abort("assets must be different")
	}

	asset0, err := NewAsset(params.Asset0, params.Asset0MappingContract)
	if err != nil {
		sdk.Abort(err.Error())
	}
	asset1, err := NewAsset(params.Asset1, params.Asset1MappingContract)
	if err != nil {
		sdk.Abort(err.Error())
	}

	asset0json, err := tinyjson.Marshal(asset0)
	if err != nil {
		sdk.Revert(fmt.Sprintf("invalid asset, error: %s", err.Error()), JSON_ERROR)
	}
	asset1json, err := tinyjson.Marshal(asset1)
	if err != nil {
		sdk.Revert(fmt.Sprintf("invalid asset, error: %s", err.Error()), JSON_ERROR)
	}

	// Default fee if not specified
	if params.FeeBps == 0 {
		params.FeeBps = defaultBaseFeeBps
	}

	// Initialize pool state
	setAsset0(string(asset0json))
	setAsset1(string(asset1json))
	setReserve0(0)
	setReserve1(0)
	setFee(params.FeeBps)
	setTotalLp(0)
	setUint(keySystemFee0, 0)
	setUint(keySystemFee1, 0)
	setStr(keyFeeLastClaim, sdk.GetEnv().Timestamp)

	return nil
}

func getClpFee(amountIn int64, reserveIn, reserveOut uint64) (uint64, error) {
	rIn := new(big.Int).SetUint64(reserveIn)
	rOut := new(big.Int).SetUint64(reserveOut)
	amtIn := big.NewInt(amountIn)
	clpFee := big.NewInt(0)
	clpFee.Mul(amtIn, amtIn)
	clpFee.Mul(clpFee, rOut)
	denominator := big.NewInt(0)
	denominator.Add(amtIn, rIn)
	denominator.Mul(denominator, denominator)
	clpFee.Div(clpFee, denominator)
	if !clpFee.IsUint64() {
		return 0, errors.New("clp fee out of bounds")
	}
	return clpFee.Uint64(), nil
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

	params.AssetIn = strings.ToLower(params.AssetIn)
	params.AssetOut = strings.ToLower(params.AssetOut)

	asset0, err := getAsset0()
	if err != nil {
		sdk.Abort("pool not initialized")
	}
	asset1, err := getAsset1()
	if err != nil {
		sdk.Abort("pool not initialized")
	}

	feeBps := getFee()

	// Determine swap direction and calculate output
	var amountOut uint64
	var inputAsset, outputAsset Asset
	var feeReserveKey string
	var rInKey, rOutKey string

	if asset0.Name() == params.AssetIn && asset1.Name() == params.AssetOut {
		// asset0 -> asset1
		inputAsset = asset0
		outputAsset = asset1
		feeReserveKey = keySystemFee0
		rInKey = keyReserve0
		rOutKey = keyReserve1
	} else if asset1.Name() == params.AssetIn && asset0.Name() == params.AssetOut {
		// asset1 -> asset0
		inputAsset = asset1
		outputAsset = asset0
		feeReserveKey = keySystemFee1
		rInKey = keyReserve1
		rOutKey = keyReserve0
	} else {
		return &[]string{"error", "invalid asset pair for pool want " + asset0.Name() + "-" + asset1.Name() + " got " + params.AssetIn + "-" + params.AssetOut}[1]
	}

	rIn := getUint(rInKey)
	rOut := getUint(rOutKey)

	if rIn == 0 || rOut == 0 {
		ce.CustomAbort(
			ce.NewContractError(ce.ErrInitialization, "pool has zero reserves"),
		)
	}

	dIn := uint64(params.AmountIn) * (10000 - feeBps) / 10000 // Apply fee
	if dIn == 0 {
		dIn = 1
	}

	// clpFee, err := getClpFee(params.AmountIn, rIn, rOut)
	// if err != nil {
	// 	ce.CustomAbort(
	// 		ce.WrapContractError(ce.ErrArithmetic, err, "error getting fee"),
	// 	)
	// }
	// dIn -= clpFee

	k := rIn * rOut
	newRIn := rIn + dIn
	amountOut = rOut - (k / newRIn)

	setUint(rInKey, newRIn)
	setUint(rOutKey, rOut-amountOut)

	// Apply slippage protection if specified
	if params.MinAmountOut != nil {
		if amountOut < uint64(*params.MinAmountOut) {
			return &[]string{"error", "slippage tolerance exceeded"}[1]
		}
	}

	maybeEnv := MaybeEnv{}

	inputAsset.DrawAsset(params.AmountIn, maybeEnv)

	// Handle referral fees
	if params.Beneficiary != nil && params.RefBps != nil {
		refOut := amountOut * uint64(*params.RefBps) / 10000
		if refOut > 0 {
			if refOut >= amountOut {
				refOut = amountOut - 1
			}
			amountOut -= refOut
			err := outputAsset.TransferAsset(*params.Beneficiary, int64(refOut))
			if err != nil {
				sdk.Abort(fmt.Sprintf("error transferring beneficiary asset out: %s", err.Error()))
			}
		}
	}

	outputAsset.TransferAsset(params.Recipient, int64(amountOut))

	// // Accumulate fees
	// if isHbd(inputAsset.Name()) {
	fee := uint64(params.AmountIn) - dIn
	if fee > 0 {
		currentFee := getUint(feeReserveKey)
		setUint(feeReserveKey, currentFee+fee)
	}
	// }

	// Return swap result with current pool state
	result := SwapResult{
		AmountOut: amountOut,
		PoolState: PoolInfo{
			Asset0:   asset0.Name(),
			Asset1:   asset1.Name(),
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
		sdk.Revert("payload required", "invalid_input_error")
	}

	var params AddLiquidityParams
	if err := tinyjson.Unmarshal([]byte(*payload), &params); err != nil {
		sdk.Revert("invalid payload: "+err.Error(), "invalid_input_error")
	}

	if params.Amount0 == 0 || params.Amount1 == 0 || params.Recipient == "" {
		sdk.Revert("missing required fields", "invalid_input_error")
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
	asset0, err := getAsset0()
	if err != nil {
		sdk.Abort("pool not initialized")
	}
	asset1, err := getAsset1()
	if err != nil {
		sdk.Abort("pool not initialized")
	}

	maybeEnv := MaybeEnv{}

	// Pull funds from user intents into contract
	if amt0U > 0 {
		asset0.DrawAsset(int64(amt0U), maybeEnv)
	}
	if amt1U > 0 {
		asset1.DrawAsset(int64(amt1U), maybeEnv)
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
	asset0, err := getAsset0()
	if err != nil {
		sdk.Abort(fmt.Sprintf("error retrieving asset: %s", err.Error()))
	}
	asset1, err := getAsset1()
	if err != nil {
		sdk.Abort(fmt.Sprintf("error retrieving asset: %s", err.Error()))
	}
	if amt0 > 0 {
		asset0.TransferAsset(provider, amt0)
	}
	if amt1 > 0 {
		asset1.TransferAsset(provider, amt1)
	}

	return nil
}

// Query pool information
//
//go:wasmexport get_pool
func GetPool(_ *string) *string {
	asset0, err := getAsset0()
	if err != nil {
		sdk.Abort("pool not initialized")
	}
	asset1, err := getAsset0()
	if err != nil {
		sdk.Abort("pool not initialized")
	}

	poolInfo := PoolInfo{
		Asset0:   asset0.Name(),
		Asset1:   asset1.Name(),
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

	asset0, err := getAsset0()
	if err != nil {
		sdk.Abort("pool not initialized")
	}
	asset1, err := getAsset1()
	if err != nil {
		sdk.Abort("pool not initialized")
	}
	dao := sdk.Address("system:fr_balance")

	f0 := getUint(keySystemFee0)
	f1 := getUint(keySystemFee1)

	// can use hive withdraw because its only hbd
	if f0 > 0 && isHbd(asset0.Name()) {
		setUint(keySystemFee0, 0)
		sdk.HiveWithdraw(dao, int64(f0), sdk.Asset(asset0.Name()))
	}
	if f1 > 0 && isHbd(asset1.Name()) {
		setUint(keySystemFee1, 0)
		sdk.HiveWithdraw(dao, int64(f1), sdk.Asset(asset1.Name()))
	}

	setStr(keyFeeLastClaim, sdk.GetEnv().Timestamp)
	return nil
}
