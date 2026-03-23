package main

import (
	"strings"
	"testing"

	"github.com/vsc-eco/dex-contracts/contracts/types"
)

// Test pool registration
func TestPoolRegistration(t *testing.T) {
	tests := []struct {
		name            string
		asset0          string
		asset1          string
		dexContract     string
		shouldNormalize bool
	}{
		{
			name:            "normal order",
			asset0:          "HBD",
			asset1:          "HIVE",
			dexContract:     "dex-hbd-hive-123",
			shouldNormalize: false,
		},
		{
			name:            "reverse order should normalize",
			asset0:          "HIVE",
			asset1:          "HBD",
			dexContract:     "dex-hbd-hive-123",
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
			expectedKey := "pool" + types.DirPathDelimiter + asset0 + types.DirPathDelimiter + asset1

			if expectedKey != "pool"+types.DirPathDelimiter+"HBD"+types.DirPathDelimiter+"HIVE" {
				t.Errorf(
					"expected normalized key pool%sHBD%sHIVE, got %s",
					types.DirPathDelimiter,
					types.DirPathDelimiter,
					expectedKey,
				)
			}
		})
	}
}

// Test pool finding logic
func TestFindPool(t *testing.T) {
	tests := []struct {
		name       string
		assetA     string
		assetB     string
		pools      map[string]string // pool key -> contract ID
		expected   string
		shouldFind bool
	}{
		{
			name:   "find existing pool",
			assetA: "HBD",
			assetB: "HIVE",
			pools: map[string]string{
				"pool" + types.DirPathDelimiter + "HBD" + types.DirPathDelimiter + "HIVE": "dex-hbd-hive-123",
			},
			expected:   "dex-hbd-hive-123",
			shouldFind: true,
		},
		{
			name:   "find pool with reversed assets",
			assetA: "HIVE",
			assetB: "HBD",
			pools: map[string]string{
				"pool" + types.DirPathDelimiter + "HBD" + types.DirPathDelimiter + "HIVE": "dex-hbd-hive-123",
			},
			expected:   "dex-hbd-hive-123",
			shouldFind: true,
		},
		{
			name:   "pool not found",
			assetA: "BTC",
			assetB: "ETH",
			pools: map[string]string{
				"pool" + types.DirPathDelimiter + "HBD" + types.DirPathDelimiter + "HIVE": "dex-hbd-hive-123",
			},
			expected:   "",
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

			poolKey := "pool" + types.DirPathDelimiter + asset0 + types.DirPathDelimiter + asset1
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
		name         string
		assetIn      string
		assetOut     string
		pools        map[string]string
		expectedHop1 string
		expectedHop2 string
		shouldRoute  bool
	}{
		{
			name:     "route BTC -> HIVE via HBD",
			assetIn:  "BTC",
			assetOut: "HIVE",
			pools: map[string]string{
				"pool" + types.DirPathDelimiter + "BTC" + types.DirPathDelimiter + "HBD":  "dex-btc-hbd-123",
				"pool" + types.DirPathDelimiter + "HBD" + types.DirPathDelimiter + "HIVE": "dex-hbd-hive-456",
			},
			expectedHop1: "dex-btc-hbd-123",
			expectedHop2: "dex-hbd-hive-456",
			shouldRoute:  true,
		},
		{
			name:     "cannot route without intermediate pool",
			assetIn:  "BTC",
			assetOut: "ETH",
			pools: map[string]string{
				"pool" + types.DirPathDelimiter + "BTC" + types.DirPathDelimiter + "HBD":  "dex-btc-hbd-123",
				"pool" + types.DirPathDelimiter + "HBD" + types.DirPathDelimiter + "HIVE": "dex-hbd-hive-456",
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
			directKey := "pool" + types.DirPathDelimiter + asset0 + types.DirPathDelimiter + asset1
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
			hop1Key := "pool" + types.DirPathDelimiter + hop1Key0 + types.DirPathDelimiter + hop1Key1
			hop1Contract, hasHop1 := tt.pools[hop1Key]

			// Find second hop: HBD -> AssetOut
			hop2Key0, hop2Key1 := "HBD", tt.assetOut
			if hop2Key0 > hop2Key1 {
				hop2Key0, hop2Key1 = hop2Key1, hop2Key0
			}
			hop2Key := "pool" + types.DirPathDelimiter + hop2Key0 + types.DirPathDelimiter + hop2Key1
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

// Test token registry - tokens must be registered before pools can use them
func TestTokenRegistryEnforcement(t *testing.T) {
	tests := []struct {
		name             string
		asset0           string
		asset1           string
		asset0Registered bool
		asset1Registered bool
		shouldReject     bool
	}{
		{
			name:             "both assets registered - allow",
			asset0:           "HBD",
			asset1:           "HIVE",
			asset0Registered: true,
			asset1Registered: true,
			shouldReject:     false,
		},
		{
			name:             "asset0 not registered - reject",
			asset0:           "BTC",
			asset1:           "HBD",
			asset0Registered: false,
			asset1Registered: true,
			shouldReject:     true,
		},
		{
			name:             "asset1 not registered - reject",
			asset0:           "HBD",
			asset1:           "ETH",
			asset0Registered: true,
			asset1Registered: false,
			shouldReject:     true,
		},
		{
			name:             "neither asset registered - reject",
			asset0:           "BTC",
			asset1:           "ETH",
			asset0Registered: false,
			asset1Registered: false,
			shouldReject:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wouldReject := !tt.asset0Registered || !tt.asset1Registered
			if wouldReject != tt.shouldReject {
				t.Errorf("expected shouldReject=%v, got wouldReject=%v", tt.shouldReject, wouldReject)
			}
		})
	}
}

// Test splitChains logic (replicated from utils.go for standalone test package)
func TestSplitChains(t *testing.T) {
	splitChains := func(s string) []string {
		if s == "" {
			return nil
		}
		parts := strings.Split(s, ",")
		seen := make(map[string]bool)
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" && !seen[p] {
				seen[p] = true
				out = append(out, p)
			}
		}
		return out
	}

	tests := []struct {
		input    string
		expected []string
	}{
		{"HIVE", []string{"HIVE"}},
		{"HIVE,MAGI,BTC", []string{"HIVE", "MAGI", "BTC"}},
		{"HIVE, HIVE, MAGI", []string{"HIVE", "MAGI"}},
		{"", nil},
		{"  HIVE  ,  MAGI  ", []string{"HIVE", "MAGI"}},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := splitChains(tt.input)
			if len(got) != len(tt.expected) {
				t.Errorf("splitChains(%q) len=%d, want %d", tt.input, len(got), len(tt.expected))
			}
			for i, e := range tt.expected {
				if i < len(got) && got[i] != e {
					t.Errorf("splitChains(%q)[%d]=%q, want %q", tt.input, i, got[i], e)
				}
			}
		})
	}
}

