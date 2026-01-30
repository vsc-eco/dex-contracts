package schemas

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFromJSON(t *testing.T) {
	tests := []struct {
		name        string
		jsonData    string
		expectError bool
		expected    *SwapInstruction
	}{
		{
			name: "valid swap instruction",
			jsonData: `{
				"type": "swap",
				"version": "1.0.0",
				"asset_in": "BTC",
				"asset_out": "HBD",
				"recipient": "alice"
			}`,
			expectError: false,
			expected: &SwapInstruction{
				InstructionType: "swap",
				SchemaVersion:   "1.0.0",
				AssetIn:         "BTC",
				AssetOut:        "HBD",
				Recipient:       "alice",
			},
		},
		{
			name: "instruction with optional fields",
			jsonData: `{
				"type": "swap",
				"version": "1.0.0",
				"asset_in": "BTC",
				"asset_out": "HBD_SAVINGS",
				"recipient": "alice",
				"slippage_bps": 200,
				"min_amount_out": 50000,
				"beneficiary": "referrer",
				"ref_bps": 500,
				"return_address": {"chain": "ETH", "address": "0x123..."},
				"metadata": {"notes": "test"}
			}`,
			expectError: false,
			expected: &SwapInstruction{
				InstructionType: "swap",
				SchemaVersion:   "1.0.0",
				AssetIn:         "BTC",
				AssetOut:        "HBD_SAVINGS",
				Recipient:       "alice",
				SlippageBps:     intPtr(200),
				MinAmountOut:    int64Ptr(50000),
				Beneficiary:     stringPtr("referrer"),
				RefBps:          intPtr(500),
				ReturnAddr:      &ReturnAddress{Chain: "ETH", Address: "0x123..."},
				Metadata:        map[string]interface{}{"notes": "test"},
			},
		},
		{
			name: "missing required field",
			jsonData: `{
				"type": "swap",
				"asset_in": "BTC",
				"asset_out": "HBD",
				"recipient": "alice"
			}`,
			expectError: true,
		},
		{
			name:     "invalid JSON",
			jsonData: `{invalid json}`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseFromJSON([]byte(tt.jsonData))

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tt.expected.InstructionType, result.InstructionType)
			assert.Equal(t, tt.expected.SchemaVersion, result.SchemaVersion)
			assert.Equal(t, tt.expected.AssetIn, result.AssetIn)
			assert.Equal(t, tt.expected.AssetOut, result.AssetOut)
			assert.Equal(t, tt.expected.Recipient, result.Recipient)
			assert.Equal(t, tt.expected.SlippageBps, result.SlippageBps)
			assert.Equal(t, tt.expected.MinAmountOut, result.MinAmountOut)
			assert.Equal(t, tt.expected.Beneficiary, result.Beneficiary)
			assert.Equal(t, tt.expected.RefBps, result.RefBps)
			assert.Equal(t, tt.expected.ReturnAddr, result.ReturnAddr)
			assert.Equal(t, tt.expected.Metadata, result.Metadata)
		})
	}
}

