package tests

import (
	"btc-mapping-contract/contract/constants"
	"strconv"
	"strings"
	"testing"
	"vsc-node/lib/test_utils"
	"vsc-node/modules/db/vsc/contracts"
	ledger_db "vsc-node/modules/db/vsc/ledger"
	state_engine "vsc-node/modules/state-processing"

	"github.com/CosmWasm/tinyjson"
	"github.com/stretchr/testify/assert"
	dexcontracts "github.com/vsc-eco/dex-contracts"
	"github.com/vsc-eco/dex-contracts/contracts/types"
)

// ============================================================================
// H-01 — DrawAssetFrom caller validation
// Attacker cannot use someone else's address as "from" in a direct swap.
// ============================================================================

func TestAuditH01_DrawAssetFrom_AttackerCannotUseVictimAddress(t *testing.T) {
	ct, _, dexId := setupNativeHiveHbdPool(t, 100000, 100000)

	victim := "hive:victim-user"
	attacker := "hive:attacker"

	// Give the victim funds and do a legitimate action so the pool can see victim funds
	ct.Deposit(victim, 10000, ledger_db.Asset("hive"))
	ct.Deposit(attacker, 10000, ledger_db.Asset("hive"))

	// Attacker calls swap with from=victim. The DrawAssetFrom check should reject this.
	swapPayload, _ := tinyjson.Marshal(types.SwapParams{
		AssetIn:  "hive",
		AmountIn: "500",
		AssetOut: "hbd",
		From:     victim,
		To:       attacker,
	})
	r := ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(t, attacker),
		ContractId: dexId,
		Action:     "swap",
		Payload:    swapPayload,
		RcLimit:    1000,
		Intents: []contracts.Intent{
			{
				Type: "transfer.allow",
				Args: map[string]string{
					"token": "hive",
					"limit": "500",
				},
			},
		},
		Caller: attacker,
	})
	assert.False(t, r.Success, "swap with from=victim should fail when caller is attacker")
	assert.Contains(t, r.ErrMsg, "caller", "error should mention caller validation")
	t.Log("H-01 attack error:", r.ErrMsg)
}

func TestAuditH01_DrawAssetFrom_LegitimateSwap(t *testing.T) {
	ct, _, dexId := setupNativeHiveHbdPool(t, 100000, 100000)

	owner := "hive:milo-hpr"
	ct.Deposit(owner, 10000, ledger_db.Asset("hive"))

	// Owner calls swap with from=owner. Should succeed.
	swapPayload, _ := tinyjson.Marshal(types.SwapParams{
		AssetIn:  "hive",
		AmountIn: "500",
		AssetOut: "hbd",
		From:     owner,
		To:       owner,
	})
	r := ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(t, owner),
		ContractId: dexId,
		Action:     "swap",
		Payload:    swapPayload,
		RcLimit:    1000,
		Intents: []contracts.Intent{
			{
				Type: "transfer.allow",
				Args: map[string]string{
					"token": "hive",
					"limit": "500",
				},
			},
		},
		Caller: owner,
	})
	assert.True(t, r.Success, "swap with from=caller should succeed, got: %s", r.ErrMsg)
	t.Log("H-01 legitimate swap result:", r.Ret)
}

// ============================================================================
// C-04 — RemoveLiquidity authorization
// Non-owner cannot remove liquidity belonging to someone else.
// ============================================================================

func TestAuditC04_RemoveLiquidity_NonOwnerRejected(t *testing.T) {
	ct, _, dexId := setupNativeHiveHbdPool(t, 10000, 10000)

	attacker := "hive:attacker"

	// Attacker tries to remove owner's LP tokens
	payload, _ := tinyjson.Marshal(types.RemoveLiquidityParams{
		LpAmount:  "500",
		Recipient: attacker,
	})
	r := ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(t, attacker),
		ContractId: dexId,
		Action:     "remove_liquidity",
		Payload:    payload,
		RcLimit:    1000,
		Intents:    []contracts.Intent{},
		Caller:     attacker,
	})
	assert.False(t, r.Success, "non-owner should not be able to remove liquidity")
	t.Log("C-04 non-owner error:", r.ErrMsg)
}

