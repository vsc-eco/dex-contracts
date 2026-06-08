package tests

// review8 DX-H1 — the pendulum host leaves the network-share credit inside the
// output reserve ("fees stay in pool implicitly"), and the swap handler accrued
// that same credit to the claimable system-fee bucket (f0/f1) WITHOUT removing
// it from the reserve. So the network credit was counted twice: once as
// LP-backed reserve, once as owner-claimable fee. claim_fees pays the fee out
// without touching reserves, so the last LP to withdraw is stranded.
//
// Solvency invariant: a pool's actual on-ledger balance of each asset must equal
// reserve + systemFee for that asset. The double-count breaks it
// (balance < reserve + fee by exactly the network credit). The fix restores it.

import (
	"math/big"
	"testing"

	"vsc-node/modules/db/vsc/contracts"
	ledger_db "vsc-node/modules/db/vsc/ledger"
	state_engine "vsc-node/modules/state-processing"

	"github.com/CosmWasm/tinyjson"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vsc-eco/dex-contracts/contracts/types"
)

func stateBigInt(ct interface {
	StateGet(string, string) string
}, contractId, key string) *big.Int {
	return new(big.Int).SetBytes([]byte(ct.StateGet(contractId, key)))
}

func TestReview8_DXH1_NetworkFeeNotDoubleCounted(t *testing.T) {
	// asset0=hbd, asset1=hive after the pool's alphabetical normalization.
	ct, _, dexId := setupNativeHiveHbdPool(t, 10_000_000, 10_000_000)
	owner := "hive:milo-hpr"

	// A large hbd->hive swap (input asset0, output asset1) so the pendulum fee
	// math yields a non-zero network-share credit on the hive (f1) side.
	ct.Deposit(owner, 4_000_000, ledger_db.Asset("hbd"))
	swapPayload, _ := tinyjson.Marshal(types.SwapParams{
		AssetIn:  "hbd",
		AmountIn: "4000000",
		AssetOut: "hive",
		From:     owner,
		To:       owner,
	})
	sr := ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(t, owner),
		ContractId: dexId,
		Action:     "swap",
		Payload:    swapPayload,
		RcLimit:    2000,
		Intents: []contracts.Intent{
			{Type: "transfer.allow", Args: map[string]string{"token": "hbd", "limit": "4000000"}},
		},
		Caller: owner,
	})
	require.True(t, sr.Success, "swap should succeed: %s: %s", sr.Err, sr.ErrMsg)

	contractAcct := "contract:" + dexId
	r0 := stateBigInt(&ct, dexId, types.KeyReserve0)
	r1 := stateBigInt(&ct, dexId, types.KeyReserve1)
	f0 := stateBigInt(&ct, dexId, types.KeySystemFee0)
	f1 := stateBigInt(&ct, dexId, types.KeySystemFee1)
	balHbd := big.NewInt(ct.GetBalance(contractAcct, ledger_db.Asset("hbd")))
	balHive := big.NewInt(ct.GetBalance(contractAcct, ledger_db.Asset("hive")))

	t.Logf("r0(hbd)=%s f0=%s balHbd=%s", r0, f0, balHbd)
	t.Logf("r1(hive)=%s f1(network credit)=%s balHive=%s", r1, f1, balHive)

	// Sanity: the swap actually produced a network-share fee to double-count.
	require.Equal(t, 1, f1.Sign(), "test needs a non-zero network credit on the output (hive) side")

	// Solvency invariant: balance == reserve + systemFee, per asset.
	// Pre-fix the hive side fails by exactly the network credit (f1): the reserve
	// still contained it AND it was also booked as a claimable fee.
	wantHive := new(big.Int).Add(r1, f1)
	assert.Equal(t, 0, balHive.Cmp(wantHive),
		"DX-H1: hive balance (%s) must equal reserve1+f1 (%s); a deficit of f1 (%s) is the double-count",
		balHive, wantHive, f1)

	wantHbd := new(big.Int).Add(r0, f0)
	assert.Equal(t, 0, balHbd.Cmp(wantHbd),
		"hbd balance (%s) must equal reserve0+f0 (%s)", balHbd, wantHbd)
}

