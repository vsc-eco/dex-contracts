package tests

import (
	"encoding/hex"
	"strconv"
	"strings"
	"testing"
	"vsc-node/lib/test_utils"
	"vsc-node/modules/db/vsc/contracts"
	state_engine "vsc-node/modules/state-processing"

	"github.com/CosmWasm/tinyjson"
	"github.com/stretchr/testify/assert"
	dexcontracts "github.com/vsc-eco/dex-contracts"
	"github.com/vsc-eco/dex-contracts/contracts/types"

	"btc-mapping-contract/contract/constants"
	btcconstants "btc-mapping-contract/contract/constants"
	"btc-mapping-contract/contract/mapping"
)

// ============================================================================
// DEX-14 (HIGH) — Same-asset swap double-fee.
//
// Before the fix, executeSwap normalized AssetIn/AssetOut to lowercase but did
// NOT reject AssetIn == AssetOut. With no direct A/A pool registered the
// instruction fell through to executeTwoHopSwap (A -> HBD -> A), which charges
// pool fees on BOTH hops for what is economically a no-op. The fix adds an
// explicit guard right after normalization.
//
// A/B behavior:
//   - PRE-FIX: the instruction reaches executeTwoHopSwap. The test asserts the
//     call is rejected with the "asset_in and asset_out must differ" guard
//     message -> assertion FAILS (RED), because pre-fix it either succeeds
//     (double-charged) or fails with an unrelated routing error.
//   - POST-FIX: the guard aborts with exactly that message -> GREEN.
// ============================================================================

func TestSecurityDEX14_SameAssetSwapRejected(t *testing.T) {
	const owner = "hive:milo-hpr"
	user := "hive:dex14-user"
	hivehbdDexId := "vsc1Bjn53csDr6wUoYsjXiN9Nhadu458Tw9wvR"
	routerId := "vsc1Bpc3SgDqCRQxzeDrvV7T4XKV6BZuHmME5F"

	ct := test_utils.NewContractTest()
	t.Cleanup(func() { ct.DataLayer.Stop() })

	ct.RegisterContract(hivehbdDexId, owner, dexcontracts.DexWasm)
	ct.RegisterContract(routerId, owner, dexcontracts.DexRouterV2Wasm)

	router := &RouterInfo{ct: &ct, id: routerId}
	hivehbdDex := &DexInfo{ct: &ct, id: hivehbdDexId}

	r := router.initRouterV2(t, owner)
	if !r.Success {
		t.Fatalf("init router: %s", r.Ret)
	}

	// Register HIVE and HBD plus a HIVE/HBD pool so the two-hop path
	// (HIVE -> HBD -> HIVE) is actually reachable pre-fix.
	r = router.registerToken(t, owner, types.RegisterTokenParams{
		Name: "HIVE", TokenInfo: types.TokenInfo{Chain: "HIVE"},
	})
	if !r.Success {
		t.Fatalf("register HIVE: %s", r.Ret)
	}
	r = router.registerToken(t, owner, types.RegisterTokenParams{
		Name: "HBD", TokenInfo: types.TokenInfo{Chain: "HIVE"},
	})
	if !r.Success {
		t.Fatalf("register HBD: %s", r.Ret)
	}
	r = router.registerPool(t, owner, types.RegisterPoolParams{
		Asset0: "hive", Asset1: "hbd", DexContractId: hivehbdDexId,
	})
	if !r.Success {
		t.Fatalf("register hive/hbd pool: %s", r.Ret)
	}

	r = hivehbdDex.initPool(t, owner, &types.InitParams{
		Asset0:         "hbd",
		Asset1:         "hive",
		FeeBps:         100,
		RouterContract: routerId,
	})
	if !r.Success {
		t.Fatalf("init hive/hbd pool: %s: %s", r.Err, r.ErrMsg)
	}
	r = hivehbdDex.addLiquidity(t, owner, 100000_000, 100000_000)
	if !r.Success {
		t.Fatalf("add hive/hbd liquidity: %s: %s", r.Err, r.ErrMsg)
	}

	swapAmount := "5000000"
	ct.Deposit(user, 10000000, "hive")

	// AssetIn == AssetOut (both "hive"). There is no hive/hive pool, so pre-fix
	// this falls through to the two-hop path and gets double-charged.
	r = router.execute(t, user, &types.DexInstruction{
		Type:      "swap",
		Version:   "1.0.0",
		AssetIn:   "hive",
		AssetOut:  "hive",
		AmountIn:  swapAmount,
		Recipient: user,
	}, []contracts.Intent{
		{
			Type: "transfer.allow",
			Args: map[string]string{"token": "hive", "limit": swapAmount},
		},
	})

	dumpLogs(t, r.Logs)
	dumpStateDiff(t, r.StateDiff)
	t.Logf("DEX-14 same-asset swap result: success=%v err=%q ret=%q", r.Success, r.ErrMsg, r.Ret)

	assert.False(t, r.Success, "same-asset swap must be rejected, not routed through two hops")
	combined := r.ErrMsg + r.Ret
	assert.True(t,
		strings.Contains(combined, "asset_in and asset_out must differ"),
		"expected the same-asset guard message, got err=%q ret=%q", r.ErrMsg, r.Ret)
}

