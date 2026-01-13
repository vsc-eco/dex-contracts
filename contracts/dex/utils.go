package main

import (
	sdk "dex/sdk"
	"math/bits"
	"strconv"
)

// Keys for state storage
const (
	keyAsset0      = "asset0"
	keyAsset1      = "asset1"
	keyReserve0    = "reserve0"
	keyReserve1    = "reserve1"
	keyFee         = "fee"
	keyTotalLP     = "total_lp"
	keyLpPrefix    = "lp/" // lp/{address}
	keyFee0        = "fee0"
	keyFee1        = "fee1"
	keyFeeLastClaim = "fee_last_claim"
)

const (
	defaultBaseFeeBps = 8 // 0.08%
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

// Pool state helpers
func getAsset0() string {
	return getStr(keyAsset0)
}

func getAsset1() string {
	return getStr(keyAsset1)
}

func getReserve0() uint64 {
	return getUint(keyReserve0)
}

func getReserve1() uint64 {
	return getUint(keyReserve1)
}

func getFee() uint64 {
	return getUint(keyFee)
}

func getTotalLp() uint64 {
	return getUint(keyTotalLP)
}

func getLp(address string) uint64 {
	return getUint(keyLpPrefix + address)
}

func setAsset0(asset string) {
	setStr(keyAsset0, asset)
}

func setAsset1(asset string) {
	setStr(keyAsset1, asset)
}

func setReserve0(reserve uint64) {
	setUint(keyReserve0, reserve)
}

func setReserve1(reserve uint64) {
	setUint(keyReserve1, reserve)
}

func setFee(fee uint64) {
	setUint(keyFee, fee)
}

func setTotalLp(totalLp uint64) {
	setUint(keyTotalLP, totalLp)
}

func setLp(address string, amount uint64) {
	setUint(keyLpPrefix+address, amount)
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
		panic("assertion failed")
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

// Token adapter wrappers
func drawAsset(amount int64, asset string) {
	sdk.HiveDraw(amount, sdk.Asset(asset))
}

func transferAsset(to string, amount int64, asset string) {
	sdk.HiveTransfer(sdk.Address(to), amount, sdk.Asset(asset))
}

// Check if asset is HBD
func isHbd(asset string) bool {
	return asset == "HBD"
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
