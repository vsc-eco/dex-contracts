package main

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/vsc-eco/dex-contracts/contracts/asset"
	"github.com/vsc-eco/dex-contracts/contracts/types"
	"github.com/vsc-eco/dex-contracts/sdk"

	ce "github.com/vsc-eco/dex-contracts/contracterrors"

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

	var params types.InitParams
	if err := tinyjson.Unmarshal([]byte(*payload), &params); err != nil {
		sdk.Revert("invalid payload", JSON_ERROR)
	}

	// Validate assets are different
	if params.Asset0 == params.Asset1 {
		sdk.Abort("assets must be different")
	}

	asset0, err := asset.NewAsset(params.Asset0, params.Asset0MappingContract)
	if err != nil {
		sdk.Abort(err.Error())
	}
	asset1, err := asset.NewAsset(params.Asset1, params.Asset1MappingContract)
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
	if params.FeeBps > 10000 {
		ce.CustomAbort(
			ce.NewContractError(
				ce.ErrInput,
				"fee bps must not exceed 10000 got "+strconv.FormatUint(params.FeeBps, 10),
			),
		)
	}

	// Initialize pool state
	setAsset0(string(asset0json))
	setAsset1(string(asset1json))
	setReserve0(big.NewInt(0))
	setReserve1(big.NewInt(0))
	setFee(big.NewInt(int64(params.FeeBps)))
	setTotalLp(big.NewInt(0))
	setBigInt(keySystemFee0, big.NewInt(0))
	setBigInt(keySystemFee1, big.NewInt(0))
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
		ce.CustomAbort(
			ce.NewContractError(ce.ErrInput, "payload required"),
		)
	}

	var params types.SwapParams
	if err := tinyjson.Unmarshal([]byte(*payload), &params); err != nil {
		ce.CustomAbort(
			ce.WrapContractError(ce.ErrInput, err, "invalid payload"),
		)
	}

	// Validate required fields
	if params.AssetIn == "" || params.AssetOut == "" || params.To == "" {
		ce.CustomAbort(
			ce.NewContractError(ce.ErrInput, "missing required fields"),
		)
	}

	params.AssetIn = strings.ToLower(params.AssetIn)
	params.AssetOut = strings.ToLower(params.AssetOut)

	asset0, err := getAsset0()
	if err != nil {
		ce.CustomAbort(
			ce.NewContractError(ce.ErrInitialization, "asset0 not found"),
		)
	}
	asset1, err := getAsset1()
	if err != nil {
		ce.CustomAbort(
			ce.NewContractError(ce.ErrInitialization, "asset1 not found"),
		)
	}

	feeBps := getFee()

	// Determine swap direction and calculate output
	var inputAsset, outputAsset asset.Asset
	var magiFeeKey string
	var rInKey, rOutKey string

	if asset0.Name() == params.AssetIn && asset1.Name() == params.AssetOut {
		// asset0 -> asset1
		inputAsset = asset0
		outputAsset = asset1
		magiFeeKey = keySystemFee0
		rInKey = keyReserve0
		rOutKey = keyReserve1
	} else if asset1.Name() == params.AssetIn && asset0.Name() == params.AssetOut {
		// asset1 -> asset0
		inputAsset = asset1
		outputAsset = asset0
		magiFeeKey = keySystemFee1
		rInKey = keyReserve1
		rOutKey = keyReserve0
	} else {
		ce.CustomAbort(
			ce.NewContractError(ce.ErrInitialization, "invalid asset pair for pool want "+asset0.Name()+"-"+asset1.Name()+" got "+params.AssetIn+"-"+params.AssetOut),
		)
	}

	rIn := getBigInt(rInKey)
	rOut := getBigInt(rOutKey)

	if rIn.Sign() == 0 || rOut.Sign() == 0 {
		ce.CustomAbort(
			ce.NewContractError(ce.ErrTransaction, "pool has zero reserves"),
		)
	}
	maxSwap := new(big.Int).Div(rIn, big.NewInt(2))

	// baseFee = amountIn * feeBps / 10000
	amountIn, ok := new(big.Int).SetString(params.AmountIn, 10)
	if !ok {
		ce.CustomAbort(
			ce.NewContractError(ce.ErrInput, "invalid input amount"),
		)
	}
	if amountIn.Cmp(maxSwap) > 0 {
		ce.CustomAbort(
			ce.NewContractError(ce.ErrTransaction, "amount > input asset liquidty / 2", "swap too large"),
		)
	}

	baseFee := new(big.Int).Mul(amountIn, feeBps)
	baseFee.Div(baseFee, big.NewInt(10000))
	if baseFee.Sign() == 0 {
		baseFee.SetUint64(1)
	}

	// numerator = (amountIn ^ 2) * reserveOut
	numerator := new(big.Int).Mul(amountIn, amountIn)
	numerator.Mul(numerator, rOut)
	// denominator = (amountIn + reserveIn)^2
	denominator := new(big.Int).Add(amountIn, rIn)
	denominator.Mul(denominator, denominator)
	clpFee := new(big.Int).Div(numerator, denominator)
	if clpFee.Sign() == 0 {
		clpFee.SetUint64(1)
	}
	magiFee := new(big.Int).Add(baseFee, clpFee)
	lpFee := new(big.Int).Set(magiFee)
	magiFee.Div(magiFee, big.NewInt(4))
	if magiFee.Sign() == 0 {
		magiFee.SetUint64(1)
	}
	lpFee.Sub(lpFee, magiFee)

	// dIn = amountIn - baseFee - clpFee  (using big.Int to avoid underflow)
	dIn := new(big.Int).Set(amountIn)
	dIn.Sub(dIn, baseFee)
	dIn.Sub(dIn, clpFee)
	if dIn.Sign() <= 0 {
		ce.CustomAbort(
			ce.NewContractError(ce.ErrInitialization, "insufficient amount to cover fees"),
		)
	}

	// k = rIn * rOut  (big.Int to prevent overflow)
	k := new(big.Int).Mul(rIn, rOut)

	// newRIn = rIn + dIn
	newRIn := new(big.Int).Add(rIn, dIn)

	// amountOut = rOut - k / newRIn
	amountOut := new(big.Int).Div(k, newRIn) // (k / newRIn)
	amountOut.Sub(rOut, amountOut)

	// newROut = rOut - amountOut
	newROut := new(big.Int).Sub(rOut, amountOut)

	// Apply slippage protection if specified

	if params.MinAmountOut != nil {
		minOut, ok := new(big.Int).SetString(*params.MinAmountOut, 10)
		if !ok {
			ce.CustomAbort(
				ce.NewContractError(ce.ErrInitialization, "invalid minimum amount out"),
			)
		}
		if amountOut.Cmp(minOut) < 0 {
			ce.CustomAbort(
				ce.NewContractError(ce.ErrInitialization, "slippage tolerance exceeded"),
			)
		}
	}

	maybeEnv := types.MaybeEnv{}

	from := sdk.Address(params.From)
	if !from.IsValid() {
		ce.CustomAbort(ce.NewContractError(ce.ErrInput, "from address ["+params.From+"] invalid"))
	}

	inputAsset.DrawAssetFrom(amountIn, from, maybeEnv)

	// Handle referral fees
	if params.Beneficiary != nil && params.RefBps != nil {
		// refOut = amountOut * refBps / 10000
		if *params.RefBps > 10000 {
			ce.CustomAbort(
				ce.NewContractError(ce.ErrInput, "ref bps ["+strconv.FormatUint(*params.RefBps, 10)+"] > 10000"),
			)
		}
		refOut := new(big.Int).Set(amountOut)
		refOut.Mul(refOut, new(big.Int).SetUint64(*params.RefBps))
		refOut.Div(refOut, big.NewInt(10000))
		if refOut.Sign() == 1 {
			if refOut.Cmp(amountOut) > 0 {
				refOut.Set(amountOut)
			}
			amountOut.Sub(amountOut, refOut)
			err := outputAsset.TransferAsset(*params.Beneficiary, refOut)
			if err != nil {
				ce.CustomAbort(
					ce.WrapContractError(ce.ErrInitialization, err, "error transferring beneficiary asset out"),
				)
			}
		}

	}

	outputAsset.TransferAsset(params.To, amountOut)

	// Accumulate fees
	if magiFee.Sign() == 1 {
		currentFee := getBigInt(magiFeeKey)
		currentFee.Add(currentFee, magiFee)
		if currentFee.IsUint64() {
			setBigInt(magiFeeKey, currentFee)
		}
	}
	// add the LP fee to the input reserve
	if lpFee.Sign() == 1 {
		newRIn.Add(newRIn, lpFee)
	}
	// set new values for inputs now that fee is added to rIn
	setBigInt(rInKey, newRIn)
	setBigInt(rOutKey, newROut)

	// log fee and amount swapped
	sdk.Log(logFee(magiFee, lpFee))
	sdk.Log(logAmounts(amountIn, amountOut))

	// Return swap result with current pool state
	result := types.SwapResult{
		AmountOut: amountOut.String(),
		PoolState: types.PoolInfo{
			Asset0:   asset0.Name(),
			Asset1:   asset1.Name(),
			Reserve0: getReserve0().String(),
			Reserve1: getReserve1().String(),
			Fee:      getFee().Uint64(),
			TotalLp:  getTotalLp().String(),
		},
	}

	resultBytes, err := tinyjson.Marshal(&result)
	if err != nil {
		ce.CustomAbort(
			ce.WrapContractError(ce.ErrJson, err, "failed to serialze output"),
		)
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
		ce.CustomAbort(
			ce.NewContractError(ce.ErrInput, "payload required"),
		)
	}

	var params types.AddLiquidityParams
	if err := tinyjson.Unmarshal([]byte(*payload), &params); err != nil {
		ce.CustomAbort(
			ce.NewContractError(ce.ErrInput, "invalid payload"),
		)
	}

	amt0, ok := new(big.Int).SetString(params.Amount0, 10)
	if !ok {
		ce.CustomAbort(
			ce.NewContractError(ce.ErrInput, "invalid amount0"),
		)
	}
	amt1, ok := new(big.Int).SetString(params.Amount1, 10)
	if !ok {
		ce.CustomAbort(
			ce.NewContractError(ce.ErrInput, "invalid amount1"),
		)
	}

	if amt0.Sign() == 0 || amt1.Sign() == 0 || params.Recipient == "" {
		ce.CustomAbort(
			ce.NewContractError(ce.ErrInput, "missing required fields"),
		)
	}

	return executeAddLiquidity(amt0, amt1, params.Recipient)
}