func TestParseFromQueryParams(t *testing.T) {
	tests := []struct {
		name        string
		query       string
		expectError bool
		expected    *SwapInstruction
	}{
		{
			name:        "valid query params",
			query:       "type=swap&version=1.0.0&asset_in=BTC&asset_out=HBD&recipient=alice",
			expectError: false,
			expected: &SwapInstruction{
				InstructionType: "swap",
				SchemaVersion:   "1.0.0",
				AssetIn:         "BTC",
				AssetOut:        "HBD",
				Recipient:       "alice",
			},
		},
		{
			name:  "query with optional fields",
			query: "type=swap&version=1.0.0&asset_in=BTC&asset_out=HBD&recipient=alice&slippage_bps=200&min_amount_out=50000&beneficiary=referrer&ref_bps=500&return_address.chain=ETH&return_address.address=0x123",
			expectError: false,
			expected: &SwapInstruction{
				InstructionType: "swap",
				SchemaVersion:   "1.0.0",
				AssetIn:         "BTC",
				AssetOut:        "HBD",
				Recipient:       "alice",
				SlippageBps:     intPtr(200),
				MinAmountOut:    int64Ptr(50000),
				Beneficiary:     stringPtr("referrer"),
				RefBps:          intPtr(500),
				ReturnAddr:      &ReturnAddress{Chain: "ETH", Address: "0x123"},
			},
		},
		{
			name:        "missing required field",
			query:       "type=swap&asset_in=BTC&asset_out=HBD&recipient=alice",
			expectError: true,
		},
		{
			name:        "invalid query format",
			query:       "invalid%query%format",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseFromQueryParams(tt.query)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tt.expected.InstructionType, result.InstructionType)
			assert.Equal(t, tt.expected.SchemaVersion, result.SchemaVersion)
			assert.Equal(t, tt.expected.AssetIn, result.AssetIn)
			assert.Equal(t, tt.expected.AssetOut, result.AssetOut)
			assert.Equal(t, tt.expected.Recipient, result.Recipient)
		})
	}
}

func TestParseFromMemo(t *testing.T) {
	tests := []struct {
		name        string
		memo        string
		expectError bool
		expected    *SwapInstruction
	}{
		{
			name:        "JSON format",
			memo:        `{"type":"swap","version":"1.0.0","asset_in":"BTC","asset_out":"HBD","recipient":"alice"}`,
			expectError: false,
			expected: &SwapInstruction{
				InstructionType: "swap",
				SchemaVersion:   "1.0.0",
				AssetIn:         "BTC",
				AssetOut:        "HBD",
				Recipient:       "alice",
			},
		},
		{
			name:        "URL query format",
			memo:        "type=swap&version=1.0.0&asset_in=BTC&asset_out=HBD&recipient=alice",
			expectError: false,
			expected: &SwapInstruction{
				InstructionType: "swap",
				SchemaVersion:   "1.0.0",
				AssetIn:         "BTC",
				AssetOut:        "HBD",
				Recipient:       "alice",
			},
		},
		{
			name:        "whitespace trimmed",
			memo:        "  type=swap&version=1.0.0&asset_in=BTC&asset_out=HBD&recipient=alice  ",
			expectError: false,
			expected: &SwapInstruction{
				InstructionType: "swap",
				SchemaVersion:   "1.0.0",
				AssetIn:         "BTC",
				AssetOut:        "HBD",
				Recipient:       "alice",
			},
		},
		{
			name:        "URL format with return_address (instruction-schema.md)",
			memo:        "type=swap&version=1.0.0&asset_in=BTC&asset_out=HBD&recipient=alice&return_address.chain=ETH&return_address.address=0x123...",
			expectError: false,
			expected: &SwapInstruction{
				InstructionType: "swap",
				SchemaVersion:   "1.0.0",
				AssetIn:         "BTC",
				AssetOut:        "HBD",
				Recipient:       "alice",
				ReturnAddr:      &ReturnAddress{Chain: "ETH", Address: "0x123..."},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseFromMemo(tt.memo)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tt.expected.InstructionType, result.InstructionType)
			assert.Equal(t, tt.expected.SchemaVersion, result.SchemaVersion)
			assert.Equal(t, tt.expected.AssetIn, result.AssetIn)
			assert.Equal(t, tt.expected.AssetOut, result.AssetOut)
			assert.Equal(t, tt.expected.Recipient, result.Recipient)
			if tt.expected.ReturnAddr != nil {
				require.NotNil(t, result.ReturnAddr)
				assert.Equal(t, tt.expected.ReturnAddr.Chain, result.ReturnAddr.Chain)
				assert.Equal(t, tt.expected.ReturnAddr.Address, result.ReturnAddr.Address)
			}
		})
	}
}

// Helper functions for creating pointers
func intPtr(i int) *int {
	return &i
}

func int64Ptr(i int64) *int64 {
	return &i
}

func stringPtr(s string) *string {
	return &s
}

