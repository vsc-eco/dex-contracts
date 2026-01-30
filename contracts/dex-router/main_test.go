package main

import (
	"encoding/json"
	"math"
	"testing"
)

func TestAMMCalculations(t *testing.T) {
	t.Run("Constant product formula", func(t *testing.T) {
		// Test x * y = k
		// Initial: 2000 HBD * 1000 HIVE = 2,000,000
		// After adding 1000 HBD, should get ~333 HIVE out
		// (1000 * 1000) / (2000 + 1000) = 1000 - 666.67 = 333.33

		amountIn := uint64(100000)    // 100 HBD (with 2 decimals)
		reserveIn := uint64(2000000)  // 2000 HBD
		reserveOut := uint64(1000000) // 1000 HIVE
		feeBps := uint64(8)           // 0.08%

		amountOut := calculateSwapOutput(amountIn, reserveIn, reserveOut, feeBps, true)

		// Expected: ~48685 (after 0.08% fee)
		// Without fee: (100000 * 1000000) / (2000000 + 100000) = 100000000000 / 2100000 = 47619
		// With 0.08% fee: 100000 * 0.9992 = 99920 in
		// (99920 * 1000000) / (2000000 + 99920) = 99920000000 / 2099920 = 47571

		expectedMin := uint64(47500)
		expectedMax := uint64(47700)

		if amountOut < expectedMin || amountOut > expectedMax {
			t.Errorf("calculateSwapOutput() = %v, want between %v and %v", amountOut, expectedMin, expectedMax)
		}
	})

	t.Run("Large vs small swaps", func(t *testing.T) {
		reserveIn := uint64(1000000)
		reserveOut := uint64(1000000)
		feeBps := uint64(0)

		// Small swap: 1% of reserves
		smallSwap := calculateSwapOutput(10000, reserveIn, reserveOut, feeBps, true)
		// Large swap: 50% of reserves
		largeSwap := calculateSwapOutput(500000, reserveIn, reserveOut, feeBps, true)

		// Large swap should give worse price due to slippage
		priceImpactRatio := float64(largeSwap) / float64(smallSwap*50)
		if priceImpactRatio > 0.95 { // Should be noticeably worse
			t.Errorf("Large swap price impact too low: ratio = %v, want < 0.95", priceImpactRatio)
		}
	})
}

func TestSlippageProtection(t *testing.T) {
	tests := []struct {
		name           string
		amountOut      uint64
		amountIn       uint64
		slippageBps    uint64
		expectedResult uint64
	}{
		{"No slippage", 100000, 100000, 0, 100000},
		{"1% slippage", 100000, 100000, 100, 99000},
		{"5% slippage", 100000, 100000, 500, 95000},
		{"10% slippage", 100000, 100000, 1000, 90000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test slippage calculation logic
			minOut := tt.amountOut * (10000 - tt.slippageBps) / 10000

			if minOut != tt.expectedResult {
				t.Errorf("Slippage calculation = %v, want %v", minOut, tt.expectedResult)
			}

			// Test if amount meets slippage requirement
			meetsSlippage := tt.amountOut >= minOut
			if !meetsSlippage {
				t.Errorf("Amount %v should meet slippage requirement of %v", tt.amountOut, minOut)
			}
		})
	}
}

func TestFeeCalculations(t *testing.T) {
	tests := []struct {
		name        string
		amountIn    uint64
		feeBps      uint64
		expectedFee uint64
		expectedNet uint64
	}{
		{"No fee", 100000, 0, 0, 100000},
		{"0.08% fee", 100000, 8, 80, 99920},
		{"0.5% fee", 100000, 50, 500, 99500},
		{"1% fee", 100000, 100, 1000, 99000},
		{"10% fee", 100000, 1000, 10000, 90000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Calculate fee: amountIn * feeBps / 10000
			fee := tt.amountIn * tt.feeBps / 10000
			netAmount := tt.amountIn - fee

			if fee != tt.expectedFee {
				t.Errorf("Fee calculation = %v, want %v", fee, tt.expectedFee)
			}

			if netAmount != tt.expectedNet {
				t.Errorf("Net amount = %v, want %v", netAmount, tt.expectedNet)
			}
		})
	}
}

func TestReferralFeeCalculations(t *testing.T) {
	tests := []struct {
		name           string
		totalFee       uint64
		refBps         uint64
		expectedRefFee uint64
		expectedNetFee uint64
	}{
		{"No referral", 1000, 0, 0, 1000},
		{"2.5% referral", 1000, 250, 25, 975},
		{"10% referral", 1000, 1000, 100, 900},
		{"50% referral", 1000, 5000, 500, 500},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			refFee := tt.totalFee * tt.refBps / 10000
			netFee := tt.totalFee - refFee

			if refFee != tt.expectedRefFee {
				t.Errorf("Referral fee = %v, want %v", refFee, tt.expectedRefFee)
			}

			if netFee != tt.expectedNetFee {
				t.Errorf("Net fee = %v, want %v", netFee, tt.expectedNetFee)
			}
		})
	}
}