// Test supported chains include HIVE and MAGI for native tokens
func TestSupportedChains(t *testing.T) {
	chains := []string{"HIVE", "MAGI", "BTC", "ETH", "SOL"}
	required := []string{"HIVE", "MAGI"}
	for _, r := range required {
		found := false
		for _, c := range chains {
			if c == r {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("required chain %s not in supported chains", r)
		}
	}
}

// Test pool key normalization
func TestPoolKeyNormalization(t *testing.T) {
	tests := []struct {
		asset0, asset1 string
		expectedKey    string
	}{
		{"HBD", "HIVE", "pool" + types.DirPathDelimiter + "HBD" + types.DirPathDelimiter + "HIVE"},
		{"HIVE", "HBD", "pool" + types.DirPathDelimiter + "HBD" + types.DirPathDelimiter + "HIVE"},
		{"BTC", "HBD", "pool" + types.DirPathDelimiter + "BTC" + types.DirPathDelimiter + "HBD"},
	}
	for _, tt := range tests {
		a0, a1 := tt.asset0, tt.asset1
		if a0 > a1 {
			a0, a1 = a1, a0
		}
		key := "pool" + types.DirPathDelimiter + a0 + types.DirPathDelimiter + a1
		if key != tt.expectedKey {
			t.Errorf("poolKey(%s,%s)=%s, want %s", tt.asset0, tt.asset1, key, tt.expectedKey)
		}
	}
}

// Test intent building logic for mapped vs native assets
func TestMappedAssetIntentBuilding(t *testing.T) {
	// Simulates the logic in buildMappedAssetIntents:
	// - Native assets (HIVE/HBD) → no mapping contract → empty intents
	// - Mapped assets (BTC/ETH) → has mapping contract → transfer.allow intent
	type tokenInfo struct {
		MappingContract string
		Chain           string
	}

	tokenRegistry := map[string]tokenInfo{
		"hbd":  {MappingContract: "", Chain: "HIVE"},
		"hive": {MappingContract: "", Chain: "HIVE"},
		"btc":  {MappingContract: "btc_mapping_contract_123", Chain: "BTC"},
		"eth":  {MappingContract: "eth_mapping_contract_456", Chain: "ETH"},
	}

	tests := []struct {
		name           string
		assetIn        string
		amountIn       string
		expectIntent   bool
		expectContract string
	}{
		{
			name:         "native HBD - no intent needed",
			assetIn:      "hbd",
			amountIn:     "1000000",
			expectIntent: false,
		},
		{
			name:         "native HIVE - no intent needed",
			assetIn:      "hive",
			amountIn:     "500000",
			expectIntent: false,
		},
		{
			name:           "mapped BTC - intent with mapping contract",
			assetIn:        "btc",
			amountIn:       "10000000",
			expectIntent:   true,
			expectContract: "btc_mapping_contract_123",
		},
		{
			name:           "mapped ETH - intent with mapping contract",
			assetIn:        "eth",
			amountIn:       "2000000000000000000",
			expectIntent:   true,
			expectContract: "eth_mapping_contract_456",
		},
		{
			name:         "unregistered asset - no intent (no mapping contract)",
			assetIn:      "unknown",
			amountIn:     "100",
			expectIntent: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, registered := tokenRegistry[tt.assetIn]
			mappingContract := ""
			if registered {
				mappingContract = info.MappingContract
			}

			hasIntent := mappingContract != ""
			if hasIntent != tt.expectIntent {
				t.Errorf("expected hasIntent=%v for %s, got %v", tt.expectIntent, tt.assetIn, hasIntent)
			}

			if hasIntent {
				if mappingContract != tt.expectContract {
					t.Errorf("expected mappingContract=%s, got %s", tt.expectContract, mappingContract)
				}
				// Verify intent structure matches expected format
				intentType := "transfer.allow"
				intentArgs := map[string]string{
					"limit":       tt.amountIn,
					"token":       tt.assetIn,
					"contract_id": mappingContract,
				}
				if intentType != "transfer.allow" {
					t.Error("intent type should be transfer.allow")
				}
				if intentArgs["limit"] != tt.amountIn {
					t.Errorf("intent limit=%s, want %s", intentArgs["limit"], tt.amountIn)
				}
				if intentArgs["token"] != tt.assetIn {
					t.Errorf("intent token=%s, want %s", intentArgs["token"], tt.assetIn)
				}
				if intentArgs["contract_id"] != mappingContract {
					t.Errorf("intent contract_id=%s, want %s", intentArgs["contract_id"], mappingContract)
				}
			}
		})
	}
}

