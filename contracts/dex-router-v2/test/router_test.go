package main

import (
	"testing"
)

// Test pool registration
func TestPoolRegistration(t *testing.T) {
	tests := []struct {
		name         string
		asset0       string
		asset1       string
		dexContract  string
		shouldNormalize bool
	}{
		{
			name:         "normal order",
			asset0:       "HBD",
			asset1:       "HIVE",
			dexContract:  "dex-hbd-hive-123",
			shouldNormalize: false,
		},
		{
			name:         "reverse order should normalize",
			asset0:       "HIVE",
			asset1:       "HBD",
			dexContract:  "dex-hbd-hive-123",
			shouldNormalize: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Normalize asset order (alphabetical)
			asset0, asset1 := tt.asset0, tt.asset1
			if asset0 > asset1 {
				asset0, asset1 = asset1, asset0
			}

			// Expected pool key
			expectedKey := "pool/" + asset0 + "/" + asset1

			if expectedKey != "pool/HBD/HIVE" {
				t.Errorf("expected normalized key pool/HBD/HIVE, got %s", expectedKey)
			}
		})
	}
}

// Test pool finding logic
func TestFindPool(t *testing.T) {
	tests := []struct {
		name      string
		assetA    string
		assetB    string
		pools     map[string]string // pool key -> contract ID
		expected  string
		shouldFind bool
	}{
		{
			name:   "find existing pool",
			assetA: "HBD",
			assetB: "HIVE",
			pools: map[string]string{
				"pool/HBD/HIVE": "dex-hbd-hive-123",
			},
			expected:  "dex-hbd-hive-123",
			shouldFind: true,
		},
		{
			name:   "find pool with reversed assets",
			assetA: "HIVE",
			assetB: "HBD",
			pools: map[string]string{
				"pool/HBD/HIVE": "dex-hbd-hive-123",
			},
			expected:  "dex-hbd-hive-123",
			shouldFind: true,
		},
		{
			name:   "pool not found",
			assetA: "BTC",
			assetB: "ETH",
			pools: map[string]string{
				"pool/HBD/HIVE": "dex-hbd-hive-123",
			},
			expected:  "",
			shouldFind: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Normalize asset order
			asset0, asset1 := tt.assetA, tt.assetB
			if asset0 > asset1 {
				asset0, asset1 = asset1, asset0
			}

			poolKey := "pool/" + asset0 + "/" + asset1
			contractID, found := tt.pools[poolKey]

			if found != tt.shouldFind {
				t.Errorf("expected found=%v, got found=%v", tt.shouldFind, found)
			}

			if found && contractID != tt.expected {
				t.Errorf("expected contract ID %s, got %s", tt.expected, contractID)
			}
		})
	}
}

// Test two-hop routing logic
func TestTwoHopRouting(t *testing.T) {
	tests := []struct {
		name        string
		assetIn     string
		assetOut    string
		pools       map[string]string
		expectedHop1 string
		expectedHop2 string
		shouldRoute bool
	}{
		{
			name:   "route BTC -> HIVE via HBD",
			assetIn: "BTC",
			assetOut: "HIVE",
			pools: map[string]string{
				"pool/BTC/HBD":  "dex-btc-hbd-123",
				"pool/HBD/HIVE": "dex-hbd-hive-456",
			},
			expectedHop1: "dex-btc-hbd-123",
			expectedHop2: "dex-hbd-hive-456",
			shouldRoute: true,
		},
		{
			name:   "cannot route without intermediate pool",
			assetIn: "BTC",
			assetOut: "ETH",
			pools: map[string]string{
				"pool/BTC/HBD":  "dex-btc-hbd-123",
				"pool/HBD/HIVE": "dex-hbd-hive-456",
			},
			shouldRoute: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Check if direct pool exists
			asset0, asset1 := tt.assetIn, tt.assetOut
			if asset0 > asset1 {
				asset0, asset1 = asset1, asset0
			}
			directKey := "pool/" + asset0 + "/" + asset1
			_, hasDirect := tt.pools[directKey]

			if hasDirect {
				// Direct route exists
				return
			}

			// Check two-hop via HBD
			if tt.assetIn == "HBD" || tt.assetOut == "HBD" {
				// Already swapping to/from HBD, should use direct
				return
			}

			// Find first hop: AssetIn -> HBD
			hop1Key0, hop1Key1 := tt.assetIn, "HBD"
			if hop1Key0 > hop1Key1 {
				hop1Key0, hop1Key1 = hop1Key1, hop1Key0
			}
			hop1Key := "pool/" + hop1Key0 + "/" + hop1Key1
			hop1Contract, hasHop1 := tt.pools[hop1Key]

			// Find second hop: HBD -> AssetOut
			hop2Key0, hop2Key1 := "HBD", tt.assetOut
			if hop2Key0 > hop2Key1 {
				hop2Key0, hop2Key1 = hop2Key1, hop2Key0
			}
			hop2Key := "pool/" + hop2Key0 + "/" + hop2Key1
			hop2Contract, hasHop2 := tt.pools[hop2Key]

			canRoute := hasHop1 && hasHop2

			if canRoute != tt.shouldRoute {
				t.Errorf("expected canRoute=%v, got canRoute=%v", tt.shouldRoute, canRoute)
			}

			if canRoute {
				if hop1Contract != tt.expectedHop1 {
					t.Errorf("expected hop1 %s, got %s", tt.expectedHop1, hop1Contract)
				}
				if hop2Contract != tt.expectedHop2 {
					t.Errorf("expected hop2 %s, got %s", tt.expectedHop2, hop2Contract)
				}
			}
		})
	}
}