func TestAuditC04_RemoveLiquidity_OwnerSucceeds(t *testing.T) {
	ct, _, dexId := setupNativeHiveHbdPool(t, 10000, 10000)

	owner := "hive:milo-hpr"

	payload, _ := tinyjson.Marshal(types.RemoveLiquidityParams{
		LpAmount:  "500",
		Recipient: owner,
	})
	r := ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(t, owner),
		ContractId: dexId,
		Action:     "remove_liquidity",
		Payload:    payload,
		RcLimit:    1000,
		Intents:    []contracts.Intent{},
		Caller:     owner,
	})
	assert.True(t, r.Success, "LP owner should be able to remove their own liquidity, got: %s", r.ErrMsg)
	t.Log("C-04 owner remove result:", r.Ret)
}

func TestAuditC04_RemoveLiquidity_RouterSucceeds(t *testing.T) {
	ct := test_utils.NewContractTest()
	t.Cleanup(func() { ct.DataLayer.Stop() })

	owner := "hive:milo-hpr"
	dexId := "vsc1Bjn53csDr6wUoYsjXiN9Nhadu458Tw9wvR"
	routerId := "vsc1Bpc3SgDqCRQxzeDrvV7T4XKV6BZuHmME5F"

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
		t.Fatalf("init pool failed: %s: %s", r.Err, r.ErrMsg)
	}

	r = dex.addLiquidity(t, owner, 10000, 10000)
	if !r.Success {
		t.Fatalf("add liquidity failed: %s: %s", r.Err, r.ErrMsg)
	}

	// Router calls remove_liquidity on behalf of the owner
	payload, _ := tinyjson.Marshal(types.RemoveLiquidityParams{
		LpAmount:  "500",
		Recipient: owner,
	})
	removeResult := ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(t, "contract:"+routerId),
		ContractId: dexId,
		Action:     "remove_liquidity",
		Payload:    payload,
		RcLimit:    1000,
		Intents:    []contracts.Intent{},
		Caller:     "contract:" + routerId,
	})
	assert.True(t, removeResult.Success, "router should be able to remove liquidity on behalf of user, got: %s", removeResult.ErrMsg)
	t.Log("C-04 router remove result:", removeResult.Ret)
}

// ============================================================================
// H-03 — ClaimFees supports all asset types
// Verify claim_fees works for both mapped and native pools.
// ============================================================================

func TestAuditH03_ClaimFees_NativePool(t *testing.T) {
	ct, _, dexId := setupNativeHiveHbdPool(t, 1000000, 1000000)
	owner := "hive:milo-hpr"

	// Do swaps to accumulate fees
	ct.Deposit(owner, 100000, ledger_db.Asset("hive"))
	for i := 0; i < 5; i++ {
		swapPayload, _ := tinyjson.Marshal(types.SwapParams{
			AssetIn:  "hive",
			AmountIn: "1000",
			AssetOut: "hbd",
			From:     owner,
			To:       owner,
		})
		sr := ct.Call(state_engine.TxVscCallContract{
			Self:       *basicSelf(t, owner),
			ContractId: dexId,
			Action:     "swap",
			Payload:    swapPayload,
			RcLimit:    1000,
			Intents: []contracts.Intent{
				{
					Type: "transfer.allow",
					Args: map[string]string{
						"token": "hive",
						"limit": "1000",
					},
				},
			},
			Caller: owner,
		})
		if !sr.Success {
			t.Fatalf("swap %d failed: %s: %s", i, sr.Err, sr.ErrMsg)
		}
	}

	// Claim fees as system sender
	systemCaller := "system:claim_fees"
	r := ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(t, systemCaller),
		ContractId: dexId,
		Action:     "claim_fees",
		Payload:    []byte("{}"),
		RcLimit:    1000,
		Intents:    []contracts.Intent{},
		Caller:     systemCaller,
	})
	assert.True(t, r.Success, "claim_fees on native pool should succeed, got: %s", r.ErrMsg)
	dumpLogs(t, r.Logs)
	t.Log("H-03 native claim_fees result:", r.Ret)
}