// ============================================================================
// DEX-06 (HIGH) — add_liquidity has no slippage protection.
//
// Before the fix, AddLiquidityParams had no MinLpOut field and
// executeAddLiquidity minted LP unconditionally. A provider supplying into a
// pool whose ratio shifted adversely could be minted far fewer LP than
// expected with no way to opt out.
//
// A/B behavior:
//   - PRE-FIX: min_lp_out is an unknown JSON key (no struct field), silently
//     ignored by the marshaler, so add_liquidity SUCCEEDS regardless ->
//     the "must be rejected" assertion FAILS (RED).
//   - POST-FIX: min_lp_out is parsed and enforced; when the minted LP is below
//     it the contract aborts with "insufficient LP minted: slippage" -> GREEN.
//
// Also asserts backward compatibility: an empty min_lp_out still succeeds.
// ============================================================================

func TestSecurityDEX06_AddLiquiditySlippageProtection(t *testing.T) {
	const owner = "hive:milo-hpr"
	dexId := "vsc1Bjn53csDr6wUoYsjXiN9Nhadu458Tw9wvR"
	routerId := "vsc1Bpc3SgDqCRQxzeDrvV7T4XKV6BZuHmME5F"

	ct := test_utils.NewContractTest()
	t.Cleanup(func() { ct.DataLayer.Stop() })

	ct.RegisterContract(dexId, owner, dexcontracts.DexWasm)
	ct.RegisterContract(routerId, owner, dexcontracts.DexRouterV2Wasm)

	dex := &DexInfo{ct: &ct, id: dexId}

	r := dex.initPool(t, owner, &types.InitParams{
		Asset0:         "hive",
		Asset1:         "hbd",
		FeeBps:         30,
		RouterContract: routerId,
	})
	if !r.Success {
		t.Fatalf("init pool: %s: %s", r.Err, r.ErrMsg)
	}

	// Seed the pool: first liquidity mints sqrt(1000*1000) = 1000 LP.
	r = dex.addLiquidity(t, owner, 1000, 1000)
	if !r.Success {
		t.Fatalf("seed liquidity: %s: %s", r.Err, r.ErrMsg)
	}

	// Add 1000/1000 again proportionally: min(1000*1000/1000, ...) = 1000 LP.
	// Demand 5000 LP minimum -> minted (1000) < min (5000) -> must be rejected
	// post-fix; pre-fix the field is dropped and the add succeeds.
	provider := "hive:dex06-lp"
	ct.Deposit(provider, 1000, "hive")
	ct.Deposit(provider, 1000, "hbd")

	tooHigh := types.AddLiquidityParams{
		Amount0:   "1000",
		Amount1:   "1000",
		Recipient: provider,
		MinLpOut:  "5000",
	}
	r = callAddLiquidity(t, &ct, dexId, provider, tooHigh)
	dumpLogs(t, r.Logs)
	dumpStateDiff(t, r.StateDiff)
	t.Logf("DEX-06 high min_lp_out result: success=%v err=%q ret=%q", r.Success, r.ErrMsg, r.Ret)

	assert.False(t, r.Success,
		"add_liquidity must be rejected when minted LP is below min_lp_out")
	combined := r.ErrMsg + r.Ret
	assert.True(t,
		strings.Contains(combined, "insufficient LP minted: slippage"),
		"expected slippage abort message, got err=%q ret=%q", r.ErrMsg, r.Ret)

	// Backward compatibility: a satisfiable min_lp_out (and the legacy
	// empty-field case) must still succeed.
	ct.Deposit(provider, 1000, "hive")
	ct.Deposit(provider, 1000, "hbd")
	ok := types.AddLiquidityParams{
		Amount0:   "1000",
		Amount1:   "1000",
		Recipient: provider,
		MinLpOut:  "900", // minted ~1000 >= 900
	}
	r = callAddLiquidity(t, &ct, dexId, provider, ok)
	t.Logf("DEX-06 satisfiable min_lp_out result: success=%v err=%q", r.Success, r.ErrMsg)
	assert.True(t, r.Success,
		"add_liquidity with a satisfiable min_lp_out should succeed: %s: %s", r.Err, r.ErrMsg)
}

