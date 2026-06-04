package tests

import (
	"strconv"
	"strings"
	"testing"
	"vsc-node/lib/test_utils"
	"vsc-node/modules/db/vsc/contracts"
	ledger_db "vsc-node/modules/db/vsc/ledger"
	state_engine "vsc-node/modules/state-processing"

	contract_session "vsc-node/modules/contract/session"

	"github.com/CosmWasm/tinyjson"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	dexcontracts "github.com/vsc-eco/dex-contracts"
	"github.com/vsc-eco/dex-contracts/contracts/types"
)

// ============================================================================
// C8 cluster LOW-fix regression tests (mainnet audit 2026-06-03).
//
// These exercise the deployed-WASM behavior through the WasmEdge contract test
// harness (same harness as security_fixes_test.go / audit_fixes_test.go). Each
// test is written A/B: it asserts the POST-FIX behavior and is constructed so
// that it FAILS on the unfixed @176942d WASM (the C2 lesson — a regression test
// that passes without the fix is worthless). The pre-fix failure mode is
// documented per-test.
//
// NOTE: these require the native WasmEdge runtime (cgo). They run wherever the
// existing tests/ suite runs (CI with WasmEdge installed). They do NOT run in a
// WasmEdge-less sandbox — there the whole tests/ package is skipped at build.
// ============================================================================

// feeBuckets pulls the lp / ns / nc values out of the pool's "fee|..." receipt
// log line. Returns (lp, ns, nc, found).
func feeBuckets(t *testing.T, logs map[string]contract_session.LogOutput) (lp, ns, nc string, found bool) {
	t.Helper()
	for _, output := range logs {
		for _, line := range output.Logs {
			if !strings.HasPrefix(line, "fee"+types.LogDelimiter) {
				continue
			}
			found = true
			for _, kv := range strings.Split(line, types.LogDelimiter) {
				parts := strings.SplitN(kv, types.LogKeyDelimiter, 2)
				if len(parts) != 2 {
					continue
				}
				switch parts[0] {
				case "lp":
					lp = parts[1]
				case "ns":
					ns = parts[1]
				case "nc":
					nc = parts[1]
				}
			}
		}
	}
	return lp, ns, nc, found
}

// ----------------------------------------------------------------------------
// DX-L07 — Execute() rejects a swap/deposit/withdrawal whose recipient is the
// router contract itself (output / LP would be permanently stranded).
//
// PRE-FIX: sdk.VerifyAddress("contract:"+ROUTER) == "contract" (not "unknown"),
// so the recipient passes validation and the swap SUCCEEDS, routing output to
// the router's own account -> the "must be rejected" assertion FAILS (RED).
// POST-FIX: the self-recipient guard aborts with
// "recipient cannot be the router contract itself" -> GREEN.
// ----------------------------------------------------------------------------
func TestDXL07_SwapToRouterSelfRejected(t *testing.T) {
	requireWasm(t, "dex", dexcontracts.DexWasm)
	requireWasm(t, "dex-router-v2", dexcontracts.DexRouterV2Wasm)

	const owner = "hive:milo-hpr"
	user := "hive:dxl07-user"
	hivehbdDexId := "vsc1Bjn53csDr6wUoYsjXiN9Nhadu458Tw9wvR"
	routerId := "vsc1Bpc3SgDqCRQxzeDrvV7T4XKV6BZuHmME5F"

	ct := test_utils.NewContractTest()
	whitelistPendulum(&ct)
	t.Cleanup(func() { ct.DataLayer.Stop() })

	ct.RegisterContract(hivehbdDexId, owner, dexcontracts.DexWasm)
	ct.RegisterContract(routerId, owner, dexcontracts.DexRouterV2Wasm)

	router := &RouterInfo{ct: &ct, id: routerId}
	hivehbdDex := &DexInfo{ct: &ct, id: hivehbdDexId}

	r := router.initRouterV2(t, owner)
	require.True(t, r.Success, "init router: %s", r.Ret)

	r = router.registerToken(t, owner, types.RegisterTokenParams{
		Name: "HIVE", TokenInfo: types.TokenInfo{Chain: "HIVE"},
	})
	require.True(t, r.Success, "register HIVE: %s", r.Ret)
	r = router.registerToken(t, owner, types.RegisterTokenParams{
		Name: "HBD", TokenInfo: types.TokenInfo{Chain: "HIVE"},
	})
	require.True(t, r.Success, "register HBD: %s", r.Ret)
	r = router.registerPool(t, owner, types.RegisterPoolParams{
		Asset0: "hive", Asset1: "hbd", DexContractId: hivehbdDexId,
	})
	require.True(t, r.Success, "register hive/hbd pool: %s", r.Ret)

	r = hivehbdDex.initPool(t, owner, &types.InitParams{
		Asset0:         "hbd",
		Asset1:         "hive",
		FeeBps:         100,
		RouterContract: routerId,
	})
	require.True(t, r.Success, "init pool: %s: %s", r.Err, r.ErrMsg)
	r = hivehbdDex.addLiquidity(t, owner, 100000_000, 100000_000)
	require.True(t, r.Success, "add liquidity: %s: %s", r.Err, r.ErrMsg)

	swapAmount := "1000000"
	ct.Deposit(user, 10000000, ledger_db.Asset("hbd"))

	// Recipient == the router contract itself.
	r = router.execute(t, user, &types.DexInstruction{
		Type:      "swap",
		Version:   "1.0.0",
		AssetIn:   "hbd",
		AssetOut:  "hive",
		AmountIn:  swapAmount,
		Recipient: "contract:" + routerId,
	}, []contracts.Intent{
		{Type: "transfer.allow", Args: map[string]string{"token": "hbd", "limit": swapAmount}},
	})

	dumpLogs(t, r.Logs)
	t.Logf("DX-L07 swap-to-router result: success=%v err=%q ret=%q", r.Success, r.ErrMsg, r.Ret)

	assert.False(t, r.Success, "swap with recipient=router must be rejected (output would be stranded)")
	combined := r.ErrMsg + r.Ret
	assert.True(t, strings.Contains(combined, "recipient cannot be the router contract itself"),
		"expected self-recipient guard message, got err=%q ret=%q", r.ErrMsg, r.Ret)
}