// Remove liquidity from the pool
// Payload: JSON with withdrawal parameters
// {"lp_amount": 500000, "recipient": "user123"}
//
//go:wasmexport remove_liquidity
func RemoveLiquidity(payload *string) *string {
	if payload == nil {
		ce.CustomAbort(
			ce.NewContractError(ce.ErrInput, "payload required"),
		)
	}

	var params types.RemoveLiquidityParams
	if err := tinyjson.Unmarshal([]byte(*payload), &params); err != nil {
		ce.CustomAbort(
			ce.NewContractError(ce.ErrInput, "invalid payload"),
		)
	}

	lpAmt, ok := new(big.Int).SetString(params.LpAmount, 10)
	if !ok {
		ce.CustomAbort(
			ce.NewContractError(ce.ErrInput, "invalid lp amount"),
		)
	}

	if lpAmt.Sign() == 0 || params.Recipient == "" {
		ce.CustomAbort(
			ce.NewContractError(ce.ErrInput, "missing required fields"),
		)
	}

	return executeRemoveLiquidity(lpAmt, params.Recipient)
}

// Execute add liquidity operation
func executeAddLiquidity(amt0, amt1 *big.Int, provider string) *string {
	asset0, err := getAsset0()
	if err != nil {
		sdk.Abort("pool not initialized")
	}
	asset1, err := getAsset1()
	if err != nil {
		sdk.Abort("pool not initialized")
	}

	maybeEnv := types.MaybeEnv{}

	// Pull funds from user intents into contract
	if amt0.Sign() == 1 {
		asset0.DrawAssetFrom(amt0, maybeEnv.UseEnv().Sender.Address, maybeEnv)
	}
	if amt1.Sign() == 1 {
		asset1.DrawAssetFrom(amt1, maybeEnv.UseEnv().Sender.Address, maybeEnv)
	}

	// Update reserves and mint LP
	r0 := getReserve0()
	r1 := getReserve1()
	totalLP := getTotalLp()

	minted := new(big.Int)
	if totalLP.Sign() == 0 {
		// Geometric mean: sqrt(amt0 * amt1) using big.Int
		product := new(big.Int).Mul(amt0, amt1)
		minted = new(big.Int).Sqrt(product)
	} else {
		// Proportional minting: min(amt0 * totalLP / r0, amt1 * totalLP / r1)
		m0 := new(big.Int).Mul(amt0, totalLP)
		m0.Div(m0, r0)

		m1 := new(big.Int).Mul(amt1, totalLP)
		m1.Div(m1, r1)

		minted = m0
		if m1.Cmp(m0) < 0 {
			minted = m1
		}
	}
	contractAssert(minted.Sign() == 1)

	setReserve0(new(big.Int).Add(r0, amt0))
	setReserve1(new(big.Int).Add(r1, amt1))
	setTotalLp(new(big.Int).Add(totalLP, minted))

	// Mint LP tokens to provider
	providerAddr := sdk.Address(provider)
	currentLP := getLp(providerAddr.String())
	newLP := new(big.Int).Add(currentLP, minted)
	setLp(providerAddr.String(), newLP)

	return nil
}

