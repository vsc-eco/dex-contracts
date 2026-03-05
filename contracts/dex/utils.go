package main

import (
	sdk "dex/sdk"
	"math/big"
	"math/bits"
	"strings"

	. "dex/dex-internal"
)

// Keys for state storage
const (
	keyAsset0                       = "a1"
	keyAsset1                       = "a2"
	keyReserve0                     = "r0"
	keyReserve1                     = "r1"
	keyFee                          = "fee"
	keyTotalLP                      = "tlp"
	keyLpPrefix                     = "lp/" // lp/{address}
	keySystemFee0                   = "f0"
	keySystemFee1                   = "f1"
	keyFeeLastClaim                 = "flc"
	keyRouterAssetPrefix            = "as/"
	keyMappingContractBalancePrefix = "bal/"
)

const (
	JSON_ERROR = "json_error"
)

const (
	defaultBaseFeeBps = 8 // 0.08%
)

// Logs
const (
	logDelimiter      = "|"
	logKeyDelimiter   = "="
	logArrayDelimiter = ","
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
func getAsset0() (Asset, error) {
	assetStr := getStr(keyAsset0)

	asset0, err := AssetFromJson(assetStr)
	if err != nil {
		return nil, err
	}

	return asset0, nil
}

func getAsset1() (Asset, error) {
	assetStr := getStr(keyAsset1)

	asset1, err := AssetFromJson(assetStr)
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

// Utility functions
func min64(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}

func contractAssert(cond bool) {
	if !cond {
		sdk.Abort("assertion failed")
	}
}

func isSystemSender() bool {
	env := sdk.GetEnv()
	if env.Sender.Address.Domain() == sdk.AddressDomainSystem {
		return true
	}
	if len(env.Sender.RequiredAuths) > 0 && env.Sender.RequiredAuths[0].Domain() == sdk.AddressDomainSystem {
		return true
	}
	return false
}

func isValidAsset(s string) bool {
	a := sdk.Asset(s)
	switch a {
	case sdk.AssetHive, sdk.AssetHiveCons, sdk.AssetHbd, sdk.AssetHbdSavings:
		return true
	default:
		return false
	}
}

// Check if asset is HBD
func isHbd(asset string) bool {
	return asset == sdk.AssetHbd.String()
}

// sqrt128 returns floor(sqrt(hi:lo)) where hi:lo is a 128-bit unsigned integer
func sqrt128(hi, lo uint64) uint64 {
	var low, high uint64 = 0, ^uint64(0) >> 1
	var ans uint64
	for low <= high {
		mid := (low + high) >> 1
		mh, ml := bits.Mul64(mid, mid)
		if mh < hi || (mh == hi && ml <= lo) {
			ans = mid
			low = mid + 1
		} else {
			high = mid - 1
		}
	}
	return ans
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

func logAmounts(in, out *big.Int) string {
	var b strings.Builder
	b.Grow(64)

	// 2. Header
	b.WriteString("amt")
	b.WriteString(logDelimiter)

	b.WriteString("i")
	b.WriteString(logKeyDelimiter)
	b.WriteString(in.String())
	b.WriteString(logDelimiter)

	b.WriteString("o")
	b.WriteString(logKeyDelimiter)
	b.WriteString(out.String())

	return b.String()
}