// Control: a swap to a normal recipient must still succeed (guards against the
// fix over-rejecting).
func TestDXL07_SwapToNormalRecipientStillSucceeds(t *testing.T) {
	requireWasm(t, "dex", dexcontracts.DexWasm)
	requireWasm(t, "dex-router-v2", dexcontracts.DexRouterV2Wasm)

	const owner = "hive:milo-hpr"
	hivehbdDexId := "vsc1Bjn53csDr6wUoYsjXiN9Nhadu458Tw9wvR"
	routerId := "vsc1Bpc3SgDqCRQxzeDrvV7T4XKV6BZuHmME5F"

	ct := test_utils.NewContractTest()
	whitelistPendulum(&ct)
	t.Cleanup(func() { ct.DataLayer.Stop() })

	ct.RegisterContract(hivehbdDexId, owner, dexcontracts.DexWasm)
	ct.RegisterContract(routerId, owner, dexcontracts.DexRouterV2Wasm)

	router := &RouterInfo{ct: &ct, id: routerId}
	hivehbdDex := &DexInfo{ct: &ct, id: hivehbdDexId}

	r := router.initRouterV2(t, owner)
	require.True(t, r.Success, "init router: %s", r.Ret)
	r = router.registerToken(t, owner, types.RegisterTokenParams{Name: "HIVE", TokenInfo: types.TokenInfo{Chain: "HIVE"}})
	require.True(t, r.Success, "register HIVE: %s", r.Ret)
	r = router.registerToken(t, owner, types.RegisterTokenParams{Name: "HBD", TokenInfo: types.TokenInfo{Chain: "HIVE"}})
	require.True(t, r.Success, "register HBD: %s", r.Ret)
	r = router.registerPool(t, owner, types.RegisterPoolParams{Asset0: "hive", Asset1: "hbd", DexContractId: hivehbdDexId})
	require.True(t, r.Success, "register pool: %s", r.Ret)
	r = hivehbdDex.initPool(t, owner, &types.InitParams{Asset0: "hbd", Asset1: "hive", FeeBps: 100, RouterContract: routerId})
	require.True(t, r.Success, "init pool: %s: %s", r.Err, r.ErrMsg)
	r = hivehbdDex.addLiquidity(t, owner, 100000_000, 100000_000)
	require.True(t, r.Success, "add liquidity: %s: %s", r.Err, r.ErrMsg)

	// owner already has RC headroom + HBD balance from pool setup; recipient is a
	// normal Hive account (NOT the router), so the swap must succeed.
	swapAmount := "1000000"
	ct.Deposit(owner, 10000000, ledger_db.Asset("hbd"))
	r = router.execute(t, owner, &types.DexInstruction{
		Type: "swap", Version: "1.0.0", AssetIn: "hbd", AssetOut: "hive",
		AmountIn: swapAmount, Recipient: "hive:milo-vsc",
	}, []contracts.Intent{
		{Type: "transfer.allow", Args: map[string]string{"token": "hbd", "limit": swapAmount}},
	})
	t.Logf("DX-L07 control swap result: success=%v err=%q", r.Success, r.ErrMsg)
	assert.True(t, r.Success, "swap to a normal recipient should still succeed: %s: %s", r.Err, r.ErrMsg)
}

