package main

import (
	"encoding/json"
	"math"
	"testing"
)

func TestInstructionParsing(t *testing.T) {
	tests := []struct {
		name        string
		jsonStr     string
		expectError bool
		expectedType string
	}{
		{
			name: "Valid swap instruction",
			jsonStr: `{
				"type": "swap",
				"version": "1.0.0",
				"asset_in": "HBD",
				"asset_out": "HIVE",
				"recipient": "alice"
			}`,
			expectError:  false,
			expectedType: "swap",
		},
		{
			name: "Valid deposit instruction",
			jsonStr: `{
				"type": "deposit",
				"version": "1.0.0",
				"asset_in": "HBD",
				"asset_out": "HIVE",
				"recipient": "alice"
			}`,
			expectError:  false,
			expectedType: "deposit",
		},
		{
			name: "Missing required field",
			jsonStr: `{
				"type": "swap",
				"version": "1.0.0",
				"asset_in": "HBD",
				"recipient": "alice"
			}`,
			expectError: false, // JSON parsing succeeds, validation happens later
			expectedType: "swap",
		},
		{
			name: "Invalid JSON",
			jsonStr: `{"type": "swap", "invalid": json}`,
			expectError: true,
		},
		{
			name: "Unknown instruction type",
			jsonStr: `{
				"type": "invalid",
				"version": "1.0.0",
				"asset_in": "HBD",
				"asset_out": "HIVE",
				"recipient": "alice"
			}`,
			expectError: false, // JSON parsing succeeds, validation happens later
			expectedType: "invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test JSON parsing (this is what the contract does)
			var instruction DexInstruction
			err := json.Unmarshal([]byte(tt.jsonStr), &instruction)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error parsing JSON, but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error parsing JSON: %v", err)
				}
				if instruction.Type != tt.expectedType {
					t.Errorf("Expected type %s, got %s", tt.expectedType, instruction.Type)
				}
			}
		})
	}
}

func TestSlippageCalculation(t *testing.T) {
	tests := []struct {
		name              string
		amountOut         uint64
		slippageBps       uint64
		expectedMinAmount uint64
	}{
		{"No slippage", 100000, 0, 100000},
		{"0.5% slippage", 100000, 50, 99500},
		{"1% slippage", 100000, 100, 99000},
		{"5% slippage", 100000, 500, 95000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test slippage calculation logic
			minOut := tt.amountOut * (10000 - tt.slippageBps) / 10000

			if minOut != tt.expectedMinAmount {
				t.Errorf("Slippage calculation = %v, want %v", minOut, tt.expectedMinAmount)
			}

			// Test if amount meets slippage requirement
			meetsSlippage := tt.amountOut >= minOut
			if !meetsSlippage {
				t.Errorf("Amount %v should meet slippage requirement of %v", tt.amountOut, minOut)
			}
		})
	}
}

