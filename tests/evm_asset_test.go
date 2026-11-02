package tests

// SP-0: a pool whose asset lives on the EVM bridge (multi-asset: "eth" and
// registered token symbols). The bridge's transfer / transferFrom refuse a
// call without an "asset" (isAllowedAsset("") is false), so before the fix
// every pool draw and payout of USDC aborted "unknown asset". This runs the
// REAL bridge wasm (EVM_BRIDGE_WASM, default: the parity devnet build) next
// to the real pool wasm.

import (
	"encoding/json"
	"os"
	"strconv"
	"testing"

	"vsc-node/lib/test_utils"
	"vsc-node/modules/db/vsc/contracts"
	ledger_db "vsc-node/modules/db/vsc/ledger"
	state_engine "vsc-node/modules/state-processing"

	"github.com/CosmWasm/tinyjson"
	"github.com/stretchr/testify/require"

	dexcontracts "github.com/vsc-eco/dex-contracts"
	"github.com/vsc-eco/dex-contracts/contracts/types"
)

const (
	evmBridgeId  = "vsc1Bo8h3TMFAYX5rkgCeauXFywZGiMbUftQL2"
	usdcPoolId   = "vsc1Bnuikc8sJii5baG5gmxno4V2xTW7joi2vu"
	usdcLp       = "hive:usdclpxx"
	usdcTrader   = "hive:usdctrader"
	usdcPoolLiq  = 100_000_000_000 // 100k USDC (6 decimals)
	hbdPoolLiq   = 100_000_000     // 100k HBD (3 decimals)
	usdcRcLimit  = 20_000
	evmBridgeEnv = "EVM_BRIDGE_WASM"
	usdcOwner    = "hive:milo-hpr"
	usdcRouter   = "vsc1Bpc3SgDqCRQxzeDrvV7T4XKV6BZuHmME5F"
)

func evmBridgeWasm(t *testing.T) []byte {
	t.Helper()
	path := os.Getenv(evmBridgeEnv)
	if path == "" {
		path = os.ExpandEnv("$HOME/evm-fix/gvn-parity-devnet/.devnet/evm-artifacts-parity/evm-bridge.wasm")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("EVM bridge wasm not available (%s): %v", path, err)
	}
	return b
}

func bridgeBalance(ct *test_utils.ContractTest, who, asset string) string {
	return ct.StateGet(evmBridgeId, "a-"+who+"-"+asset)
}

func bridgeCall(t *testing.T, ct *test_utils.ContractTest, caller, action string, payload any) test_utils.ContractTestCallResult {
	t.Helper()
	b, err := json.Marshal(payload)
	require.NoError(t, err)
	return ct.Call(state_engine.TxVscCallContract{
		Self: *basicSelf(t, caller), ContractId: evmBridgeId, Action: action, Payload: b,
		RcLimit: usdcRcLimit, Intents: []contracts.Intent{}, Caller: caller,
	})
}

func setupUsdcHbdPool(t *testing.T) test_utils.ContractTest {
	t.Helper()
	return setupUsdcHbdPoolWith(t, types.InitParams{
		Asset0: "usdc", Asset0MappingContract: evmBridgeId, Asset1: "hbd", FeeBps: 8, RouterContract: usdcRouter,
	}, func(ct *test_utils.ContractTest) { whitelistPendulum(ct, usdcPoolId) })
}

