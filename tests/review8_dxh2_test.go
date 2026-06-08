package tests

// review8 DX-H2 — defense-in-depth against a compromised pendulum host (the
// applier runs in consensus; CRIT-3 cartel). The contract stores reserves via
// setBigInt, which writes the big.Int MAGNITUDE, so a host returning a NEGATIVE
// NewYReserve was silently persisted as its positive value, corrupting the pool.
// The contract had no sign check.

import (
	"testing"

	"vsc-node/lib/test_utils"
	"vsc-node/modules/db/vsc/contracts"
	ledger_db "vsc-node/modules/db/vsc/ledger"
	state_engine "vsc-node/modules/state-processing"
	wasm_context "vsc-node/modules/wasm/context"

	"github.com/CosmWasm/tinyjson"
	"github.com/JustinKnueppel/go-result"
	"github.com/stretchr/testify/assert"

	"github.com/vsc-eco/dex-contracts/contracts/types"
)

// fakePendulumApplier returns a fixed, attacker-chosen swap-fee result,
// modelling a compromised consensus host. Shared by the DX-H2 and DX-H7 tests.
type fakePendulumApplier struct {
	res wasm_context.PendulumSwapFeeResult
}

func (f *fakePendulumApplier) ApplySwapFees(
	contractID, txID string,
	blockHeight uint64,
	args wasm_context.PendulumSwapFeeArgs,
	accrueNodeBucket wasm_context.AccrueNodeBucketFn,
) result.Result[wasm_context.PendulumSwapFeeResult] {
	return result.Ok(f.res)
}

func swapHbdToHiveWithMaliciousHost(t *testing.T, res wasm_context.PendulumSwapFeeResult) *test_utils.ContractTestCallResult {
	t.Helper()
	ct, _, dexId := setupNativeHiveHbdPool(t, 1_000_000, 1_000_000)
	owner := "hive:milo-hpr"
	ct.Deposit(owner, 10_000, ledger_db.Asset("hbd"))

	// Swap into the pool, but the (malicious) host dictates the post-swap reserves.
	ct.PendulumApplier = &fakePendulumApplier{res: res}

	swapPayload, _ := tinyjson.Marshal(types.SwapParams{
		AssetIn: "hbd", AmountIn: "10000", AssetOut: "hive", From: owner, To: owner,
	})
	r := ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(t, owner),
		ContractId: dexId,
		Action:     "swap",
		Payload:    swapPayload,
		RcLimit:    2000,
		Intents:    []contracts.Intent{{Type: "transfer.allow", Args: map[string]string{"token": "hbd", "limit": "10000"}}},
		Caller:     owner,
	})
	return &r
}

func TestReview8_DXH2_NegativeReserveRejected(t *testing.T) {
	// Host returns a NEGATIVE output reserve. Pre-fix setBigInt stored +42; now
	// the sign check aborts the swap.
	r := swapHbdToHiveWithMaliciousHost(t, wasm_context.PendulumSwapFeeResult{
		UserOutput:          100,
		NewXReserve:         1_010_000, // plausible input reserve (rIn + amountIn)
		NewYReserve:         -42,       // malicious: sign would be dropped on store
		NetworkCreditOutput: 0,
	})
	assert.False(t, r.Success,
		"DX-H2: a negative reserve from the host must be rejected, not stored as its magnitude")
}