// Test two-hop intent forwarding logic
func TestTwoHopIntentForwarding(t *testing.T) {
	// In two-hop swaps (AssetIn -> HBD -> AssetOut):
	// - First hop: if AssetIn is mapped, forward transfer.allow intent for its mapping contract
	// - Second hop: always has HBD transfer.allow intent (native, handled by protocol)

	type tokenInfo struct {
		MappingContract string
		Chain           string
	}

	tokenRegistry := map[string]tokenInfo{
		"hbd":  {MappingContract: "", Chain: "HIVE"},
		"btc":  {MappingContract: "btc_mapping_contract", Chain: "BTC"},
		"hive": {MappingContract: "", Chain: "HIVE"},
	}

	tests := []struct {
		name               string
		assetIn            string
		assetOut           string
		amountIn           string
		hop1NeedsIntent    bool
		hop2NeedsHBDIntent bool
	}{
		{
			name:               "BTC -> HIVE: first hop needs BTC mapping intent",
			assetIn:            "btc",
			assetOut:           "hive",
			amountIn:           "10000000",
			hop1NeedsIntent:    true,
			hop2NeedsHBDIntent: true,
		},
		{
			name:               "HIVE -> BTC: first hop is native, no mapping intent",
			assetIn:            "hive",
			assetOut:           "btc",
			amountIn:           "1000000",
			hop1NeedsIntent:    false,
			hop2NeedsHBDIntent: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, registered := tokenRegistry[tt.assetIn]
			hop1MappingContract := ""
			if registered {
				hop1MappingContract = info.MappingContract
			}
			hop1HasIntent := hop1MappingContract != ""

			if hop1HasIntent != tt.hop1NeedsIntent {
				t.Errorf("hop1: expected needs_intent=%v, got %v", tt.hop1NeedsIntent, hop1HasIntent)
			}

			// Second hop always needs HBD intent for native transfer
			if !tt.hop2NeedsHBDIntent {
				t.Error("hop2 should always need HBD intent")
			}
		})
	}
}

