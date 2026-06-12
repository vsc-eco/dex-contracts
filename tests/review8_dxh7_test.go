package tests

// review8 DX-H7 — defense-in-depth against a compromised pendulum host (CRIT-3
// cartel). Nothing stopped the host from collapsing the constant product (e.g.
// returning 1/1 reserves), which mis-prices the pool so the next swapper drains
// it. The contract had no k post-condition; it now requires the input reserve to
// not shrink and the output reserve to stay at/above the constant-product floor.
// The malicious-host stub lives in review8_dxh2_test.go (same package).

import (
	"testing"

	wasm_context "vsc-node/modules/wasm/context"

	"github.com/stretchr/testify/assert"
)

func TestReview8_DXH7_ConstantProductCollapseRejected(t *testing.T) {
	// Host collapses the reserves to 1/1 (k: 1e12 -> 1). Pre-fix the contract
	// stored them; now the constant-product post-condition aborts the swap.
	r := swapHbdToHiveWithMaliciousHost(t, wasm_context.PendulumSwapFeeResult{
		UserOutput:          100,
		NewXReserve:         1, // collapsed
		NewYReserve:         1, // collapsed
		NetworkCreditOutput: 0,
	})
	assert.False(t, r.Success,
		"DX-H7: collapsed reserves (constant-product drop) from the host must be rejected")
}

// Isolates the output-reserve floor check: the input reserve is honest
// (rIn + amountIn) so the `newRIn < rIn` guard does NOT fire first; only the
// constant-product floor branch can reject this. Pool is 1_000_000/1_000_000,
// amountIn 10_000, so the floor is floor(1e6*1e6/1_010_000) = 990_099.
func TestReview8_DXH7_OutputBelowFloorRejected(t *testing.T) {
	r := swapHbdToHiveWithMaliciousHost(t, wasm_context.PendulumSwapFeeResult{
		UserOutput:          100,
		NewXReserve:         1_010_000, // honest: rIn + amountIn (passes the shrink guard)
		NewYReserve:         990_098,   // one below the constant-product floor
		NetworkCreditOutput: 0,
	})
	assert.False(t, r.Success,
		"DX-H7: an output reserve below the constant-product floor must be rejected")
}

// The reserve bounds are necessary but not sufficient: a host can return
// PLAUSIBLE reserves (input grown, output at the floor) while inflating
// user_output to drain the pool. The payout cap (user_output <= rOut - newROut)
// must reject this even though both reserve post-conditions pass.
func TestReview8_DXH7_InflatedUserOutputRejected(t *testing.T) {
	r := swapHbdToHiveWithMaliciousHost(t, wasm_context.PendulumSwapFeeResult{
		UserOutput:          500_000,   // honest output is ~9_800; this is 50x the reserve drop
		NewXReserve:         1_010_000, // honest-looking (rIn + amountIn)
		NewYReserve:         990_099,   // exactly the constant-product floor — passes the floor check
		NetworkCreditOutput: 0,
	})
	assert.False(t, r.Success,
		"DX-H7: a user_output exceeding the output-reserve decrease must be rejected (pool drain)")
}