// setupUsdcHbdPoolWith: a USDC/HBD pool initialized with `init` on the real
// EVM bridge, its applier wired by `wire`, seeded with 100k/100k.
func setupUsdcHbdPoolWith(t *testing.T, init types.InitParams, wire func(*test_utils.ContractTest)) test_utils.ContractTest {
	t.Helper()
	requireWasm(t, "dex", dexcontracts.DexWasm)
	bridge := evmBridgeWasm(t)
	ct := test_utils.NewContractTest()
	wire(&ct)
	t.Cleanup(func() { ct.DataLayer.Stop() })
	ct.RegisterContract(evmBridgeId, usdcOwner, bridge)
	ct.RegisterContract(usdcPoolId, usdcOwner, dexcontracts.DexWasm)
	ct.RegisterContract(usdcRouter, usdcOwner, dexcontracts.DexRouterV2Wasm)

	// The bridge after registerToken("usdc") and two credited deposits.
	ct.StateSet(evmBridgeId, "as-usdc", "1")
	ct.StateSet(evmBridgeId, "a-"+usdcLp+"-usdc", strconv.Itoa(usdcPoolLiq))
	ct.StateSet(evmBridgeId, "a-"+usdcTrader+"-usdc", "10000000000") // 10k USDC

	payload, _ := tinyjson.Marshal(&init)
	r := ct.Call(state_engine.TxVscCallContract{
		Self: *basicSelf(t, usdcOwner), ContractId: usdcPoolId, Action: "init", Payload: payload,
		RcLimit: usdcRcLimit, Intents: []contracts.Intent{}, Caller: usdcOwner,
	})
	require.True(t, r.Success, "init: %s: %s", r.Err, r.ErrMsg)

	// LP approves the pool on the bridge, then adds liquidity (hbd is asset0).
	r = bridgeCall(t, &ct, usdcLp, "approve", map[string]string{"spender": "contract:" + usdcPoolId, "amount": strconv.Itoa(usdcPoolLiq), "asset": "usdc"})
	require.True(t, r.Success, "approve: %s: %s", r.Err, r.ErrMsg)
	ct.Deposit(usdcLp, hbdPoolLiq+rcHeadroomHBD, ledger_db.Asset("hbd"))
	lp, _ := tinyjson.Marshal(types.AddLiquidityParams{
		Amount0: strconv.Itoa(hbdPoolLiq), Amount1: strconv.Itoa(usdcPoolLiq), Recipient: usdcLp,
	})
	r = ct.Call(state_engine.TxVscCallContract{
		Self: *basicSelf(t, usdcLp), ContractId: usdcPoolId, Action: "add_liquidity", Payload: lp,
		RcLimit: usdcRcLimit, Caller: usdcLp,
		Intents: []contracts.Intent{{Type: "transfer.allow", Args: map[string]string{"token": "hbd", "limit": strconv.Itoa(hbdPoolLiq)}}},
	})
	require.True(t, r.Success, "SP-0: add_liquidity drawing USDC from the EVM bridge: %s: %s", r.Err, r.ErrMsg)
	require.Equal(t, strconv.Itoa(usdcPoolLiq), bridgeBalance(&ct, "contract:"+usdcPoolId, "usdc"), "pool's USDC on the bridge")
	require.Equal(t, "0", bridgeBalance(&ct, usdcLp, "usdc"))
	return ct
}

func TestSP0_PoolDrawsAndPaysUsdcOnTheEvmBridge(t *testing.T) {
	ct := setupUsdcHbdPool(t)

	// HBD -> USDC: the pool pays USDC out through the bridge's transfer.
	ct.Deposit(usdcTrader, 1_000_000+rcHeadroomHBD, ledger_db.Asset("hbd"))
	sw, _ := tinyjson.Marshal(types.SwapParams{AssetIn: "hbd", AmountIn: "1000000", AssetOut: "usdc", From: usdcTrader, To: usdcTrader})
	r := ct.Call(state_engine.TxVscCallContract{
		Self: *basicSelf(t, usdcTrader), ContractId: usdcPoolId, Action: "swap", Payload: sw,
		RcLimit: usdcRcLimit, Caller: usdcTrader,
		Intents: []contracts.Intent{{Type: "transfer.allow", Args: map[string]string{"token": "hbd", "limit": "1000000"}}},
	})
	require.True(t, r.Success, "SP-0: swap paying USDC: %s: %s", r.Err, r.ErrMsg)
	got, _ := strconv.ParseInt(bridgeBalance(&ct, usdcTrader, "usdc"), 10, 64)
	require.Greater(t, got, int64(10_000_000_000), "trader received USDC on the bridge")

	// USDC -> HBD: the pool draws USDC through transferFrom.
	r = bridgeCall(t, &ct, usdcTrader, "approve", map[string]string{"spender": "contract:" + usdcPoolId, "amount": "500000000", "asset": "usdc"})
	require.True(t, r.Success, "approve: %s: %s", r.Err, r.ErrMsg)
	hbdBefore := ct.GetBalance(usdcTrader, ledger_db.Asset("hbd"))
	sw, _ = tinyjson.Marshal(types.SwapParams{AssetIn: "usdc", AmountIn: "500000000", AssetOut: "hbd", From: usdcTrader, To: usdcTrader})
	r = ct.Call(state_engine.TxVscCallContract{
		Self: *basicSelf(t, usdcTrader), ContractId: usdcPoolId, Action: "swap", Payload: sw,
		RcLimit: usdcRcLimit, Caller: usdcTrader, Intents: []contracts.Intent{},
	})
	require.True(t, r.Success, "SP-0: swap drawing USDC: %s: %s", r.Err, r.ErrMsg)
	require.Greater(t, ct.GetBalance(usdcTrader, ledger_db.Asset("hbd")), hbdBefore, "trader received HBD")
}