func TestReview8_DXH1_LastLpSolventAfterClaimFees(t *testing.T) {
	ct, _, dexId := setupNativeHiveHbdPool(t, 10_000_000, 10_000_000)
	owner := "hive:milo-hpr"

	// Generate a network fee.
	ct.Deposit(owner, 4_000_000, ledger_db.Asset("hbd"))
	swapPayload, _ := tinyjson.Marshal(types.SwapParams{
		AssetIn: "hbd", AmountIn: "4000000", AssetOut: "hive", From: owner, To: owner,
	})
	sr := ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(t, owner),
		ContractId: dexId,
		Action:     "swap",
		Payload:    swapPayload,
		RcLimit:    2000,
		Intents:    []contracts.Intent{{Type: "transfer.allow", Args: map[string]string{"token": "hbd", "limit": "4000000"}}},
		Caller:     owner,
	})
	require.True(t, sr.Success, "swap should succeed: %s: %s", sr.Err, sr.ErrMsg)

	// Owner claims the system fees.
	cr := ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(t, owner),
		ContractId: dexId,
		Action:     "claim_fees",
		Payload:    []byte("{}"),
		RcLimit:    2000,
		Intents:    []contracts.Intent{},
		Caller:     owner,
	})
	require.True(t, cr.Success, "claim_fees should succeed: %s: %s", cr.Err, cr.ErrMsg)

	// After claiming, the invariant must still hold: the pool still backs exactly
	// its reserves (the claimed fee left the reserve too). The last LP removing
	// ALL liquidity must therefore succeed — pre-fix the reserve overstated the
	// pool by the claimed network credit, so this withdrawal is short.
	pr := ct.Call(state_engine.TxVscCallContract{
		Self: *basicSelf(t, owner), ContractId: dexId, Action: "get_pool",
		Payload: []byte("{}"), RcLimit: 2000, Intents: []contracts.Intent{}, Caller: owner,
	})
	require.True(t, pr.Success, "get_pool failed: %s", pr.ErrMsg)
	var pool types.PoolInfo
	require.NoError(t, tinyjson.Unmarshal([]byte(pr.Ret), &pool))
	removePayload, _ := tinyjson.Marshal(types.RemoveLiquidityParams{LpAmount: pool.TotalLp, Recipient: owner})
	rr := ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(t, owner),
		ContractId: dexId,
		Action:     "remove_liquidity",
		Payload:    removePayload,
		RcLimit:    2000,
		Intents:    []contracts.Intent{},
		Caller:     owner,
	})
	require.True(t, rr.Success, "DX-H1: removing all liquidity after claim_fees must succeed (pool solvent): %s: %s", rr.Err, rr.ErrMsg)

	contractAcct := "contract:" + dexId
	balHive := ct.GetBalance(contractAcct, ledger_db.Asset("hive"))
	balHbd := ct.GetBalance(contractAcct, ledger_db.Asset("hbd"))
	r0 := stateBigInt(&ct, dexId, types.KeyReserve0)
	r1 := stateBigInt(&ct, dexId, types.KeyReserve1)
	f0 := stateBigInt(&ct, dexId, types.KeySystemFee0)
	f1 := stateBigInt(&ct, dexId, types.KeySystemFee1)
	t.Logf("after full removal: balHbd=%d r0=%s f0=%s | balHive=%d r1=%s f1=%s", balHbd, r0, f0, balHive, r1, f1)

	// Everything is claimed/withdrawn: reserves and fees are zero and the residual
	// balance equals the still-claimable fees (zero here).
	assert.Equal(t, int64(0), balHive-(r1.Int64()+f1.Int64()), "hive must reconcile after full removal")
	assert.Equal(t, int64(0), balHbd-(r0.Int64()+f0.Int64()), "hbd must reconcile after full removal")
}
