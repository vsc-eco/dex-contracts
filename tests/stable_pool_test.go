package tests

// StableSwap pool (curve "stable"): the node prices the swap on the curve
// when the network lists the pool (go-vsc-node system-config StablePools,
// consensus 0.13.0) and the pool checks the result against the same integer
// curve. These tests run the real pool wasm on the real node applier.

import (
	"math/big"
	"strconv"
	"testing"

	"vsc-node/lib/test_utils"
	"vsc-node/modules/db/vsc/contracts"
	ledger_db "vsc-node/modules/db/vsc/ledger"
	pendulumoracle "vsc-node/modules/incentive-pendulum/oracle"
	pendulumwasm "vsc-node/modules/incentive-pendulum/wasm"
	state_engine "vsc-node/modules/state-processing"
	wasm_context "vsc-node/modules/wasm/context"

	"github.com/CosmWasm/tinyjson"
	"github.com/stretchr/testify/require"

	dexcontracts "github.com/vsc-eco/dex-contracts"
	"github.com/vsc-eco/dex-contracts/contracts/types"
)

const (
	stablePoolId = "vsc1BmLNMQep1RaaUdYTPfEhqn1inESqNz4Ekt"
	stableOwner  = "hive:milo-hpr"
	stableRouter = "vsc1Bpc3SgDqCRQxzeDrvV7T4XKV6BZuHmME5F"
)

func u64(v uint64) *uint64 { return &v }

var testGeometry = pendulumoracle.GeometryOutputs{OK: true, V: 500_000, P: 250_000, E: 1_000_000, T: 1_000_000, SBps: 5000}

// listStable wires the real applier with the pool listed (HBD/HIVE, 3/3
// decimals, A=100) at consensus 0.13.0.
func listStable(ct *test_utils.ContractTest, amp int64) {
	ct.SetPendulumStablePools(testGeometry, append(append([]string{}, knownDexPoolIDs...), stablePoolId),
		map[string]pendulumwasm.StableListing{
			stablePoolId: {Amp: amp, Asset0: "hbd", Decimals0: 3, Asset1: "hive", Decimals1: 3},
		})
}

func setupStableHbdHivePool(t *testing.T, liq uint64) (test_utils.ContractTest, *DexInfo) {
	t.Helper()
	requireWasm(t, "dex", dexcontracts.DexWasm)
	ct := test_utils.NewContractTest()
	listStable(&ct, 100)
	t.Cleanup(func() { ct.DataLayer.Stop() })
	ct.RegisterContract(stablePoolId, stableOwner, dexcontracts.DexWasm)
	ct.RegisterContract(stableRouter, stableOwner, dexcontracts.DexRouterV2Wasm)
	dex := &DexInfo{ct: &ct, id: stablePoolId}
	r := dex.initPool(t, stableOwner, &types.InitParams{
		Asset0: "hive", Asset1: "hbd", FeeBps: 8, RouterContract: stableRouter,
		Curve: "stable", Amp: 100, Decimals0: u64(3), Decimals1: u64(3),
	})
	require.True(t, r.Success, "init stable pool: %s: %s", r.Err, r.ErrMsg)
	r = dex.addLiquidity(t, stableOwner, liq, liq)
	require.True(t, r.Success, "add liquidity: %s: %s", r.Err, r.ErrMsg)
	return ct, dex
}

func swapNative(t *testing.T, ct *test_utils.ContractTest, pool, user, in, out string, amount uint64) test_utils.ContractTestCallResult {
	t.Helper()
	ct.Deposit(user, int64(amount), ledger_db.Asset(in))
	ct.Deposit(user, rcHeadroomHBD, ledger_db.Asset("hbd"))
	amt := strconv.FormatUint(amount, 10)
	payload, _ := tinyjson.Marshal(types.SwapParams{AssetIn: in, AmountIn: amt, AssetOut: out, From: user, To: user})
	return ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(t, user),
		ContractId: pool,
		Action:     "swap",
		Payload:    payload,
		RcLimit:    10_000,
		Intents:    []contracts.Intent{{Type: "transfer.allow", Args: map[string]string{"token": in, "limit": amt}}},
		Caller:     user,
	})
}