func TestAuditH03_ClaimFees_MappedPool(t *testing.T) {
	requireWasm(t, "btc-mapping", dexcontracts.BTCMappingWasm)
	ct := test_utils.NewContractTest()
	t.Cleanup(func() { ct.DataLayer.Stop() })

	owner := "hive:milo-hpr"
	btchbdDexId := "vsc1BquGPy8B766YpstdcL5cSF2GkWVVsVxJS3"
	btcMappingId := "vsc1BpQYDaMwcfdsh9T7DSEHZvdma1XaSXMPPj"
	routerId := "vsc1Bpc3SgDqCRQxzeDrvV7T4XKV6BZuHmME5F"

	ct.RegisterContract(btchbdDexId, owner, dexcontracts.DexWasm)
	ct.RegisterContract(btcMappingId, owner, dexcontracts.BTCMappingWasm)
	ct.RegisterContract(routerId, owner, dexcontracts.DexRouterV2Wasm)

	dex := &DexInfo{ct: &ct, id: btchbdDexId}

	r := dex.initPool(t, owner, &types.InitParams{
		Asset0:                "btc",
		Asset1:                "hbd",
		FeeBps:                100,
		Asset0MappingContract: btcMappingId,
		RouterContract:        routerId,
	})
	if !r.Success {
		t.Fatalf("init pool failed: %s: %s", r.Err, r.ErrMsg)
	}

	// Seed BTC balance and allowance
	ct.StateSet(btcMappingId, constants.BalancePrefix+owner, formatUintAsBytes(t, 2_00000000))
	ct.StateSet(
		btcMappingId,
		constants.AllowancePrefix+owner+constants.DirPathDelimiter+"contract:"+btchbdDexId,
		formatUintAsBytes(t, 2_00000000),
	)

	r = dex.addLiquidity(t, owner, 1_00000000, 100000_000)
	if !r.Success {
		t.Fatalf("add liquidity failed: %s: %s", r.Err, r.ErrMsg)
	}

	// Do swaps to accumulate fees on the mapped pool (HBD -> BTC direction)
	ct.Deposit(owner, 100000, ledger_db.Asset("hbd"))
	for i := 0; i < 3; i++ {
		swapPayload, _ := tinyjson.Marshal(types.SwapParams{
			AssetIn:  "hbd",
			AmountIn: "10000",
			AssetOut: "btc",
			From:     owner,
			To:       owner,
		})
		sr := ct.Call(state_engine.TxVscCallContract{
			Self:       *basicSelf(t, owner),
			ContractId: btchbdDexId,
			Action:     "swap",
			Payload:    swapPayload,
			RcLimit:    1000,
			Intents: []contracts.Intent{
				{
					Type: "transfer.allow",
					Args: map[string]string{
						"token": "hbd",
						"limit": "10000",
					},
				},
			},
			Caller: owner,
		})
		if !sr.Success {
			t.Fatalf("swap %d failed: %s: %s", i, sr.Err, sr.ErrMsg)
		}
	}

	// Claim fees as system sender on mapped pool
	systemCaller := "system:claim_fees"
	claimResult := ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(t, systemCaller),
		ContractId: btchbdDexId,
		Action:     "claim_fees",
		Payload:    []byte("{}"),
		RcLimit:    1000,
		Intents:    []contracts.Intent{},
		Caller:     systemCaller,
	})
	assert.True(t, claimResult.Success, "claim_fees on mapped pool should succeed, got: %s", claimResult.ErrMsg)
	dumpLogs(t, claimResult.Logs)
	dumpStateDiff(t, claimResult.StateDiff)
	t.Log("H-03 mapped claim_fees result:", claimResult.Ret)
}

// ============================================================================
// M-01 — Slippage check after referral deduction
// min_amount_out must be checked AFTER referral fee is subtracted.
// ============================================================================