// SP-0b + SP-0 on the router: register USDC from the EVM bridge (getInfo by
// symbol), register the pool, and route swaps both ways. The router pre-funds
// a mapped input with the bridge's transferFrom, so it needs the asset field
// too. This is also the path an EVM deposit-and-swap takes (the bridge calls
// router.execute after granting the router an allowance).
func TestSP0b_RouterRegistersAndRoutesUsdc(t *testing.T) {
	ct := setupUsdcHbdPool(t)
	router := &RouterInfo{ct: &ct, id: usdcRouter}
	r := router.initRouterV2(t, usdcOwner)
	require.True(t, r.Success, "init router: %s: %s", r.Err, r.ErrMsg)

	// The bridge's token registry after registerToken(usdc, 6 decimals).
	ref := "11155111-1c7d4b196cb0c7b01d743fbc6116a902379c7238"
	ct.StateSet(evmBridgeId, "ts-usdc", ref)
	ct.StateSet(evmBridgeId, "t-"+ref, "usdc|6|1")

	r = router.registerToken(t, usdcOwner, types.RegisterTokenParams{Name: "HBD", TokenInfo: types.TokenInfo{Chain: "HIVE"}})
	require.True(t, r.Success, "register hbd: %s: %s", r.Err, r.ErrMsg)
	// register_token reads getInfo from the bridge (a large wasm): over the
	// helper's 2000 RC cap.
	regPayload, _ := tinyjson.Marshal(types.RegisterTokenParams{Name: "usdc", TokenInfo: types.TokenInfo{Chain: "ETH", MappingContract: evmBridgeId}})
	reg := ct.Call(state_engine.TxVscCallContract{
		Self: *basicSelf(t, usdcOwner), ContractId: usdcRouter, Action: "register_token", Payload: regPayload,
		RcLimit: usdcRcLimit, Intents: []contracts.Intent{}, Caller: usdcOwner,
	})
	require.True(t, reg.Success, "SP-0b: register usdc from the EVM bridge: %s: %s", reg.Err, reg.ErrMsg)
	t.Logf("register_token usdc: %d RC", reg.RcUsed)
	r = router.registerPool(t, usdcOwner, types.RegisterPoolParams{Asset0: "hbd", Asset1: "usdc", DexContractId: usdcPoolId})
	require.True(t, r.Success, "register pool: %s: %s", r.Err, r.ErrMsg)

	// USDC -> HBD through the router: the router pulls USDC from the trader.
	r2 := bridgeCall(t, &ct, usdcTrader, "approve", map[string]string{"spender": "contract:" + usdcRouter, "amount": "300000000", "asset": "usdc"})
	require.True(t, r2.Success, "approve router: %s: %s", r2.Err, r2.ErrMsg)
	ct.Deposit(usdcTrader, rcHeadroomHBD, ledger_db.Asset("hbd"))
	hbdBefore := ct.GetBalance(usdcTrader, ledger_db.Asset("hbd"))
	r = router.execute(t, usdcTrader, &types.DexInstruction{
		Type: "swap", Version: "1.0.0", AssetIn: "usdc", AssetOut: "hbd", AmountIn: "300000000", Recipient: usdcTrader,
	}, []contracts.Intent{})
	require.True(t, r.Success, "SP-0: routed USDC->HBD: %s: %s", r.Err, r.ErrMsg)
	require.Greater(t, ct.GetBalance(usdcTrader, ledger_db.Asset("hbd")), hbdBefore)

	// HBD -> USDC through the router.
	ct.Deposit(usdcTrader, 200_000, ledger_db.Asset("hbd"))
	usdcBefore, _ := strconv.ParseInt(bridgeBalance(&ct, usdcTrader, "usdc"), 10, 64)
	r = router.execute(t, usdcTrader, &types.DexInstruction{
		Type: "swap", Version: "1.0.0", AssetIn: "hbd", AssetOut: "usdc", AmountIn: "200000", Recipient: usdcTrader,
	}, []contracts.Intent{{Type: "transfer.allow", Args: map[string]string{"token": "hbd", "limit": "200000"}}})
	require.True(t, r.Success, "routed HBD->USDC: %s: %s", r.Err, r.ErrMsg)
	usdcAfter, _ := strconv.ParseInt(bridgeBalance(&ct, usdcTrader, "usdc"), 10, 64)
	require.Greater(t, usdcAfter, usdcBefore)
}