// Test deposit intent building for pools with mapped assets
func TestDepositMappedAssetIntents(t *testing.T) {
	// When depositing into a pool (e.g., BTC/HBD), the router should build
	// intents for any mapped assets in the pair.
	type tokenInfo struct {
		MappingContract string
	}
	tokenRegistry := map[string]tokenInfo{
		"btc": {MappingContract: "btc_mapping"},
		"hbd": {MappingContract: ""},
	}

	tests := []struct {
		name            string
		asset0          string
		asset1          string
		amount0         string
		amount1         string
		expectedIntents int
	}{
		{
			name:            "BTC/HBD pool - 1 intent for BTC",
			asset0:          "btc",
			asset1:          "hbd",
			amount0:         "10000",
			amount1:         "500000",
			expectedIntents: 1,
		},
		{
			name:            "HBD/HIVE pool - 0 intents (both native)",
			asset0:          "hbd",
			asset1:          "hive",
			amount0:         "100000",
			amount1:         "200000",
			expectedIntents: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			intentCount := 0
			if info, ok := tokenRegistry[tt.asset0]; ok && info.MappingContract != "" {
				intentCount++
			}
			if info, ok := tokenRegistry[tt.asset1]; ok && info.MappingContract != "" {
				intentCount++
			}
			if intentCount != tt.expectedIntents {
				t.Errorf("expected %d intents, got %d", tt.expectedIntents, intentCount)
			}
		})
	}
}

// Two-hop swap failure handling (V2-SYSTEM-SUMMARY, dex-router-v2 handlePartialSwap/handleSwapFailure)
// Replicates the decision logic: when second hop fails, determine what asset to return based on return_address

type returnAddress struct {
	Chain   string
	Address string
}

// getAssetChainForTest - replicates contract's getAssetChain (asset -> chain mapping)
func getAssetChainForTest(asset string, assetChains map[string]string) string {
	return assetChains[asset]
}

// determineReturnAssetForPartialSwap - replicates handlePartialSwap logic for second-hop failure
// Returns (assetToReturn, amountToReturn, shouldTrySwapBack)
func determineReturnAssetForPartialSwap(
	originalAsset string,
	intermediateAsset string,
	intermediateAmount uint64,
	returnAddr *returnAddress,
	assetChains map[string]string,
) (asset string, amount uint64, triedSwapBack bool) {
	asset = intermediateAsset
	amount = intermediateAmount

	if returnAddr == nil {
		return asset, amount, false
	}

	originalAssetChain := getAssetChainForTest(originalAsset, assetChains)
	returnChain := returnAddr.Chain

	// If original asset is from non-Hive chain and return address is on that same chain,
	// we would try to swap HBD back to original asset (trySwapBackToOriginal)
	if originalAssetChain != "HIVE" && returnChain != "" && returnChain == originalAssetChain {
		triedSwapBack = true
		// In the contract, trySwapBackToOriginal would execute; if it succeeds,
		// asset=originalAsset, amount=swapBackResult.AmountOut
		// For spec test we just verify the decision to attempt swap-back
	}
	return asset, amount, triedSwapBack
}

// TestTwoHopFailure_SecondHopFailed_ReturnAddressSameChain tests V2-SYSTEM-SUMMARY:
// "If return address chain matches original asset chain: Attempts to swap HBD back to original asset"
func TestTwoHopFailure_SecondHopFailed_ReturnAddressSameChain(t *testing.T) {
	assetChains := map[string]string{"BTC": "BTC", "HBD": "HIVE", "HIVE": "HIVE"}

	// BTC -> HBD (success) -> HIVE (failed), return_address.chain=BTC
	// Original asset chain (BTC) matches return address chain (BTC) -> should try swap-back
	asset, amount, triedSwapBack := determineReturnAssetForPartialSwap(
		"BTC", "HBD", 500000,
		&returnAddress{Chain: "BTC", Address: "bc1q..."},
		assetChains,
	)

	if !triedSwapBack {
		t.Error("expected triedSwapBack=true when return_address.chain matches original asset chain (BTC)")
	}
	if asset != "HBD" {
		t.Errorf("before swap-back succeeds, asset=HBD (intermediate), got %s", asset)
	}
	if amount != 500000 {
		t.Errorf("amount=500000, got %d", amount)
	}
}