// callAddLiquidity mirrors DexInfo.addLiquidity but lets the caller pass an
// explicit AddLiquidityParams (including MinLpOut).
func callAddLiquidity(
	t *testing.T,
	ct *test_utils.ContractTest,
	dexId, provider string,
	params types.AddLiquidityParams,
) *test_utils.ContractTestCallResult {
	t.Helper()
	amount0, _ := strconv.ParseUint(params.Amount0, 10, 64)
	amount1, _ := strconv.ParseUint(params.Amount1, 10, 64)
	payload, _ := tinyjson.Marshal(params)
	r := ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(t, provider),
		Caller:     provider,
		ContractId: dexId,
		Action:     "add_liquidity",
		Payload:    payload,
		RcLimit:    10000,
		Intents: []contracts.Intent{
			{
				Type: "transfer.allow",
				Args: map[string]string{
					"limit": strconv.FormatUint(amount0, 10),
					"token": "hive",
				},
			},
			{
				Type: "transfer.allow",
				Args: map[string]string{
					"limit": strconv.FormatUint(amount1, 10),
					"token": "hbd",
				},
			},
		},
	})
	return &r
}

// ============================================================================
// DEX-09 (MEDIUM) — settleToChain dropped MaxFee.
//
// Before the fix, settleToChain built types.UnmapParams without MaxFee and
// DexInstruction had no max_fee field, so a cross-chain swap could never cap
// the bridge fee: the BTC mapping contract's unmap charged whatever fee it
// computed. The fix adds DexInstruction.MaxFee and threads it through
// settleToChain into UnmapParams.MaxFee.
//
// A/B behavior (reuses the proven HIVE->BTC->unmap flow):
//   - PRE-FIX: max_fee is an unknown JSON key, dropped, never reaches the
//     mapping contract. unmap proceeds and the swap SUCCEEDS -> the
//     "must be rejected by max_fee cap" assertion FAILS (RED).
//   - POST-FIX: max_fee=1 sat reaches unmap; totalFee > 1 so unmap aborts with
//     "exceeds max_fee" and the whole swap fails -> GREEN.
// ============================================================================