func TestStablePool_InitValidation(t *testing.T) {
	requireWasm(t, "dex", dexcontracts.DexWasm)
	bad := []types.InitParams{
		{Curve: "curvy", Amp: 100, Decimals0: u64(3), Decimals1: u64(3)},
		{Curve: "stable", Amp: 0, Decimals0: u64(3), Decimals1: u64(3)},
		{Curve: "stable", Amp: 10_001, Decimals0: u64(3), Decimals1: u64(3)},
		{Curve: "stable", Amp: 100, Decimals0: u64(3)},
		{Curve: "stable", Amp: 100, Decimals1: u64(3)},
		{Curve: "stable", Amp: 100, Decimals0: u64(19), Decimals1: u64(3)},
	}
	for i, p := range bad {
		ct := test_utils.NewContractTest()
		ct.RegisterContract(stablePoolId, stableOwner, dexcontracts.DexWasm)
		ct.RegisterContract(stableRouter, stableOwner, dexcontracts.DexRouterV2Wasm)
		dex := &DexInfo{ct: &ct, id: stablePoolId}
		p.Asset0, p.Asset1, p.RouterContract = "hive", "hbd", stableRouter
		r := dex.initPool(t, stableOwner, &p)
		require.False(t, r.Success, "case %d (%+v) must be refused", i, p)
		ct.DataLayer.Stop()
	}

	// The pair is stored alphabetically; the decimals follow their asset.
	ct := test_utils.NewContractTest()
	t.Cleanup(func() { ct.DataLayer.Stop() })
	ct.RegisterContract(stablePoolId, stableOwner, dexcontracts.DexWasm)
	ct.RegisterContract(stableRouter, stableOwner, dexcontracts.DexRouterV2Wasm)
	payload, _ := tinyjson.Marshal(&types.InitParams{
		Asset0: "usdc", Asset0MappingContract: "vsc1Bo8h3TMFAYX5rkgCeauXFywZGiMbUftQL2", Decimals0: u64(6),
		Asset1: "hbd", Decimals1: u64(3),
		Curve: "stable", Amp: 250, RouterContract: stableRouter,
	})
	r := ct.Call(state_engine.TxVscCallContract{
		Self: *basicSelf(t, stableOwner), ContractId: stablePoolId, Action: "init", Payload: payload,
		RcLimit: 10_000, Intents: []contracts.Intent{}, Caller: stableOwner,
	})
	require.True(t, r.Success, "%s: %s", r.Err, r.ErrMsg)
	require.Equal(t, "hbd", ct.StateGet(stablePoolId, types.KeyAsset0Name))
	require.Equal(t, "3", ct.StateGet(stablePoolId, types.KeyDecimals0))
	require.Equal(t, "6", ct.StateGet(stablePoolId, types.KeyDecimals1))
	require.Equal(t, "250", ct.StateGet(stablePoolId, types.KeyAmp))
	require.Equal(t, types.CurveStable, ct.StateGet(stablePoolId, types.KeyCurve))
}

func TestStablePool_SwapsOnTheCurve(t *testing.T) {
	const liq = 100_000_000 // 100k HBD / 100k HIVE (3 decimals)
	const in = 10_000_000   // 10% of the pool
	user := "hive:stableuser"

	for _, dir := range [][2]string{{"hbd", "hive"}, {"hive", "hbd"}} {
		ct, _ := setupStableHbdHivePool(t, liq)
		cpCt, _, cpId := setupNativeHiveHbdPool(t, liq, liq) // fresh, balanced
		r := swapNative(t, &ct, stablePoolId, user, dir[0], dir[1], in)
		require.True(t, r.Success, "%s->%s stable swap: %s: %s", dir[0], dir[1], r.Err, r.ErrMsg)
		got := ct.GetBalance(user, ledger_db.Asset(dir[1]))

		rc := swapNative(t, &cpCt, cpId, "hive:cpuser"+dir[0], dir[0], dir[1], in)
		require.True(t, rc.Success, "cpmm swap: %s: %s", rc.Err, rc.ErrMsg)
		cp := cpCt.GetBalance("hive:cpuser"+dir[0], ledger_db.Asset(dir[1]))
		if dir[1] == "hbd" {
			got -= rcHeadroomHBD
			cp -= rcHeadroomHBD
		}
		t.Logf("%s->%s: stable %d vs constant product %d for %d in", dir[0], dir[1], got, cp, in)
		require.Greater(t, got, int64(in)*997/1000, "stable output within 0.3%% of par")
		require.Greater(t, got, cp, "stable beats the constant product on a 10%% swap")

		// Solvency: balance == reserve + system fee, both assets.
		contractAcct := "contract:" + stablePoolId
		for _, k := range [][3]string{{"hbd", types.KeyReserve0, types.KeySystemFee0}, {"hive", types.KeyReserve1, types.KeySystemFee1}} {
			bal := big.NewInt(ct.GetBalance(contractAcct, ledger_db.Asset(k[0])))
			want := new(big.Int).Add(stateBigInt(&ct, stablePoolId, k[1]), stateBigInt(&ct, stablePoolId, k[2]))
			require.Equal(t, 0, bal.Cmp(want), "%s: balance %s != reserve+fee %s", k[0], bal, want)
		}
	}
}