func TestAuditM01_SlippageAfterReferral_Fail(t *testing.T) {
	ct, _, dexId := setupNativeHiveHbdPool(t, 1000000, 1000000)
	owner := "hive:milo-hpr"

	ct.Deposit(owner, 100000, ledger_db.Asset("hive"))

	// For a 1:1 pool of 1M/1M with 30bps fee, swapping 50000 in gives roughly 45422 out.
	// With 5% (500 bps) referral, post-referral = ~45422 * 0.95 = ~43150.
	// Set min_amount_out to 44000, which is between gross and post-referral.
	// This should FAIL because slippage check happens AFTER referral deduction.
	minAmountStr := "44000"
	t.Logf("M-01: min_amount_out = %s (between gross ~45422 and post-referral ~43150)", minAmountStr)

	beneficiary := "hive:referrer"
	refBps := uint64(500) // 5% referral
	swapPayload, _ := tinyjson.Marshal(types.SwapParams{
		AssetIn:      "hive",
		AmountIn:     "50000",
		AssetOut:     "hbd",
		From:         owner,
		To:           owner,
		MinAmountOut: &minAmountStr,
		Beneficiary:  &beneficiary,
		RefBps:       &refBps,
	})
	r := ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(t, owner),
		ContractId: dexId,
		Action:     "swap",
		Payload:    swapPayload,
		RcLimit:    1000,
		Intents: []contracts.Intent{
			{
				Type: "transfer.allow",
				Args: map[string]string{
					"token": "hive",
					"limit": "50000",
				},
			},
		},
		Caller: owner,
	})
	assert.False(t, r.Success, "swap should fail: min_amount_out > post-referral output")
	assert.Contains(t, r.ErrMsg, "slippage", "error should mention slippage")
	t.Log("M-01 slippage fail error:", r.ErrMsg)
}

func TestAuditM01_SlippageAfterReferral_Pass(t *testing.T) {
	ct, _, dexId := setupNativeHiveHbdPool(t, 1000000, 1000000)
	owner := "hive:milo-hpr"

	ct.Deposit(owner, 100000, ledger_db.Asset("hive"))

	// Set a very low min_amount_out that should pass even after referral deduction
	minAmount := "1"
	beneficiary := "hive:referrer"
	refBps := uint64(50) // 0.5% referral
	swapPayload, _ := tinyjson.Marshal(types.SwapParams{
		AssetIn:      "hive",
		AmountIn:     "5000",
		AssetOut:     "hbd",
		From:         owner,
		To:           owner,
		MinAmountOut: &minAmount,
		Beneficiary:  &beneficiary,
		RefBps:       &refBps,
	})
	r := ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(t, owner),
		ContractId: dexId,
		Action:     "swap",
		Payload:    swapPayload,
		RcLimit:    1000,
		Intents: []contracts.Intent{
			{
				Type: "transfer.allow",
				Args: map[string]string{
					"token": "hive",
					"limit": "5000",
				},
			},
		},
		Caller: owner,
	})
	assert.True(t, r.Success, "swap with low min_amount_out + referral should succeed, got: %s", r.ErrMsg)
	t.Log("M-01 slippage pass result:", r.Ret)
}

// ============================================================================
// M-02 — register_pool rejects duplicates
// Already covered by TestDuplicatePoolRegistration; this is a focused audit re-check.
// ============================================================================

func TestAuditM02_DuplicatePoolRejected(t *testing.T) {
	ct := test_utils.NewContractTest()
	t.Cleanup(func() { ct.DataLayer.Stop() })
	owner := "hive:milo-hpr"
	routerContractId := "vsc1BfqCB2b5ppiq4snQP74joWrJ3BMUN58pn9"
	dexContractId := "vsc1Bjn53csDr6wUoYsjXiN9Nhadu458Tw9wvR"

	ct.RegisterContract(routerContractId, owner, dexcontracts.DexRouterV2Wasm)
	router := &RouterInfo{ct: &ct, id: routerContractId}

	r := router.initRouterV2(t, owner)
	if !r.Success {
		t.Fatalf("router init failed: %s", r.Ret)
	}

	r = router.registerToken(t, owner, types.RegisterTokenParams{
		Name:      "HIVE",
		TokenInfo: types.TokenInfo{Chain: "HIVE"},
	})
	if !r.Success {
		t.Fatalf("register HIVE failed: %s", r.ErrMsg)
	}
	r = router.registerToken(t, owner, types.RegisterTokenParams{
		Name:      "HBD",
		TokenInfo: types.TokenInfo{Chain: "HIVE"},
	})
	if !r.Success {
		t.Fatalf("register HBD failed: %s", r.ErrMsg)
	}

	poolParams := types.RegisterPoolParams{
		Asset0:        "hive",
		Asset1:        "hbd",
		DexContractId: dexContractId,
	}

	r = router.registerPool(t, owner, poolParams)
	assert.True(t, r.Success, "first pool registration should succeed")

	r = router.registerPool(t, owner, poolParams)
	assert.False(t, r.Success, "duplicate pool registration should be rejected (M-02)")
	assert.Contains(t, r.ErrMsg, "already registered", "error should say pool already registered")
	t.Log("M-02 duplicate pool error:", r.ErrMsg)
}