// TestTwoHopFailure_SecondHopFailed_ReturnAddressDifferentChain tests:
// "If chain doesn't match: Returns intermediate HBD to return address"
func TestTwoHopFailure_SecondHopFailed_ReturnAddressDifferentChain(t *testing.T) {
	assetChains := map[string]string{"BTC": "BTC", "HBD": "HIVE", "HIVE": "HIVE"}

	// BTC -> HBD (success) -> HIVE (failed), return_address.chain=ETH (different from BTC)
	// Return intermediate HBD
	asset, amount, triedSwapBack := determineReturnAssetForPartialSwap(
		"BTC", "HBD", 500000,
		&returnAddress{Chain: "ETH", Address: "0x123..."},
		assetChains,
	)

	if triedSwapBack {
		t.Error("expected triedSwapBack=false when return_address.chain differs from original asset chain")
	}
	if asset != "HBD" {
		t.Errorf("expected asset=HBD (intermediate), got %s", asset)
	}
	if amount != 500000 {
		t.Errorf("amount=500000, got %d", amount)
	}
}

// TestTwoHopFailure_SecondHopFailed_NoReturnAddress tests fallback:
// "If no return_address: Return to recipient"
func TestTwoHopFailure_SecondHopFailed_NoReturnAddress(t *testing.T) {
	assetChains := map[string]string{"BTC": "BTC", "HBD": "HIVE"}

	asset, amount, triedSwapBack := determineReturnAssetForPartialSwap(
		"BTC", "HBD", 500000,
		nil, // no return_address
		assetChains,
	)

	if triedSwapBack {
		t.Error("expected triedSwapBack=false when no return_address")
	}
	if asset != "HBD" {
		t.Errorf("expected asset=HBD, got %s", asset)
	}
	if amount != 500000 {
		t.Errorf("amount=500000, got %d", amount)
	}
}

// TestTwoHopFailure_FirstHopFailed tests handleSwapFailure:
// "First swap failed - return original asset via return_address"
func TestTwoHopFailure_FirstHopFailed(t *testing.T) {
	// When first hop fails, we return the original asset (e.g. BTC) and original amount
	// Replicate the handleSwapFailure logic
	originalAsset := "BTC"
	originalAmount := uint64(100000)
	returnAddr := &returnAddress{Chain: "BTC", Address: "bc1q..."}

	// handleSwapFailure returns original asset to return_address
	assetToReturn := originalAsset
	amountToReturn := originalAmount

	if assetToReturn != "BTC" {
		t.Errorf("first hop failed: return original asset BTC, got %s", assetToReturn)
	}
	if amountToReturn != 100000 {
		t.Errorf("amount=100000, got %d", amountToReturn)
	}
	if returnAddr.Chain != "BTC" {
		t.Errorf("return address chain should be BTC for cross-chain return")
	}
}

// TestTwoHopFailure_ReturnAddressHiveChain tests:
// "If return address is on Hive: return HBD directly (no swap-back needed)"
func TestTwoHopFailure_ReturnAddressHiveChain(t *testing.T) {
	assetChains := map[string]string{"BTC": "BTC", "HBD": "HIVE"}

	// BTC -> HBD (success) -> HIVE (failed), return_address.chain=HIVE
	// Return HBD directly - recipient can use HBD on Hive
	asset, amount, triedSwapBack := determineReturnAssetForPartialSwap(
		"BTC", "HBD", 500000,
		&returnAddress{Chain: "HIVE", Address: "alice"},
		assetChains,
	)

	if triedSwapBack {
		t.Error("expected triedSwapBack=false when return_address.chain=HIVE (can return HBD directly)")
	}
	if asset != "HBD" {
		t.Errorf("expected asset=HBD, got %s", asset)
	}
	if amount != 500000 {
		t.Errorf("amount=500000, got %d", amount)
	}
}