// The pool trades only on its curve: a node that does not list it (or a
// 0.12 node) answers on the constant product, and the pool refuses.
func TestStablePool_RefusedUnlessTheNetworkListsIt(t *testing.T) {
	ct, _ := setupStableHbdHivePool(t, 10_000_000)
	whitelistPendulum(&ct, stablePoolId) // the plain applier: no listing, no 0.13
	r := swapNative(t, &ct, stablePoolId, "hive:user1x", "hbd", "hive", 100_000)
	require.False(t, r.Success, "a constant-product answer must be refused by a stable pool")
	t.Logf("refused: %s", r.ErrMsg)

	listStable(&ct, 200) // listed with a different A: the node refuses
	r = swapNative(t, &ct, stablePoolId, "hive:user2x", "hbd", "hive", 100_000)
	require.False(t, r.Success, "an A mismatch must be refused")
	t.Logf("refused: %s", r.ErrMsg)

	listStable(&ct, 100)
	r = swapNative(t, &ct, stablePoolId, "hive:user3x", "hbd", "hive", 100_000)
	require.True(t, r.Success, "%s: %s", r.Err, r.ErrMsg)
}

// A constant-product pool on a node that lists ANOTHER pool as stable is
// untouched (and the listed id cannot be claimed by a constant-product pool).
func TestStablePool_ConstantProductPoolListedIsRefused(t *testing.T) {
	ct, _, cpId := setupNativeHiveHbdPool(t, 10_000_000, 10_000_000)
	ct.SetPendulumStablePools(testGeometry, knownDexPoolIDs, map[string]pendulumwasm.StableListing{
		cpId: {Amp: 100, Asset0: "hbd", Decimals0: 3, Asset1: "hive", Decimals1: 3},
	})
	r := swapNative(t, &ct, cpId, "hive:user4x", "hbd", "hive", 100_000)
	require.False(t, r.Success, "a listed pool that did not ask for the curve must be refused")
	ct.SetPendulumStablePools(testGeometry, knownDexPoolIDs, map[string]pendulumwasm.StableListing{})
	r = swapNative(t, &ct, cpId, "hive:user5x", "hbd", "hive", 100_000)
	require.True(t, r.Success, "%s: %s", r.Err, r.ErrMsg)
}

// DX-H7 on the stable pool: a host that echoes "stable" but pays more than
// the curve allows is refused by the pool's own floor.
func TestStablePool_LyingHostRefused(t *testing.T) {
	const liq = 10_000_000
	const in = 1_000_000
	ct, _ := setupStableHbdHivePool(t, liq)
	// The honest answer, from the real applier the pool is wired to.
	args := wasm_context.PendulumSwapFeeArgs{AssetIn: "hbd", AssetOut: "hive", X: in, XReserve: liq, YReserve: liq,
		Curve: `"stable"`, Amp: `"100"`, DecIn: `"3"`, DecOut: `"3"`}
	res := ct.PendulumApplier.ApplySwapFees(stablePoolId, "tx", 1, args, func(int64) error { return nil })
	require.True(t, res.IsOk(), "honest pricing")
	h := res.Unwrap()
	require.Equal(t, "stable", h.Curve)

	// Pay 1% more out of the reserve than the curve allows.
	extra := h.UserOutput / 100
	lie := h
	lie.UserOutput += extra
	lie.NewYReserve -= extra
	ct.PendulumApplier = &fakePendulumApplier{res: lie}
	r := swapNative(t, &ct, stablePoolId, "hive:liarxx", "hbd", "hive", in)
	require.False(t, r.Success, "a payout beyond the stable curve must be refused")
	require.Contains(t, r.ErrMsg+r.Ret, "stable curve floor")

	// The honest answer passes through the same stub.
	ct.PendulumApplier = &fakePendulumApplier{res: h}
	r = swapNative(t, &ct, stablePoolId, "hive:honestx", "hbd", "hive", in)
	require.True(t, r.Success, "%s: %s", r.Err, r.ErrMsg)

	// An honest-looking constant-product answer (no echo) is refused too.
	cp := h
	cp.Curve = ""
	ct.PendulumApplier = &fakePendulumApplier{res: cp}
	r = swapNative(t, &ct, stablePoolId, "hive:cpxxxx", "hbd", "hive", in)
	require.False(t, r.Success, "no stable echo must be refused")
}