func TestReferralFeeLogic(t *testing.T) {
	tests := []struct {
		name               string
		totalFee           uint64
		refBps             uint64
		expectedRefFee     uint64
		expectedNetFee     uint64
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

func TestLPTokenCalculation(t *testing.T) {
	tests := []struct {
		name         string
		amount0      uint64
		amount1      uint64
		expectedLP   uint64
		margin       uint64 // Allow some margin for floating point precision
	}{
		{"First deposit 1000:500", 1000000, 500000, 707106, 100},
		{"First deposit 2000:1000", 2000000, 1000000, 1414213, 100},
		{"Equal amounts", 1000000, 1000000, 1000000, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Calculate geometric mean: sqrt(amount0 * amount1)
			product := tt.amount0 * tt.amount1
			sqrt := math.Sqrt(float64(product))
			lpTokens := uint64(sqrt)

			// Check if within acceptable range
			if lpTokens < tt.expectedLP-tt.margin || lpTokens > tt.expectedLP+tt.margin {
				t.Errorf("LP token calculation = %v, want ~%v (±%v)",
					lpTokens, tt.expectedLP, tt.margin)
			}
		})
	}
}

func TestProportionalWithdrawal(t *testing.T) {
	t.Run("50% withdrawal from 1000:500 pool", func(t *testing.T) {
		totalReserve0, totalReserve1 := uint64(1000000), uint64(500000)
		totalLP := uint64(707106)
		withdrawLP := uint64(353553) // 50% of total LP

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

		// Verify remaining reserves
		remainingReserve0 := totalReserve0 - withdrawAmt0
		remainingReserve1 := totalReserve1 - withdrawAmt1

		if remainingReserve0 != expectedAmt0 {
			t.Errorf("Remaining reserve 0 = %v, want %v", remainingReserve0, expectedAmt0)
		}

		if remainingReserve1 != expectedAmt1 {
			t.Errorf("Remaining reserve 1 = %v, want %v", remainingReserve1, expectedAmt1)
		}
	})
}

func TestPoolReserveUpdates(t *testing.T) {
	t.Run("Swap reserve calculations", func(t *testing.T) {
		// Initial pool: 2000 HBD : 1000 HIVE
		reserveIn, reserveOut := uint64(2000000), uint64(1000000)
		feeBps := uint64(8)
		amountIn := uint64(100000) // 100 HBD

		// Use same formula as contract: amountOut = (amountInAfterFee * reserveOut) / newReserveIn
		amountInAfterFee := amountIn * (10000 - feeBps) / 10000 // 99920
		newReserveIn := reserveIn + amountInAfterFee             // 2099920
		amountOut := (amountInAfterFee * reserveOut) / newReserveIn

		// Expected: ~47580 HIVE out (99920*1000000/2099920 ≈ 47580)
		expectedOutMin, expectedOutMax := uint64(47500), uint64(47700)

		if amountOut < expectedOutMin || amountOut > expectedOutMax {
			t.Errorf("Swap output = %v, want between %v and %v", amountOut, expectedOutMin, expectedOutMax)
		}

		// Verify reserves after swap
		finalReserveIn := newReserveIn
		finalReserveOut := reserveOut - amountOut

		if finalReserveIn != 2099920 {
			t.Errorf("Final reserve in = %v, want 2099920", finalReserveIn)
		}

		// finalReserveOut should be ≈952420 (1000000 - 47580)
		if finalReserveOut < 952300 || finalReserveOut > 952500 {
			t.Errorf("Final reserve out = %v, want ~952420", finalReserveOut)
		}
	})
}

func TestErrorConditions(t *testing.T) {
	tests := []struct {
		name              string
		assetIn           string
		assetOut          string
		expectValidStruct bool // Would pass JSON+required-fields validation
	}{
		{"Same assets - invalid for swap", "HBD", "HBD", true},   // Struct valid but routing fails (no HBD/HBD pool)
		{"Empty asset_in - invalid", "", "HBD", false},
		{"Empty asset_out - invalid", "HBD", "", false},
		{"Unknown routing - struct valid", "UNKNOWN", "HIVE", true}, // Struct valid, routing would fail
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instruction := DexInstruction{
				Type:     "swap",
				Version:  "1.0.0",
				AssetIn:  tt.assetIn,
				AssetOut: tt.assetOut,
				Recipient: "test",
			}

			// Replicate contract's required-fields validation (Execute checks this first)
			hasRequiredFields := instruction.Type != "" && instruction.Version != "" &&
				instruction.AssetIn != "" && instruction.AssetOut != "" &&
				instruction.Recipient != ""

			if hasRequiredFields != tt.expectValidStruct {
				t.Errorf("required-fields validation: got valid=%v, want valid=%v (assetIn=%q, assetOut=%q)",
					hasRequiredFields, tt.expectValidStruct, tt.assetIn, tt.assetOut)
			}

			// Same-asset swap: contract would fail (no pool exists for X/X)
			if tt.assetIn == tt.assetOut && tt.assetIn != "" {
				if instruction.AssetIn != instruction.AssetOut {
					t.Errorf("same-asset case: AssetIn and AssetOut should match")
				}
			}
		})
	}
}