// Execute remove liquidity operation
func executeRemoveLiquidity(lpAmount *big.Int, provider string) *string {
	providerAddr := sdk.Address(provider)
	userLP := getLp(providerAddr.String())
	totalLP := getTotalLp()

	contractAssert(lpAmount.Sign() == 1 && lpAmount.Cmp(userLP) <= 0 && lpAmount.Cmp(totalLP) <= 0)

	r0 := getReserve0()
	r1 := getReserve1()

	// amt = reserve * lpAmount / totalLP  (big.Int to avoid overflow)
	amt0 := new(big.Int).Mul(r0, lpAmount)
	amt0.Div(amt0, totalLP)

	amt1 := new(big.Int).Mul(r1, lpAmount)
	amt1.Div(amt1, totalLP)

	// newR0 = r0 - amt0
	newR0 := new(big.Int).Sub(r0, amt0)
	newR1 := new(big.Int).Sub(r1, amt1)
	newUserLP := new(big.Int).Sub(userLP, lpAmount)
	newTotalLP := new(big.Int).Sub(totalLP, lpAmount)

	if newR0.Sign() < 0 || newR1.Sign() < 0 || newUserLP.Sign() < 0 || newTotalLP.Sign() < 0 {
		ce.CustomAbort(
			ce.NewContractError(ce.ErrArithmetic, "underflow in remove liquidity"),
		)
	}

	setLp(providerAddr.String(), newUserLP)
	setTotalLp(newTotalLP)
	setReserve0(newR0)
	setReserve1(newR1)

	// Transfer assets out
	asset0, err := getAsset0()
	if err != nil {
		ce.CustomAbort(
			ce.WrapContractError(ce.ErrStateAccess, err, "error retrieving asset0"),
		)
	}
	asset1, err := getAsset1()
	if err != nil {
		ce.CustomAbort(
			ce.WrapContractError(ce.ErrStateAccess, err, "error retrieving asset0"),
		)
	}
	if amt0.Sign() == 1 {
		asset0.TransferAsset(provider, amt0)
	}
	if amt1.Sign() == 1 {
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
		ce.CustomAbort(
			ce.WrapContractError(ce.ErrInitialization, err, "pool not initialized"),
		)
	}
	asset1, err := getAsset0()
	if err != nil {
		ce.CustomAbort(
			ce.WrapContractError(ce.ErrInitialization, err, "pool not initialized"),
		)
	}

	poolInfo := types.PoolInfo{
		Asset0:   asset0.Name(),
		Asset1:   asset1.Name(),
		Reserve0: getReserve0().String(),
		Reserve1: getReserve1().String(),
		Fee:      getFee().Uint64(),
		TotalLp:  getTotalLp().String(),
	}

	resultBytes, err := tinyjson.Marshal(&poolInfo)
	if err != nil {
		ce.CustomAbort(
			ce.WrapContractError(ce.ErrJson, err, "failed to serialize output"),
		)
	}

	result := string(resultBytes)
	return &result
}

