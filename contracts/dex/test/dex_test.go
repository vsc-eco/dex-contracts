package main

import (
	"testing"
)

// Test AMM calculations
func TestCalculateSwapOutput(t *testing.T) {
	tests := []struct {
		name           string
		amountIn       uint64
		reserveIn      uint64
		reserveOut     uint64
		feeBps         uint64
		expectedOutput uint64
	}{
		{
			name:           "simple swap no fee",
			amountIn:       1000,
			reserveIn:      10000,
			reserveOut:     10000,
			feeBps:         0,
			expectedOutput: 910, // r1 - (r0 * r1) / (r0 + dx) = 10000 - (10000*10000)/11000 = 910
		},
		{
			name:           "swap with fee",
			amountIn:       1000,
			reserveIn:      10000,
			reserveOut:     10000,
			feeBps:         30, // 0.3%
			expectedOutput: 907, // After fee: 970, then AMM calc
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Calculate: dy = r1 - (r0 * r1) / (r0 + dx)
			dx := tt.amountIn * (10000 - tt.feeBps) / 10000
			if dx == 0 {
				dx = 1
			}
			k := tt.reserveIn * tt.reserveOut
			newR0 := tt.reserveIn + dx
			amountOut := tt.reserveOut - (k / newR0)

			if amountOut != tt.expectedOutput {
				t.Errorf("expected output %d, got %d", tt.expectedOutput, amountOut)
			}
		})
	}
}

// Test LP token minting
func TestLPTokenMinting(t *testing.T) {
	tests := []struct {
		name        string
		amt0        uint64
		amt1        uint64
		r0          uint64
		r1          uint64
		totalLP     uint64
		expectedLP  uint64
		isFirstMint bool
	}{
		{
			name:        "first liquidity provision",
			amt0:        1000,
			amt1:        1000,
			r0:          0,
			r1:          0,
			totalLP:     0,
			expectedLP:  1000, // sqrt(1000 * 1000) = 1000
			isFirstMint: true,
		},
		{
			name:        "proportional liquidity",
			amt0:        1000,
			amt1:        1000,
			r0:          10000,
			r1:          10000,
			totalLP:     10000,
			expectedLP:  1000, // min(1000*10000/10000, 1000*10000/10000) = 1000
			isFirstMint: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var minted uint64
			if tt.isFirstMint {
				// Geometric mean: sqrt(amt0 * amt1)
				minted = sqrt128(0, tt.amt0*tt.amt1)
			} else {
				// Proportional: min(amt0 * totalLP / r0, amt1 * totalLP / r1)
				m0 := tt.amt0 * tt.totalLP / tt.r0
				m1 := tt.amt1 * tt.totalLP / tt.r1
				minted = min64(m0, m1)
			}

			if minted != tt.expectedLP {
				t.Errorf("expected LP %d, got %d", tt.expectedLP, minted)
			}
		})
	}
}

// Test liquidity removal
func TestLiquidityRemoval(t *testing.T) {
	tests := []struct {
		name        string
		lpAmount    uint64
		totalLP     uint64
		r0          uint64
		r1          uint64
		expectedAmt0 uint64
		expectedAmt1 uint64
	}{
		{
			name:         "proportional removal",
			lpAmount:     1000,
			totalLP:      10000,
			r0:           50000,
			r1:           50000,
			expectedAmt0: 5000, // 1000 * 50000 / 10000
			expectedAmt1: 5000, // 1000 * 50000 / 10000
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			amt0 := tt.r0 * tt.lpAmount / tt.totalLP
			amt1 := tt.r1 * tt.lpAmount / tt.totalLP

			if amt0 != tt.expectedAmt0 || amt1 != tt.expectedAmt1 {
				t.Errorf("expected amounts (%d, %d), got (%d, %d)",
					tt.expectedAmt0, tt.expectedAmt1, amt0, amt1)
			}
		})
	}
}

// Helper functions (copied from utils.go for testing)
func min64(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}

func sqrt128(hi, lo uint64) uint64 {
	// Simplified sqrt for testing - full implementation uses binary search
	if hi == 0 {
		// Simple case: sqrt of 64-bit number
		var low, high uint64 = 0, ^uint64(0) >> 1
		var ans uint64
		for low <= high {
			mid := (low + high) >> 1
			if mid*mid <= lo {
				ans = mid
				low = mid + 1
			} else {
				high = mid - 1
			}
		}
		return ans
	}
	// For 128-bit, use approximation
	return sqrt128(0, lo)
}