// ----------------------------------------------------------------------------
// DX-L38 — a dust swap must pay the rounded-up minimum protocol fee (1 unit),
// not zero. The pendulum's integer fee math floors lp/ns/nc to 0 for a tiny
// input; the contract now charges 1 unit on the output side when ALL buckets
// are zero and the output is > 1.
//
// PRE-FIX: dust swap succeeds with fee log lp=0|ns=0|nc=0 -> the
// "nc must be non-zero" assertion FAILS (RED).
// POST-FIX: the same dust swap succeeds but with nc=1 -> GREEN.
// Happy-path control: a normal swap is unchanged (already non-zero buckets).
// ----------------------------------------------------------------------------
func TestDXL38_DustSwapChargesMinimumFee(t *testing.T) {
	requireWasm(t, "dex", dexcontracts.DexWasm)

	owner := "hive:milo-hpr"
	// Tiny reserves so a 1-unit input produces a positive gross output (>1) while
	// the pendulum's integer fee math floors EVERY bucket to zero — the exact
	// dust-bypass condition D-L38 proved on mainnet.
	//
	// setupNativeHiveHbdPool normalizes assets alphabetically: asset0=hbd,
	// asset1=hive, so liq0=10 seeds reserve_hbd=10 and liq1=30 seeds
	// reserve_hive=30. We swap HBD -> HIVE so the OUTPUT reserve (hive=30) is the
	// large side; with X=reserve_hbd=10, Y=reserve_hive=30, x=1:
	//   grossOut   = floor(1*30/11) = 2  (> 0, so the pendulum does not reject)
	//   baseProto  = floor(2*8/10000) = 0
	//   baseCLP    = floor(1*1*30/121) = 0
	// => lp=ns=nc=0 from the pendulum, userOutput=2. The fix then charges the
	// rounded-up minimum 1-unit fee (nc=1) and the user keeps 1.
	ct, _, dexId := setupNativeHiveHbdPool(t, 10, 30)

	// owner already holds RC headroom + balances from pool setup; just top up the
	// HBD input balance for the swap.
	ct.Deposit(owner, 1000, ledger_db.Asset("hbd"))

	// Dust swap: amount_in = 1, hbd -> hive.
	dustPayload, _ := tinyjson.Marshal(types.SwapParams{
		AssetIn:  "hbd",
		AmountIn: "1",
		AssetOut: "hive",
		From:     owner,
		To:       owner,
	})
	r := ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(t, owner),
		ContractId: dexId,
		Action:     "swap",
		Payload:    dustPayload,
		RcLimit:    2000,
		Intents: []contracts.Intent{
			{Type: "transfer.allow", Args: map[string]string{"token": "hbd", "limit": "1"}},
		},
		Caller: owner,
	})
	dumpLogs(t, r.Logs)
	t.Logf("DX-L38 dust swap result: success=%v err=%q ret=%q", r.Success, r.ErrMsg, r.Ret)

	require.True(t, r.Success, "dust swap should still succeed (we charge a min fee, not reject): %s: %s", r.Err, r.ErrMsg)

	lp, ns, nc, found := feeBuckets(t, r.Logs)
	require.True(t, found, "expected a fee receipt log line")
	t.Logf("DX-L38 dust fee buckets: lp=%s ns=%s nc=%s", lp, ns, nc)

	// The rounded-up minimum fee lands in the network-share bucket. PRE-FIX the
	// pendulum returns nc=0 for this swap; the fix bumps it to 1.
	assert.Equal(t, "1", nc,
		"dust swap must charge a 1-unit minimum protocol fee (nc), got lp=%s ns=%s nc=%s", lp, ns, nc)

	// And the user must still receive a positive output (we did not zero them).
	var res types.SwapResult
	require.NoError(t, tinyjson.Unmarshal([]byte(r.Ret), &res))
	ao, perr := strconv.ParseInt(res.AmountOut, 10, 64)
	require.NoError(t, perr, "parse amount_out %q", res.AmountOut)
	assert.Greater(t, ao, int64(0), "user output must remain positive after the min fee")
}