func TestSecurityDEX09_SettleToChainHonorsMaxFee(t *testing.T) {
	requireWasm(t, "btc-mapping", dexcontracts.BTCMappingWasm)

	const owner = "hive:milo-hpr"
	btcMappingId := "vsc1BpQYDaMwcfdsh9T7DSEHZvdma1XaSXMPPj"
	btchbdDexId := "vsc1BquGPy8B766YpstdcL5cSF2GkWVVsVxJS3"
	hivehbdDexId := "vsc1Bjn53csDr6wUoYsjXiN9Nhadu458Tw9wvR"
	routerId := "vsc1Bpc3SgDqCRQxzeDrvV7T4XKV6BZuHmME5F"

	btcDestAddress := swapTestRegtestDestAddress(t)

	ct := test_utils.NewContractTest()
	t.Cleanup(func() { ct.DataLayer.Stop() })

	ct.RegisterContract(btcMappingId, owner, dexcontracts.BTCMappingWasm)
	ct.RegisterContract(btchbdDexId, owner, dexcontracts.DexWasm)
	ct.RegisterContract(hivehbdDexId, owner, dexcontracts.DexWasm)
	ct.RegisterContract(routerId, owner, dexcontracts.DexRouterV2Wasm)

	router := &RouterInfo{ct: &ct, id: routerId}
	btchbdDex := &DexInfo{ct: &ct, id: btchbdDexId}
	hivehbdDex := &DexInfo{ct: &ct, id: hivehbdDexId}

	r := router.registerToken(t, owner, types.RegisterTokenParams{
		Name:      "BTC",
		TokenInfo: types.TokenInfo{Chain: "BTC", MappingContract: btcMappingId},
	})
	if !r.Success {
		t.Fatalf("register BTC: %s", r.Ret)
	}
	r = router.registerToken(t, owner, types.RegisterTokenParams{
		Name: "HBD", TokenInfo: types.TokenInfo{Chain: "HIVE"},
	})
	if !r.Success {
		t.Fatalf("register HBD: %s", r.Ret)
	}
	r = router.registerToken(t, owner, types.RegisterTokenParams{
		Name: "HIVE", TokenInfo: types.TokenInfo{Chain: "HIVE"},
	})
	if !r.Success {
		t.Fatalf("register HIVE: %s", r.Ret)
	}

	r = router.registerPool(t, owner, types.RegisterPoolParams{
		Asset0: "btc", Asset1: "hbd", DexContractId: btchbdDexId,
	})
	if !r.Success {
		t.Fatalf("register btc/hbd pool: %s", r.Ret)
	}
	r = router.registerPool(t, owner, types.RegisterPoolParams{
		Asset0: "hive", Asset1: "hbd", DexContractId: hivehbdDexId,
	})
	if !r.Success {
		t.Fatalf("register hive/hbd pool: %s", r.Ret)
	}

	r = btchbdDex.initPool(t, owner, &types.InitParams{
		Asset0:                "btc",
		Asset1:                "hbd",
		FeeBps:                100,
		Asset0MappingContract: btcMappingId,
		RouterContract:        routerId,
	})
	if !r.Success {
		t.Fatalf("init btc/hbd pool: %s: %s", r.Err, r.ErrMsg)
	}
	ct.StateSet(btcMappingId, constants.BalancePrefix+owner, formatUintAsBytes(t, 2_00000000))
	ct.StateSet(btcMappingId,
		constants.AllowancePrefix+owner+constants.DirPathDelimiter+"contract:"+btchbdDexId,
		formatUintAsBytes(t, 2_00000000))
	r = btchbdDex.addLiquidity(t, owner, 1_00000000, 100000_000)
	if !r.Success {
		t.Fatalf("add btc/hbd liquidity: %s: %s", r.Err, r.ErrMsg)
	}

	r = hivehbdDex.initPool(t, owner, &types.InitParams{
		Asset0:         "hbd",
		Asset1:         "hive",
		FeeBps:         100,
		RouterContract: routerId,
	})
	if !r.Success {
		t.Fatalf("init hive/hbd pool: %s: %s", r.Err, r.ErrMsg)
	}
	r = hivehbdDex.addLiquidity(t, owner, 100000_000, 100000_000)
	if !r.Success {
		t.Fatalf("add hive/hbd liquidity: %s: %s", r.Err, r.ErrMsg)
	}

	// Seed BTC mapping state so unmap can build a spend transaction.
	const fakeTxId0 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const fakeTxId1 = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	txid0Bytes, _ := hex.DecodeString(fakeTxId0)
	txid1Bytes, _ := hex.DecodeString(fakeTxId1)
	var observedEntries []byte
	observedEntries = append(observedEntries, txid0Bytes...)
	observedEntries = append(observedEntries, 0, 0)
	observedEntries = append(observedEntries, txid1Bytes...)
	observedEntries = append(observedEntries, 0, 0)
	ct.StateSet(btcMappingId, constants.ObservedBlockPrefix+"100", string(observedEntries))
	ct.StateSet(btcMappingId, btcconstants.UtxoRegistryKey,
		string(mapping.MarshalUtxoRegistry(mapping.UtxoRegistry{
			{Id: 64, Amount: 50000000},
			{Id: 65, Amount: 50000000},
		})))
	ct.StateSet(btcMappingId, btcconstants.UtxoPrefix+"40",
		swapTestChangeUtxoBinary(t, fakeTxId0, 0, 50000000))
	ct.StateSet(btcMappingId, btcconstants.UtxoPrefix+"41",
		swapTestChangeUtxoBinary(t, fakeTxId1, 0, 50000000))
	ct.StateSet(btcMappingId, btcconstants.UtxoLastIdKey, string([]byte{66, 0}))
	ct.StateSet(btcMappingId, btcconstants.SupplyKey,
		string(mapping.MarshalSupply(&mapping.SystemSupply{
			ActiveSupply: 1_00000000,
			UserSupply:   1_00000000,
			BaseFeeRate:  1,
		})))
	ct.StateSet(btcMappingId, constants.LastHeightKey, "100")
	ct.StateSet(btcMappingId, btcconstants.BlockPrefix+"100",
		swapTestBuildSeedHeaderRaw(t))
	ct.StateSet(btcMappingId, btcconstants.PrimaryPublicKeyStateKey,
		swapTestDecodeHex(t, swapTestPrimaryPubKeyHex))
	ct.StateSet(btcMappingId, btcconstants.BackupPublicKeyStateKey,
		swapTestDecodeHex(t, swapTestBackupPubKeyHex))
	ct.StateSet(btcMappingId, btcconstants.RouterContractIdKey, routerId)

	swapAmount := "5000000"
	ct.Deposit(owner, 10000000, "hive")

	// max_fee = 1 satoshi. The real BTC unmap fee is far larger, so a
	// correctly-threaded MaxFee makes the mapping contract reject the unmap.
	tinyMaxFee := int64(1)

	r = router.execute(t, owner, &types.DexInstruction{
		Type:             "swap",
		Version:          "1.0.0",
		AssetIn:          "hive",
		AssetOut:         "btc",
		AmountIn:         swapAmount,
		Recipient:        btcDestAddress,
		DestinationChain: "BTC",
		MaxFee:           &tinyMaxFee,
	}, []contracts.Intent{
		{
			Type: "transfer.allow",
			Args: map[string]string{"token": "hive", "limit": swapAmount},
		},
	})

	dumpLogs(t, r.Logs)
	dumpStateDiff(t, r.StateDiff)
	t.Logf("DEX-09 max_fee=1 result: success=%v err=%q ret=%q", r.Success, r.ErrMsg, r.Ret)

	assert.False(t, r.Success,
		"cross-chain swap must be rejected when the unmap fee exceeds max_fee")
	combined := r.ErrMsg + r.Ret
	assert.True(t,
		strings.Contains(combined, "exceeds max_fee"),
		"expected the mapping contract's max_fee cap to fire, got err=%q ret=%q",
		r.ErrMsg, r.Ret)
}