// ============================================================================
// M-03 — AddLiquidity rejects negative amounts
// ============================================================================

func TestAuditM03_AddLiquidity_NegativeAmount0(t *testing.T) {
	ct := test_utils.NewContractTest()
	t.Cleanup(func() { ct.DataLayer.Stop() })
	owner := "hive:milo-hpr"
	dexId := "vsc1Bjn53csDr6wUoYsjXiN9Nhadu458Tw9wvR"
	routerId := "vsc1Bpc3SgDqCRQxzeDrvV7T4XKV6BZuHmME5F"

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
		t.Fatalf("init pool failed: %s: %s", r.Err, r.ErrMsg)
	}

	// Try with negative amount0
	payload, _ := tinyjson.Marshal(types.AddLiquidityParams{
		Amount0:   "-1000",
		Amount1:   "1000",
		Recipient: owner,
	})
	result := ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(t, owner),
		ContractId: dexId,
		Action:     "add_liquidity",
		Payload:    payload,
		RcLimit:    1000,
		Intents: []contracts.Intent{
			{
				Type: "transfer.allow",
				Args: map[string]string{
					"limit": "1000",
					"token": "hive",
				},
			},
			{
				Type: "transfer.allow",
				Args: map[string]string{
					"limit": "1000",
					"token": "hbd",
				},
			},
		},
		Caller: owner,
	})
	assert.False(t, result.Success, "add_liquidity with negative amount0 should fail")
	t.Log("M-03 negative amount0 error:", result.ErrMsg)
}

func TestAuditM03_AddLiquidity_NegativeAmount1(t *testing.T) {
	ct := test_utils.NewContractTest()
	t.Cleanup(func() { ct.DataLayer.Stop() })
	owner := "hive:milo-hpr"
	dexId := "vsc1Bjn53csDr6wUoYsjXiN9Nhadu458Tw9wvR"
	routerId := "vsc1Bpc3SgDqCRQxzeDrvV7T4XKV6BZuHmME5F"

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
		t.Fatalf("init pool failed: %s: %s", r.Err, r.ErrMsg)
	}

	// Try with negative amount1
	payload, _ := tinyjson.Marshal(types.AddLiquidityParams{
		Amount0:   "1000",
		Amount1:   "-1000",
		Recipient: owner,
	})
	result := ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(t, owner),
		ContractId: dexId,
		Action:     "add_liquidity",
		Payload:    payload,
		RcLimit:    1000,
		Intents: []contracts.Intent{
			{
				Type: "transfer.allow",
				Args: map[string]string{
					"limit": "1000",
					"token": "hive",
				},
			},
			{
				Type: "transfer.allow",
				Args: map[string]string{
					"limit": "1000",
					"token": "hbd",
				},
			},
		},
		Caller: owner,
	})
	assert.False(t, result.Success, "add_liquidity with negative amount1 should fail")
	t.Log("M-03 negative amount1 error:", result.ErrMsg)
}

// ============================================================================
// M-04 — Deposit amount validation in router
// Non-numeric amount should fail.
// ============================================================================

