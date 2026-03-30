package main

import (
	"math/big"
	"strconv"
	"strings"

	"github.com/vsc-eco/dex-contracts/contracts/asset"
	"github.com/vsc-eco/dex-contracts/contracts/types"
	"github.com/vsc-eco/dex-contracts/sdk"
)

// Keys for state storage
const (
	keyAsset0         = "a1"
	keyAsset1         = "a2"
	keyReserve0       = "r0"
	keyReserve1       = "r1"
	keyFee            = "fee"
	keyTotalLP        = "tlp"
	keyLpPrefix       = "lp" + types.DirPathDelimiter // lp-{address}
	keySystemFee0     = "f0"
	keySystemFee1     = "f1"
	keyFeeLastClaim   = "flc"
	keyRouter         = "rtr" // authorized router contract ID
	keyAsset0Name     = "a1n" // cached asset0 lowercase name (avoids JSON unmarshal)
	keyAsset1Name     = "a2n" // cached asset1 lowercase name
	keyMigrateVersion = "mv"  // migration version tracker
)

const (
	JSON_ERROR = "json_error"
)

const (
	defaultBaseFeeBps = 8 // 0.08%
)

// Logs
const (
	logDelimiter    = "|"
	logKeyDelimiter = "="
	// logArrayDelimiter = ","
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

func getBigInt(key string) *big.Int {
	v := sdk.StateGetObject(key)
	n := new(big.Int)
	if v == nil || *v == "" {
		return n
	}

	n.SetBytes([]byte(*v))
	return n
}

func setBigInt(key string, val *big.Int) {
	sdk.StateSetObject(key, string(val.Bytes()))
}

// Pool state helpers
func getAsset0() (asset.Asset, error) {
	assetStr := getStr(keyAsset0)

	asset0, err := asset.AssetFromJson(assetStr)
	if err != nil {
		return nil, err
	}

	return asset0, nil
}

func getAsset1() (asset.Asset, error) {
	assetStr := getStr(keyAsset1)

	asset1, err := asset.AssetFromJson(assetStr)
	if err != nil {
		return nil, err
	}

	return asset1, nil
}

func getReserve0() *big.Int {
	return getBigInt(keyReserve0)
}

func getReserve1() *big.Int {
	return getBigInt(keyReserve1)
}

func getFee() *big.Int {
	return getBigInt(keyFee)
}

func getTotalLp() *big.Int {
	return getBigInt(keyTotalLP)
}

func getLp(address string) *big.Int {
	return getBigInt(keyLpPrefix + address)
}

func setAsset0(asset string) {
	setStr(keyAsset0, asset)
}

func setAsset1(asset string) {
	setStr(keyAsset1, asset)
}

func setReserve0(reserve *big.Int) {
	setBigInt(keyReserve0, reserve)
}

func setReserve1(reserve *big.Int) {
	setBigInt(keyReserve1, reserve)
}

func setFee(fee *big.Int) {
	setBigInt(keyFee, fee)
}

func setTotalLp(totalLp *big.Int) {
	setBigInt(keyTotalLP, totalLp)
}

func setLp(address string, amount *big.Int) {
	setBigInt(keyLpPrefix+address, amount)
}

func contractAssert(cond bool, msgs ...string) {
	if !cond {
		msg := "assertion failed"
		if len(msgs) > 0 {
			msg = msgs[0]
		}
		sdk.Abort(msg)
	}
}

func logFee(magiFee, lpFee *big.Int) string {
	totalFee := new(big.Int).Add(magiFee, lpFee)
	var b strings.Builder
	b.Grow(64)

	// 2. Header
	b.WriteString("fee")
	b.WriteString(logDelimiter)

	b.WriteString("t")
	b.WriteString(logKeyDelimiter)
	b.WriteString(totalFee.String())
	b.WriteString(logDelimiter)

	b.WriteString("m")
	b.WriteString(logKeyDelimiter)
	b.WriteString(magiFee.String())
	b.WriteString(logDelimiter)

	b.WriteString("lp")
	b.WriteString(logKeyDelimiter)
	b.WriteString(lpFee.String())

	return b.String()
}

func logSwap(assetIn, assetOut string, amountIn, amountOut *big.Int, to string) string {
	var b strings.Builder
	b.Grow(64)

	b.WriteString("swap")
	b.WriteString(logDelimiter)

	b.WriteString("in")
	b.WriteString(logKeyDelimiter)
	b.WriteString(assetIn)
	b.WriteString(logDelimiter)

	b.WriteString("out")
	b.WriteString(logKeyDelimiter)
	b.WriteString(assetOut)
	b.WriteString(logDelimiter)

	b.WriteString("ai")
	b.WriteString(logKeyDelimiter)
	b.WriteString(amountIn.String())
	b.WriteString(logDelimiter)

	b.WriteString("ao")
	b.WriteString(logKeyDelimiter)
	b.WriteString(amountOut.String())
	b.WriteString(logDelimiter)

	b.WriteString("to")
	b.WriteString(logKeyDelimiter)
	b.WriteString(to)

	return b.String()
}

func logAddLiquidity(provider string, amt0, amt1, minted *big.Int) string {
	var b strings.Builder
	b.Grow(64)

	b.WriteString("add_liq")
	b.WriteString(logDelimiter)

	b.WriteString("p")
	b.WriteString(logKeyDelimiter)
	b.WriteString(provider)
	b.WriteString(logDelimiter)

	b.WriteString("a0")
	b.WriteString(logKeyDelimiter)
	b.WriteString(amt0.String())
	b.WriteString(logDelimiter)

	b.WriteString("a1")
	b.WriteString(logKeyDelimiter)
	b.WriteString(amt1.String())
	b.WriteString(logDelimiter)

	b.WriteString("lp")
	b.WriteString(logKeyDelimiter)
	b.WriteString(minted.String())

	return b.String()
}

func logRemoveLiquidity(provider string, amt0, amt1, lpAmount *big.Int) string {
	var b strings.Builder
	b.Grow(64)

	b.WriteString("rem_liq")
	b.WriteString(logDelimiter)

	b.WriteString("p")
	b.WriteString(logKeyDelimiter)
	b.WriteString(provider)
	b.WriteString(logDelimiter)

	b.WriteString("a0")
	b.WriteString(logKeyDelimiter)
	b.WriteString(amt0.String())
	b.WriteString(logDelimiter)

	b.WriteString("a1")
	b.WriteString(logKeyDelimiter)
	b.WriteString(amt1.String())
	b.WriteString(logDelimiter)

	b.WriteString("lp")
	b.WriteString(logKeyDelimiter)
	b.WriteString(lpAmount.String())

	return b.String()
}

func logPoolInit(asset0, asset1 string, feeBps uint64) string {
	var b strings.Builder
	b.Grow(64)

	b.WriteString("pool_init")
	b.WriteString(logDelimiter)

	b.WriteString("a0")
	b.WriteString(logKeyDelimiter)
	b.WriteString(asset0)
	b.WriteString(logDelimiter)

	b.WriteString("a1")
	b.WriteString(logKeyDelimiter)
	b.WriteString(asset1)
	b.WriteString(logDelimiter)

	b.WriteString("fee")
	b.WriteString(logKeyDelimiter)
	b.WriteString(strconv.FormatUint(feeBps, 10))

	return b.String()
}

func logMigrate(version, asset0, asset1 string) string {
	var b strings.Builder
	b.Grow(64)

	b.WriteString("migrate")
	b.WriteString(logDelimiter)

	b.WriteString("v")
	b.WriteString(logKeyDelimiter)
	b.WriteString(version)
	b.WriteString(logDelimiter)

	b.WriteString("a0")
	b.WriteString(logKeyDelimiter)
	b.WriteString(asset0)
	b.WriteString(logDelimiter)

	b.WriteString("a1")
	b.WriteString(logKeyDelimiter)
	b.WriteString(asset1)

	return b.String()
}