// Claim fees (system only)
//
//go:wasmexport claim_fees
func ClaimFees(payload *string) *string {
	if !isSystemSender() {
		ce.CustomAbort(
			ce.NewContractError(ce.ErrAuth, "system administrator only"),
		)
	}

	asset0, err := getAsset0()
	if err != nil {
		ce.CustomAbort(
			ce.WrapContractError(ce.ErrStateAccess, err, "pool not initialized"),
		)
	}
	asset1, err := getAsset1()
	if err != nil {
		ce.CustomAbort(
			ce.WrapContractError(ce.ErrStateAccess, err, "pool not initialized"),
		)
	}
	dao := sdk.Address("system:fr_balance")

	f0 := getBigInt(keySystemFee0)
	f1 := getBigInt(keySystemFee1)

	// can use hive withdraw because its only hbd
	if f0.Sign() == 1 && isHbd(asset0.Name()) {
		setBigInt(keySystemFee0, big.NewInt(0))
		sdk.HiveWithdraw(dao, f0, sdk.Asset(asset0.Name()))
	}
	if f1.Sign() == 1 && isHbd(asset1.Name()) {
		setBigInt(keySystemFee1, big.NewInt(0))
		sdk.HiveWithdraw(dao, f1, sdk.Asset(asset1.Name()))
	}

	setStr(keyFeeLastClaim, sdk.GetEnv().Timestamp)
	return nil
}