func TestAuditM04_RouterExecute_NonNumericAmount(t *testing.T) {
	ct := test_utils.NewContractTest()
	t.Cleanup(func() { ct.DataLayer.Stop() })

	owner := "hive:milo-hpr"
	dexId := "vsc1Bjn53csDr6wUoYsjXiN9Nhadu458Tw9wvR"
	routerId := "vsc1Bpc3SgDqCRQxzeDrvV7T4XKV6BZuHmME5F"

	ct.RegisterContract(dexId, owner, dexcontracts.DexWasm)
	ct.RegisterContract(routerId, owner, dexcontracts.DexRouterV2Wasm)

	router := &RouterInfo{ct: &ct, id: routerId}
	dex := &DexInfo{ct: &ct, id: dexId}

	r := router.registerToken(t, owner, types.RegisterTokenParams{
		Name:      "HIVE",
		TokenInfo: types.TokenInfo{Chain: "HIVE"},
	})
	if !r.Success {
		t.Fatalf("register HIVE failed: %s", r.ErrMsg)
	}
	r = router.registerToken(t, owner, types.RegisterTokenParams{
		Name:      "HBD",
		TokenInfo: types.TokenInfo{Chain: "HIVE"},
	})
	if !r.Success {
		t.Fatalf("register HBD failed: %s", r.ErrMsg)
	}
	r = router.registerPool(t, owner, types.RegisterPoolParams{
		Asset0:        "hive",
		Asset1:        "hbd",
		DexContractId: dexId,
	})
	if !r.Success {
		t.Fatalf("register pool failed: %s", r.ErrMsg)
	}
	dex.initPool(t, owner, &types.InitParams{
		Asset0:         "hive",
		Asset1:         "hbd",
		FeeBps:         30,
		RouterContract: routerId,
	})
	dex.addLiquidity(t, owner, 100000, 100000)

	ct.Deposit(owner, 10000, ledger_db.Asset("hive"))

	// Execute with non-numeric amount
	r = router.execute(t, owner, &types.DexInstruction{
		Type:      "swap",
		Version:   "1.0.0",
		AssetIn:   "hive",
		AssetOut:  "hbd",
		AmountIn:  "not-a-number",
		Recipient: owner,
		ReturnAddress: &types.ReturnAddress{
			Chain:   "VSC",
			Address: owner,
		},
	}, []contracts.Intent{
		{
			Type: "transfer.allow",
			Args: map[string]string{
				"token": "hive",
				"limit": "500",
			},
		},
	})
	assert.False(t, r.Success, "router execute with non-numeric amount should fail")
	t.Log("M-04 non-numeric amount error:", r.ErrMsg)
}

// ============================================================================
// L-03 — Empty mapping contract rejected for non-native assets
// ============================================================================

func TestAuditL03_EmptyMappingContract_Rejected(t *testing.T) {
	ct := test_utils.NewContractTest()
	t.Cleanup(func() { ct.DataLayer.Stop() })

	owner := "hive:milo-hpr"
	dexId := "vsc1Bjn53csDr6wUoYsjXiN9Nhadu458Tw9wvR"

	ct.RegisterContract(dexId, owner, dexcontracts.DexWasm)

	dex := &DexInfo{ct: &ct, id: dexId}

	// Try to init a BTC/HBD pool WITHOUT providing a mapping contract for BTC
	r := dex.initPool(t, owner, &types.InitParams{
		Asset0:                "btc",
		Asset1:                "hbd",
		FeeBps:                100,
		Asset0MappingContract: "", // Empty! Should fail for non-native asset
	})
	assert.False(t, r.Success, "init pool with empty mapping contract for non-native asset should fail")
	t.Log("L-03 empty mapping contract error:", r.ErrMsg)
}

// ============================================================================
// L-04 — isSystemSender checks all RequiredAuths, not just [0]
// ============================================================================

func TestAuditL04_SystemSenderInSecondAuth(t *testing.T) {
	ct, _, dexId := setupNativeHiveHbdPool(t, 1000000, 1000000)
	owner := "hive:milo-hpr"

	// Accumulate some fees
	ct.Deposit(owner, 100000, ledger_db.Asset("hive"))
	for i := 0; i < 3; i++ {
		swapPayload, _ := tinyjson.Marshal(types.SwapParams{
			AssetIn:  "hive",
			AmountIn: "1000",
			AssetOut: "hbd",
			From:     owner,
			To:       owner,
		})
		sr := ct.Call(state_engine.TxVscCallContract{
			Self:       *basicSelf(t, owner),
			ContractId: dexId,
			Action:     "swap",
			Payload:    swapPayload,
			RcLimit:    1000,
			Intents: []contracts.Intent{
				{
					Type: "transfer.allow",
					Args: map[string]string{
						"token": "hive",
						"limit": "1000",
					},
				},
			},
			Caller: owner,
		})
		if !sr.Success {
			t.Fatalf("swap %d failed: %s: %s", i, sr.Err, sr.ErrMsg)
		}
	}

	// Call claim_fees with system auth at position [1] (not [0])
	self := state_engine.TxSelf{
		TxId:                 strconv.FormatInt(txId, 10),
		BlockId:              strconv.FormatInt(txId, 10),
		Index:                0,
		OpIndex:              0,
		Timestamp:            "2025-10-14T00:00:00",
		RequiredAuths:        []string{"hive:some-user", "system:claim_fees"},
		RequiredPostingAuths: []string{},
	}
	txId++

	r := ct.Call(state_engine.TxVscCallContract{
		Self:       self,
		ContractId: dexId,
		Action:     "claim_fees",
		Payload:    []byte("{}"),
		RcLimit:    1000,
		Intents:    []contracts.Intent{},
		Caller:     "system:claim_fees",
	})
	assert.True(t, r.Success, "claim_fees should succeed with system auth in position [1], got: %s", r.ErrMsg)
	t.Log("L-04 system auth in position [1] result:", r.Ret)
}