// Happy-path control: a normal-sized swap must be UNCHANGED by the dust fix —
// its fee buckets are already non-zero and the swap math is untouched.
func TestDXL38_NormalSwapUnaffected(t *testing.T) {
	requireWasm(t, "dex", dexcontracts.DexWasm)

	owner := "hive:milo-hpr"
	ct, _, dexId := setupNativeHiveHbdPool(t, 100000_000, 100000_000)

	ct.Deposit(owner, 1000000, ledger_db.Asset("hbd"))

	payload, _ := tinyjson.Marshal(types.SwapParams{
		AssetIn:  "hbd",
		AmountIn: "100000", // well above dust
		AssetOut: "hive",
		From:     owner,
		To:       owner,
	})
	r := ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(t, owner),
		ContractId: dexId,
		Action:     "swap",
		Payload:    payload,
		RcLimit:    2000,
		Intents: []contracts.Intent{
			{Type: "transfer.allow", Args: map[string]string{"token": "hbd", "limit": "100000"}},
		},
		Caller: owner,
	})
	dumpLogs(t, r.Logs)
	require.True(t, r.Success, "normal swap should succeed: %s: %s", r.Err, r.ErrMsg)

	lp, ns, nc, found := feeBuckets(t, r.Logs)
	require.True(t, found, "expected a fee receipt log line")
	t.Logf("DX-L38 normal fee buckets: lp=%s ns=%s nc=%s", lp, ns, nc)
	// At least one bucket non-zero from the pendulum itself — the dust branch
	// never fires for a normal swap, so this is the pendulum's own output.
	assert.True(t, lp != "0" || ns != "0" || nc != "0",
		"normal swap must have a non-zero pendulum fee bucket, got lp=%s ns=%s nc=%s", lp, ns, nc)
}

// ----------------------------------------------------------------------------
// D-L29 — RegisterToken charset-validates the native token name. A comma (which
// corrupts the comma-joined keyTokensList) or any non-[a-z0-9] char is rejected.
//
// PRE-FIX: a name like "hbd,hive" passes (no charset check) and is appended to
// the comma-joined registry, corrupting it -> the "must be rejected" assertion
// FAILS (RED).
// POST-FIX: register_token aborts with the charset message -> GREEN.
// ----------------------------------------------------------------------------
func TestDL29_RegisterTokenRejectsDelimiterInName(t *testing.T) {
	requireWasm(t, "dex-router-v2", dexcontracts.DexRouterV2Wasm)

	const owner = "hive:milo-hpr"
	routerId := "vsc1Bpc3SgDqCRQxzeDrvV7T4XKV6BZuHmME5F"

	ct := test_utils.NewContractTest()
	whitelistPendulum(&ct)
	t.Cleanup(func() { ct.DataLayer.Stop() })
	ct.RegisterContract(routerId, owner, dexcontracts.DexRouterV2Wasm)
	router := &RouterInfo{ct: &ct, id: routerId}

	r := router.initRouterV2(t, owner)
	require.True(t, r.Success, "init router: %s", r.Ret)

	bad := []string{"hbd,hive", "h b d", "hbd\tx", "HBD!"}
	for _, name := range bad {
		t.Run(name, func(t *testing.T) {
			r := router.registerToken(t, owner, types.RegisterTokenParams{
				Name: name, TokenInfo: types.TokenInfo{Chain: "HIVE"},
			})
			combined := r.ErrMsg + r.Ret
			t.Logf("D-L29 register %q: success=%v err=%q ret=%q", name, r.Success, r.ErrMsg, r.Ret)
			assert.False(t, r.Success, "register_token with name %q must be rejected", name)
			assert.True(t, strings.Contains(combined, "lowercase ASCII"),
				"expected charset message for %q, got err=%q ret=%q", name, r.ErrMsg, r.Ret)
		})
	}
}

