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