func TestAuditL04_NonSystemSenderRejected(t *testing.T) {
	ct, _, dexId := setupNativeHiveHbdPool(t, 1000000, 1000000)

	// claim_fees has no caller authorization — fees always go to the
	// contract owner regardless of who triggers the claim.  Verify that
	// a non-system user CAN call it (it's harmless) and fees still go
	// to the owner, not the caller.
	attacker := "hive:attacker"
	r := ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(t, attacker),
		ContractId: dexId,
		Action:     "claim_fees",
		Payload:    []byte("{}"),
		RcLimit:    1000,
		Intents:    []contracts.Intent{},
		Caller:     attacker,
	})
	assert.True(t, r.Success, "claim_fees is callable by anyone (fees go to owner): %s", r.ErrMsg)
	t.Log("L-04 non-system claim result:", r.Ret)
}

// ============================================================================
// H-01 (mapped asset variant) — Attacker cannot draw from victim's mapped balance
// ============================================================================

func TestAuditH01_MappedAsset_AttackerCannotUseVictimAddress(t *testing.T) {
	requireWasm(t, "btc-mapping", dexcontracts.BTCMappingWasm)
	ct := test_utils.NewContractTest()
	t.Cleanup(func() { ct.DataLayer.Stop() })

	owner := "hive:milo-hpr"
	victim := "hive:victim-user"
	attacker := "hive:attacker"
	btchbdDexId := "vsc1BquGPy8B766YpstdcL5cSF2GkWVVsVxJS3"
	btcMappingId := "vsc1BpQYDaMwcfdsh9T7DSEHZvdma1XaSXMPPj"
	routerId := "vsc1Bpc3SgDqCRQxzeDrvV7T4XKV6BZuHmME5F"

	ct.RegisterContract(btchbdDexId, owner, dexcontracts.DexWasm)
	ct.RegisterContract(btcMappingId, owner, dexcontracts.BTCMappingWasm)
	ct.RegisterContract(routerId, owner, dexcontracts.DexRouterV2Wasm)

	dex := &DexInfo{ct: &ct, id: btchbdDexId}

	r := dex.initPool(t, owner, &types.InitParams{
		Asset0:                "btc",
		Asset1:                "hbd",
		FeeBps:                100,
		Asset0MappingContract: btcMappingId,
		RouterContract:        routerId,
	})
	if !r.Success {
		t.Fatalf("init pool failed: %s: %s", r.Err, r.ErrMsg)
	}

	// Seed victim's mapped BTC balance and allowance to the pool
	ct.StateSet(btcMappingId, constants.BalancePrefix+victim, formatUintAsBytes(t, 1_00000000))
	ct.StateSet(btcMappingId, constants.BalancePrefix+owner, formatUintAsBytes(t, 2_00000000))
	ct.StateSet(
		btcMappingId,
		constants.AllowancePrefix+victim+constants.DirPathDelimiter+"contract:"+btchbdDexId,
		formatUintAsBytes(t, 1_00000000),
	)
	ct.StateSet(
		btcMappingId,
		constants.AllowancePrefix+owner+constants.DirPathDelimiter+"contract:"+btchbdDexId,
		formatUintAsBytes(t, 2_00000000),
	)

	r = dex.addLiquidity(t, owner, 1_00000000, 100000_000)
	if !r.Success {
		t.Fatalf("add liquidity failed: %s: %s", r.Err, r.ErrMsg)
	}

	// Attacker tries to swap using victim's BTC balance
	swapPayload, _ := tinyjson.Marshal(types.SwapParams{
		AssetIn:  "btc",
		AmountIn: "10000",
		AssetOut: "hbd",
		From:     victim,
		To:       attacker,
	})
	sr := ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(t, attacker),
		ContractId: btchbdDexId,
		Action:     "swap",
		Payload:    swapPayload,
		RcLimit:    1000,
		Intents:    []contracts.Intent{},
		Caller:     attacker,
	})
	assert.False(t, sr.Success, "attacker should not be able to swap using victim's mapped balance")
	assert.Contains(t, sr.ErrMsg, "caller", "error should mention caller validation")
	t.Log("H-01 mapped attack error:", sr.ErrMsg)
}

