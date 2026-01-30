package router

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockDEXExecutorForServer implements DEXExecutor for server tests
type mockDEXExecutorForServer struct {
	executedOperations []string
}

func (m *mockDEXExecutorForServer) ExecuteDexOperation(ctx context.Context, operationType string, payload string) error {
	m.executedOperations = append(m.executedOperations, operationType+":"+payload)
	return nil
}

func (m *mockDEXExecutorForServer) ExecuteDexOperationWithIntents(ctx context.Context, operationType string, payload string, intents []Intent) error {
	m.executedOperations = append(m.executedOperations, operationType+":"+payload)
	return nil
}

func (m *mockDEXExecutorForServer) ExecuteDexSwap(ctx context.Context, amountOut int64, route []string, fee int64) error {
	return nil
}

// TestServer_handleExecuteInstruction_Success tests POST /api/v1/instruction (instruction-schema.md)
func TestServer_handleExecuteInstruction_Success(t *testing.T) {
	mockExecutor := &mockDEXExecutorForServer{}
	config := VSCConfig{DexRouterContract: "dex-router-contract"}
	svc := NewService(config, mockExecutor)
	server := NewServer(svc, "8080")

	// Documented format: instruction as JSON object
	reqBody := map[string]interface{}{
		"instruction": map[string]interface{}{
			"type":      "swap",
			"version":   "1.0.0",
			"asset_in":  "BTC",
			"asset_out": "HBD",
			"recipient": "alice",
		},
		"amountIn": 100000,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/v1/instruction", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.handleExecuteInstruction(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var result SwapResult
	require.NoError(t, json.NewDecoder(w.Body).Decode(&result))
	assert.True(t, result.Success)
	assert.Len(t, mockExecutor.executedOperations, 1)
	assert.True(t, strings.Contains(mockExecutor.executedOperations[0], "execute:"))
	assert.True(t, strings.Contains(mockExecutor.executedOperations[0], "BTC"))
	assert.True(t, strings.Contains(mockExecutor.executedOperations[0], "HBD"))
}

// TestServer_handleExecuteInstruction_WithOptionalFields tests instruction with slippage, return_address
func TestServer_handleExecuteInstruction_WithOptionalFields(t *testing.T) {
	mockExecutor := &mockDEXExecutorForServer{}
	config := VSCConfig{DexRouterContract: "dex-router-contract"}
	svc := NewService(config, mockExecutor)
	server := NewServer(svc, "8080")

	reqBody := map[string]interface{}{
		"instruction": map[string]interface{}{
			"type":           "swap",
			"version":        "1.0.0",
			"asset_in":       "BTC",
			"asset_out":      "HBD",
			"recipient":      "alice",
			"slippage_bps":   200,
			"min_amount_out": 95000,
			"return_address": map[string]interface{}{
				"chain":   "BTC",
				"address": "bc1qxy2kgdygjrsqtzq2n0yrf249p83kkfjhx0wlh",
			},
		},
		"amountIn": 100000,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/v1/instruction", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.handleExecuteInstruction(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var result SwapResult
	require.NoError(t, json.NewDecoder(w.Body).Decode(&result))
	assert.True(t, result.Success)
}

// TestServer_handleExecuteInstruction_InvalidInstruction tests validation failure
func TestServer_handleExecuteInstruction_InvalidInstruction(t *testing.T) {
	mockExecutor := &mockDEXExecutorForServer{}
	config := VSCConfig{DexRouterContract: "dex-router-contract"}
	svc := NewService(config, mockExecutor)
	server := NewServer(svc, "8080")

	// Missing required field (version)
	reqBody := map[string]interface{}{
		"instruction": map[string]interface{}{
			"type":      "swap",
			"asset_in":  "BTC",
			"asset_out": "HBD",
			"recipient": "alice",
		},
		"amountIn": 100000,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/v1/instruction", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.handleExecuteInstruction(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Failed to process instruction")
}

// TestServer_handleExecuteInstruction_InvalidJSON tests malformed JSON
func TestServer_handleExecuteInstruction_InvalidJSON(t *testing.T) {
	mockExecutor := &mockDEXExecutorForServer{}
	config := VSCConfig{DexRouterContract: "dex-router-contract"}
	svc := NewService(config, mockExecutor)
	server := NewServer(svc, "8080")

	reqBody := `{"instruction": "not valid json", "amountIn": 100000}`
	req := httptest.NewRequest("POST", "/api/v1/instruction", bytes.NewReader([]byte(reqBody)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.handleExecuteInstruction(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestServer_handleExecuteInstruction_MissingInstruction tests empty instruction
func TestServer_handleExecuteInstruction_MissingInstruction(t *testing.T) {
	mockExecutor := &mockDEXExecutorForServer{}
	config := VSCConfig{DexRouterContract: "dex-router-contract"}
	svc := NewService(config, mockExecutor)
	server := NewServer(svc, "8080")

	// Empty string or null instruction
	reqBody := `{"instruction": "", "amountIn": 100000}`
	req := httptest.NewRequest("POST", "/api/v1/instruction", bytes.NewReader([]byte(reqBody)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.handleExecuteInstruction(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "instruction is required")
}

// TestServer_handleExecuteInstruction_InvalidAmountIn tests amountIn <= 0
func TestServer_handleExecuteInstruction_InvalidAmountIn(t *testing.T) {
	mockExecutor := &mockDEXExecutorForServer{}
	config := VSCConfig{DexRouterContract: "dex-router-contract"}
	svc := NewService(config, mockExecutor)
	server := NewServer(svc, "8080")

	reqBody := map[string]interface{}{
		"instruction": map[string]interface{}{
			"type":      "swap",
			"version":   "1.0.0",
			"asset_in":  "BTC",
			"asset_out": "HBD",
			"recipient": "alice",
		},
		"amountIn": 0,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/v1/instruction", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.handleExecuteInstruction(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "amountIn")
}

// TestServer_handleComputeRoute tests POST /api/v1/route
func TestServer_handleComputeRoute(t *testing.T) {
	mockExecutor := &mockDEXExecutorForServer{}
	config := VSCConfig{DexRouterContract: "dex-router-contract"}
	svc := NewService(config, mockExecutor)
	server := NewServer(svc, "8080")

	reqBody := `{"fromAsset":"BTC","toAsset":"HBD","amount":100000}`
	req := httptest.NewRequest("POST", "/api/v1/route", bytes.NewReader([]byte(reqBody)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.handleComputeRoute(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var result SwapResult
	require.NoError(t, json.NewDecoder(w.Body).Decode(&result))
	assert.True(t, result.Success)
}

// TestServer_handleHealth tests GET /health
func TestServer_handleHealth(t *testing.T) {
	mockExecutor := &mockDEXExecutorForServer{}
	config := VSCConfig{DexRouterContract: "dex-router-contract"}
	svc := NewService(config, mockExecutor)
	server := NewServer(svc, "8080")

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	server.handleHealth(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "healthy", resp["status"])
	assert.Equal(t, "dex-router", resp["service"])
}