func TestJSONValidation(t *testing.T) {
	tests := []struct {
		name      string
		jsonStr   string
		shouldErr bool
		errMsg    string
	}{
		{
			"Valid swap instruction",
			`{
				"type": "swap",
				"version": "1.0.0",
				"asset_in": "HBD",
				"asset_out": "HIVE",
				"recipient": "alice",
				"min_amount_out": 1000
			}`,
			false,
			"",
		},
		{
			"Missing required field",
			`{
				"type": "swap",
				"version": "1.0.0",
				"asset_in": "HBD",
				"recipient": "alice"
			}`,
			false, // JSON parsing succeeds, validation happens later
			"",
		},
		{
			"Invalid JSON",
			`{"type": "swap", "invalid": json}`,
			true,
			"invalid json payload",
		},
		{
			"Unknown instruction type",
			`{
				"type": "invalid",
				"version": "1.0.0",
				"asset_in": "HBD",
				"asset_out": "HIVE",
				"recipient": "alice"
			}`,
			false, // JSON parsing succeeds, validation happens later
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test JSON parsing (this is what the contract does)
			var instruction DexInstruction
			err := json.Unmarshal([]byte(tt.jsonStr), &instruction)

			if tt.shouldErr {
				if err == nil {
					t.Errorf("Expected error parsing JSON, but got none")
				}
				// Note: In real contract, additional validation happens after JSON parsing
			} else {
				if err != nil {
					t.Errorf("Unexpected error parsing JSON: %v", err)
				}

				// For valid instructions, verify required fields are present
				hasRequiredFields := instruction.Type != "" && instruction.Version != "" &&
					instruction.AssetIn != "" && instruction.AssetOut != "" &&
					instruction.Recipient != ""
				if tt.name == "Valid swap instruction" && !hasRequiredFields {
					t.Errorf("Required fields validation should pass for valid instruction")
				}
				if tt.name == "Missing required field" && hasRequiredFields {
					t.Errorf("Instruction with missing field should fail validation")
				}
			}
		})
	}
}

func TestLiquidityMath(t *testing.T) {
	t.Run("LP token calculation", func(t *testing.T) {
		// First liquidity provision: geometric mean
		amt0, amt1 := uint64(1000000), uint64(500000) // 1000:500 ratio
		expectedLP := uint64(707106)                  // sqrt(1000000 * 500000) ≈ 707106

		// This would be calculated as: sqrt(amt0 * amt1)
		// But we can't test the actual function easily without mocking
		// Let's test the mathematical property
		product := amt0 * amt1
		sqrt := math.Sqrt(float64(product))
		result := uint64(sqrt)

		if result < expectedLP-1000 || result > expectedLP+1000 {
			t.Errorf("LP calculation ≈ %v, want ≈ %v", result, expectedLP)
		}
	})

	t.Run("Proportional withdrawal", func(t *testing.T) {
		totalReserve0, totalReserve1 := uint64(1000000), uint64(500000)
		totalLP := uint64(707106)
		withdrawLP := uint64(353553) // 50% of LP tokens

		// Calculate proportional amounts
		withdrawAmt0 := totalReserve0 * withdrawLP / totalLP
		withdrawAmt1 := totalReserve1 * withdrawLP / totalLP

		expectedAmt0 := uint64(500000) // 50% of 1000000
		expectedAmt1 := uint64(250000) // 50% of 500000

		if withdrawAmt0 != expectedAmt0 {
			t.Errorf("Withdraw amount 0 = %v, want %v", withdrawAmt0, expectedAmt0)
		}

		if withdrawAmt1 != expectedAmt1 {
			t.Errorf("Withdraw amount 1 = %v, want %v", withdrawAmt1, expectedAmt1)
		}
	})
}

func TestMathFunctions(t *testing.T) {
	t.Run("sqrt128", func(t *testing.T) {
		// Test sqrt(1000000 * 500000) = sqrt(500000000000) ≈ 707106
		// sqrt128(hi, lo) - 500000000000 fits in 64 bits, so hi=0
		result := sqrt128(0, 500000000000)
		expected := uint64(707106)

		if result != expected {
			t.Errorf("sqrt128(500000000000, 0) = %v, want %v", result, expected)
		}
	})

	t.Run("min64", func(t *testing.T) {
		if min64(10, 20) != 10 {
			t.Errorf("min64(10, 20) should be 10")
		}
		if min64(30, 20) != 20 {
			t.Errorf("min64(30, 20) should be 20")
		}
	})

	t.Run("max64", func(t *testing.T) {
		if max64(10, 20) != 20 {
			t.Errorf("max64(10, 20) should be 20")
		}
		if max64(30, 20) != 30 {
			t.Errorf("max64(30, 20) should be 30")
		}
	})
}