func TestParseFromCustomJSON(t *testing.T) {
	jsonData := `{"type":"swap","version":"1.0.0","asset_in":"BTC","asset_out":"HBD","recipient":"alice"}`
	result, err := ParseFromCustomJSON([]byte(jsonData))
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "swap", result.InstructionType)
	assert.Equal(t, "1.0.0", result.SchemaVersion)
	assert.Equal(t, "BTC", result.AssetIn)
	assert.Equal(t, "HBD", result.AssetOut)
	assert.Equal(t, "alice", result.Recipient)
}

func TestValidateInstructionStruct(t *testing.T) {
	instruction := &SwapInstruction{
		InstructionType: "swap",
		SchemaVersion:   "1.0.0",
		AssetIn:         "BTC",
		AssetOut:        "HBD",
		Recipient:       "alice",
	}
	err := ValidateInstructionStruct(instruction)
	assert.NoError(t, err)
}

func TestSwapInstruction_Validate(t *testing.T) {
	tests := []struct {
		name        string
		instruction SwapInstruction
		expectError bool
	}{
		{"valid", SwapInstruction{InstructionType: "swap", SchemaVersion: "1.0.0", AssetIn: "BTC", AssetOut: "HBD", Recipient: "alice"}, false},
		{"missing type", SwapInstruction{SchemaVersion: "1.0.0", AssetIn: "BTC", AssetOut: "HBD", Recipient: "alice"}, true},
		{"missing version", SwapInstruction{InstructionType: "swap", AssetIn: "BTC", AssetOut: "HBD", Recipient: "alice"}, true},
		{"missing asset_in", SwapInstruction{InstructionType: "swap", SchemaVersion: "1.0.0", AssetOut: "HBD", Recipient: "alice"}, true},
		{"missing asset_out", SwapInstruction{InstructionType: "swap", SchemaVersion: "1.0.0", AssetIn: "BTC", Recipient: "alice"}, true},
		{"missing recipient", SwapInstruction{InstructionType: "swap", SchemaVersion: "1.0.0", AssetIn: "BTC", AssetOut: "HBD"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.instruction.Validate()
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidationError_Error(t *testing.T) {
	err := &ValidationError{Field: "type", Message: "type is required"}
	assert.Equal(t, "type is required", err.Error())
}

func TestSwapInstruction_ToJSON(t *testing.T) {
	instruction := SwapInstruction{
		InstructionType: "swap",
		SchemaVersion:   "1.0.0",
		AssetIn:         "BTC",
		AssetOut:        "HBD",
		Recipient:       "alice",
	}
	data, err := instruction.ToJSON()
	require.NoError(t, err)
	assert.NotEmpty(t, data)
	var parsed map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &parsed))
	assert.Equal(t, "swap", parsed["type"])
	assert.Equal(t, "alice", parsed["recipient"])
}

func TestValidateInstruction(t *testing.T) {
	tests := []struct {
		name        string
		jsonData    string
		expectError bool
	}{
		{
			name: "valid instruction",
			jsonData: `{
				"type": "swap",
				"version": "1.0.0",
				"asset_in": "BTC",
				"asset_out": "HBD",
				"recipient": "alice"
			}`,
			expectError: false,
		},
		{
			name: "missing required field",
			jsonData: `{
				"type": "swap",
				"asset_in": "BTC",
				"asset_out": "HBD",
				"recipient": "alice"
			}`,
			expectError: true,
		},
		{
			name: "invalid slippage_bps",
			jsonData: `{
				"type": "swap",
				"version": "1.0.0",
				"asset_in": "BTC",
				"asset_out": "HBD",
				"recipient": "alice",
				"slippage_bps": 20000
			}`,
			expectError: true,
		},
		{
			name: "invalid type",
			jsonData: `{
				"type": "invalid",
				"version": "1.0.0",
				"asset_in": "BTC",
				"asset_out": "HBD",
				"recipient": "alice"
			}`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateInstruction([]byte(tt.jsonData))

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