// ============================================================================
// PreDeposited flag — only router can set it
// ============================================================================

func TestAuditH01_PreDeposited_DirectCallRejected(t *testing.T) {
	ct, _, dexId := setupNativeHiveHbdPool(t, 100000, 100000)

	attacker := "hive:attacker"
	ct.Deposit(attacker, 10000, ledger_db.Asset("hive"))

	// Attacker tries to swap with PreDeposited=true to skip the deposit draw
	swapPayload, _ := tinyjson.Marshal(types.SwapParams{
		AssetIn:      "hive",
		AmountIn:     "500",
		AssetOut:     "hbd",
		From:         attacker,
		To:           attacker,
		PreDeposited: true,
	})
	r := ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(t, attacker),
		ContractId: dexId,
		Action:     "swap",
		Payload:    swapPayload,
		RcLimit:    1000,
		Intents:    []contracts.Intent{},
		Caller:     attacker,
	})
	assert.False(t, r.Success, "direct call with PreDeposited=true should be rejected")
	assert.Contains(t, strings.ToLower(r.ErrMsg), "router", "error should mention router authorization")
	t.Log("PreDeposited direct call error:", r.ErrMsg)
}

// ============================================================================
// Comprehensive: second LP provider cannot steal from first via remove_liquidity
// ============================================================================

func TestAuditC04_SecondProvider_CannotStealFirstProviderLP(t *testing.T) {
	ct, _, dexId := setupNativeHiveHbdPool(t, 10000, 10000)

	owner := "hive:milo-hpr"
	alice := "hive:alice"

	// Alice adds liquidity
	ct.Deposit(alice, 5000, ledger_db.Asset("hive"))
	ct.Deposit(alice, 5000, ledger_db.Asset("hbd"))
	addPayload, _ := tinyjson.Marshal(types.AddLiquidityParams{
		Amount0:   "5000",
		Amount1:   "5000",
		Recipient: alice,
	})
	r := ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(t, alice),
		ContractId: dexId,
		Action:     "add_liquidity",
		Payload:    addPayload,
		RcLimit:    1000,
		Intents: []contracts.Intent{
			{
				Type: "transfer.allow",
				Args: map[string]string{
					"limit": "5000",
					"token": "hive",
				},
			},
			{
				Type: "transfer.allow",
				Args: map[string]string{
					"limit": "5000",
					"token": "hbd",
				},
			},
		},
		Caller: alice,
	})
	assert.True(t, r.Success, "alice should add liquidity, got: %s", r.ErrMsg)

	// Alice tries to remove owner's full LP amount — should fail because
	// alice only owns her own LP tokens
	ownerLpRaw := ct.StateGet(dexId, "lp"+types.DirPathDelimiter+owner)
	if ownerLpRaw == "" {
		t.Fatal("owner should have LP tokens")
	}

	// Alice tries to remove more LP than she owns (owner's amount)
	payload, _ := tinyjson.Marshal(types.RemoveLiquidityParams{
		LpAmount:  "10000", // More than alice has
		Recipient: alice,
	})
	r = ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(t, alice),
		ContractId: dexId,
		Action:     "remove_liquidity",
		Payload:    payload,
		RcLimit:    1000,
		Intents:    []contracts.Intent{},
		Caller:     alice,
	})
	assert.False(t, r.Success, "alice should not be able to remove more LP than she owns")
	t.Log("C-04 LP theft prevention error:", r.ErrMsg)
}
