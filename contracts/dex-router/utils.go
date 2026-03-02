package main

import (
	sdk "dex-router/sdk"
	"math/bits"
	"strconv"
)

// Keys for state storage
const (
	keyVersion          = "version"
	keyNextPoolId       = "next_pool_id"
	keyPoolPrefix       = "pool-" // pool/{poolId}/...
	keyPoolAsset0       = "asset0"
	keyPoolAsset1       = "asset1"
	keyPoolReserve0     = "reserve0"
	keyPoolReserve1     = "reserve1"
	keyPoolFee          = "fee"
	keyPoolTotalLP      = "total_lp"
	keyPoolLpPrefix     = "lp-" // lp/{address}
	keyPoolFee0         = "fee0"
	keyPoolFee1         = "fee1"
	keyPoolFeeLastClaim = "fee_last_claim"
)

const (
	defaultBaseFeeBps        = 8     // 0.08%
	defaultFeeClaimIntervalS = 86400 // 1 day
	defaultSlipBaselineBps   = 0     // off by default
	defaultSlipShareBps      = 0     // off by default
)

// Pool key helpers
func poolKey(poolId string, suffix string) string {
	return keyPoolPrefix + poolId + "-" + suffix
}

func poolAsset0Key(poolId string) string {
	return poolKey(poolId, keyPoolAsset0)
}

func poolAsset1Key(poolId string) string {
	return poolKey(poolId, keyPoolAsset1)
}

func poolReserve0Key(poolId string) string {
	return poolKey(poolId, keyPoolReserve0)
}

func poolReserve1Key(poolId string) string {
	return poolKey(poolId, keyPoolReserve1)
}

func poolFeeKey(poolId string) string {
	return poolKey(poolId, keyPoolFee)
}

func poolTotalLpKey(poolId string) string {
	return poolKey(poolId, keyPoolTotalLP)
}

func poolLpKey(poolId, address string) string {
	return poolKey(poolId, keyPoolLpPrefix+address)
}

func poolFee0Key(poolId string) string {
	return poolKey(poolId, keyPoolFee0)
}

func poolFee1Key(poolId string) string {
	return poolKey(poolId, keyPoolFee1)
}

func poolFeeLastClaimKey(poolId string) string {
	return poolKey(poolId, keyPoolFeeLastClaim)
}

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

func getInt(key string) int64 {
	v := sdk.StateGetObject(key)
	if v == nil {
		return 0
	}
	n, _ := strconv.ParseInt(*v, 10, 64)
	return n
}

func setInt(key string, val int64) {
	sdk.StateSetObject(key, strconv.FormatInt(val, 10))
}

// Pool state helpers
func getPoolAsset0(poolId string) string {
	return getStr(poolAsset0Key(poolId))
}

func getPoolAsset1(poolId string) string {
	return getStr(poolAsset1Key(poolId))
}

func getPoolReserve0(poolId string) uint64 {
	return getUint(poolReserve0Key(poolId))
}

func getPoolReserve1(poolId string) uint64 {
	return getUint(poolReserve1Key(poolId))
}

func getPoolFee(poolId string) uint64 {
	return getUint(poolFeeKey(poolId))
}

func getPoolTotalLp(poolId string) uint64 {
	return getUint(poolTotalLpKey(poolId))
}

func getPoolLp(poolId, address string) uint64 {
	return getUint(poolLpKey(poolId, address))
}

func setPoolAsset0(poolId, asset string) {
	setStr(poolAsset0Key(poolId), asset)
}

func setPoolAsset1(poolId, asset string) {
	setStr(poolAsset1Key(poolId), asset)
}

func setPoolReserve0(poolId string, reserve uint64) {
	setUint(poolReserve0Key(poolId), reserve)
}

func setPoolReserve1(poolId string, reserve uint64) {
	setUint(poolReserve1Key(poolId), reserve)
}

func setPoolFee(poolId string, fee uint64) {
	setUint(poolFeeKey(poolId), fee)
}

func setPoolTotalLp(poolId string, totalLp uint64) {
	setUint(poolTotalLpKey(poolId), totalLp)
}

func setPoolLp(poolId, address string, amount uint64) {
	setUint(poolLpKey(poolId, address), amount)
}

// Utility functions
func min64(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}

func max64(a, b uint64) uint64 {
	if a > b {
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