// The target: USDC/HBD StableSwap pool (HBD 3 decimals, USDC 6, A=100) on the
// real EVM bridge, listed by the node. Both directions near par, and every
// unit accounted for on both ledgers (HBD on the Magi ledger, USDC on the
// bridge): balance == reserve + system fee.
func TestStablePool_UsdcHbdOnTheEvmBridge(t *testing.T) {
	ct := setupUsdcHbdPoolWith(t, types.InitParams{
		Asset0: "usdc", Asset0MappingContract: evmBridgeId, Decimals0: u64(6),
		Asset1: "hbd", Decimals1: u64(3),
		FeeBps: 8, RouterContract: usdcRouter, Curve: "stable", Amp: 100,
	}, func(ct *test_utils.ContractTest) {
		ct.SetPendulumStablePools(testGeometry, append(append([]string{}, knownDexPoolIDs...), usdcPoolId),
			map[string]pendulumwasm.StableListing{
				usdcPoolId: {Amp: 100, Asset0: "hbd", Decimals0: 3, Asset1: "usdc", Decimals1: 6},
			})
	})
	solvent := func(when string) {
		t.Helper()
		r0 := stateBigInt(&ct, usdcPoolId, types.KeyReserve0) // hbd
		f0 := stateBigInt(&ct, usdcPoolId, types.KeySystemFee0)
		r1 := stateBigInt(&ct, usdcPoolId, types.KeyReserve1) // usdc
		f1 := stateBigInt(&ct, usdcPoolId, types.KeySystemFee1)
		hbd := big.NewInt(ct.GetBalance("contract:"+usdcPoolId, ledger_db.Asset("hbd")))
		usdc, _ := new(big.Int).SetString(bridgeBalance(&ct, "contract:"+usdcPoolId, "usdc"), 10)
		require.Equal(t, 0, hbd.Cmp(new(big.Int).Add(r0, f0)), "%s: HBD balance %s != reserve %s + fee %s", when, hbd, r0, f0)
		require.Equal(t, 0, usdc.Cmp(new(big.Int).Add(r1, f1)), "%s: USDC balance %s != reserve %s + fee %s", when, usdc, r1, f1)
	}
	solvent("after liquidity")

	// 10k HBD -> USDC (10% of the pool).
	ct.Deposit(usdcTrader, 10_000_000+rcHeadroomHBD, ledger_db.Asset("hbd"))
	before, _ := strconv.ParseInt(bridgeBalance(&ct, usdcTrader, "usdc"), 10, 64)
	sw, _ := tinyjson.Marshal(types.SwapParams{AssetIn: "hbd", AmountIn: "10000000", AssetOut: "usdc", From: usdcTrader, To: usdcTrader})
	r := ct.Call(state_engine.TxVscCallContract{
		Self: *basicSelf(t, usdcTrader), ContractId: usdcPoolId, Action: "swap", Payload: sw, RcLimit: usdcRcLimit, Caller: usdcTrader,
		Intents: []contracts.Intent{{Type: "transfer.allow", Args: map[string]string{"token": "hbd", "limit": "10000000"}}},
	})
	require.True(t, r.Success, "HBD->USDC: %s: %s", r.Err, r.ErrMsg)
	after, _ := strconv.ParseInt(bridgeBalance(&ct, usdcTrader, "usdc"), 10, 64)
	got := after - before
	t.Logf("10,000 HBD -> %d.%06d USDC (%d RC)", got/1_000_000, got%1_000_000, r.RcUsed)
	require.Greater(t, got, int64(9_970_000_000), "within 0.3%% of par")
	require.Less(t, got, int64(10_000_000_000))
	solvent("after HBD->USDC")

	// 10k USDC -> HBD back.
	ra := bridgeCall(t, &ct, usdcTrader, "approve", map[string]string{"spender": "contract:" + usdcPoolId, "amount": "10000000000", "asset": "usdc"})
	require.True(t, ra.Success, "approve: %s: %s", ra.Err, ra.ErrMsg)
	hbdBefore := ct.GetBalance(usdcTrader, ledger_db.Asset("hbd"))
	sw, _ = tinyjson.Marshal(types.SwapParams{AssetIn: "usdc", AmountIn: "10000000000", AssetOut: "hbd", From: usdcTrader, To: usdcTrader})
	r = ct.Call(state_engine.TxVscCallContract{
		Self: *basicSelf(t, usdcTrader), ContractId: usdcPoolId, Action: "swap", Payload: sw, RcLimit: usdcRcLimit, Caller: usdcTrader,
	})
	require.True(t, r.Success, "USDC->HBD: %s: %s", r.Err, r.ErrMsg)
	gotHbd := ct.GetBalance(usdcTrader, ledger_db.Asset("hbd")) - hbdBefore
	t.Logf("10,000 USDC -> %d.%03d HBD (%d RC)", gotHbd/1000, gotHbd%1000, r.RcUsed)
	require.Greater(t, gotHbd, int64(9_970_000))
	solvent("after USDC->HBD")
}