// Control: a valid token name must still register (idempotent — second attempt
// is "asset already registered", which is also acceptable).
func TestDL29_RegisterTokenAcceptsValidName(t *testing.T) {
	requireWasm(t, "dex-router-v2", dexcontracts.DexRouterV2Wasm)

	const owner = "hive:milo-hpr"
	routerId := "vsc1Bpc3SgDqCRQxzeDrvV7T4XKV6BZuHmME5F"

	ct := test_utils.NewContractTest()
	whitelistPendulum(&ct)
	t.Cleanup(func() { ct.DataLayer.Stop() })
	ct.RegisterContract(routerId, owner, dexcontracts.DexRouterV2Wasm)
	router := &RouterInfo{ct: &ct, id: routerId}

	r := router.initRouterV2(t, owner)
	require.True(t, r.Success, "init router: %s", r.Ret)

	for _, name := range []string{"hbd", "hive", "btc", "eth", "usdt0"} {
		r := router.registerToken(t, owner, types.RegisterTokenParams{
			Name: name, TokenInfo: types.TokenInfo{Chain: "HIVE"},
		})
		t.Logf("D-L29 register valid %q: success=%v ret=%q", name, r.Success, r.Ret)
		assert.True(t, r.Success || strings.Contains(r.Ret, "already registered"),
			"valid name %q should register (or be already registered), got %q", name, r.Ret)
	}
}

// ----------------------------------------------------------------------------
// DX-L55 — the same RegisterToken charset guard blocks non-UTF-8 / non-ASCII
// bytes in a token name, so no non-UTF-8 string can reach tinyjson.Marshal (the
// jwriter U+FFFD roundtrip-corruption path is closed at the entry point).
//
// PRE-FIX: a name with a raw 0xff byte passes ToLower unchanged, is marshaled,
// and jwriter silently substitutes U+FFFD -> the register SUCCEEDS with a
// mutated stored name -> the "must be rejected" assertion FAILS (RED).
// POST-FIX: the charset guard rejects the non-ASCII byte -> GREEN.
// ----------------------------------------------------------------------------
func TestDXL55_RegisterTokenRejectsNonUTF8Name(t *testing.T) {
	requireWasm(t, "dex-router-v2", dexcontracts.DexRouterV2Wasm)

	const owner = "hive:milo-hpr"
	routerId := "vsc1Bpc3SgDqCRQxzeDrvV7T4XKV6BZuHmME5F"

	ct := test_utils.NewContractTest()
	whitelistPendulum(&ct)
	t.Cleanup(func() { ct.DataLayer.Stop() })
	ct.RegisterContract(routerId, owner, dexcontracts.DexRouterV2Wasm)
	router := &RouterInfo{ct: &ct, id: routerId}

	r := router.initRouterV2(t, owner)
	require.True(t, r.Success, "init router: %s", r.Ret)

	// Build the register_token payload manually so a raw non-UTF-8 / non-ASCII
	// byte survives into the name field (the typed helper would still carry it,
	// but we craft the JSON directly to be explicit about the injected byte).
	for _, name := range []string{"hb\xffd", "hb\x80d", "héllo"} {
		t.Run(strconv.QuoteToASCII(name), func(t *testing.T) {
			payload, _ := tinyjson.Marshal(types.RegisterTokenParams{
				Name: name, TokenInfo: types.TokenInfo{Chain: "HIVE"},
			})
			rr := ct.Call(state_engine.TxVscCallContract{
				Self:       *basicSelf(t, owner),
				ContractId: routerId,
				Action:     "register_token",
				Payload:    payload,
				RcLimit:    2000,
				Intents:    []contracts.Intent{},
				Caller:     owner,
			})
			t.Logf("DX-L55 register %q: success=%v ret=%q", name, rr.Success, rr.Ret)
			assert.False(t, rr.Success, "register_token with non-ASCII name %q must be rejected", name)
		})
	}
}
