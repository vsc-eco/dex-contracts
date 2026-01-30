package indexer

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewServer(t *testing.T) {
	svc := NewService("http://localhost:4000", ":8081")
	server := NewServer(svc, "8081")

	assert.NotNil(t, server)
	assert.Equal(t, svc, server.indexer)
	assert.NotNil(t, server.http)
	assert.Equal(t, ":8081", server.http.Addr)
}

func TestServer_handleGetPools(t *testing.T) {
	svc := NewService("http://localhost:4000", ":8081")

	// Add test pools to the DEX reader
	dexReader := svc.readers[0].(*DexReadModel)
	dexReader.pools["pool-1"] = PoolInfo{
		ID:       "pool-1",
		Asset0:   "HBD",
		Asset1:   "HIVE",
		Reserve0: 1000000,
		Reserve1: 500000,
		Fee:      0.08,
	}
	dexReader.pools["pool-2"] = PoolInfo{
		ID:       "pool-2",
		Asset0:   "BTC",
		Asset1:   "HBD",
		Reserve0: 100000000,
		Reserve1: 20000000,
		Fee:      0.1,
	}

	server := NewServer(svc, "8081")

	// Create request
	req := httptest.NewRequest("GET", "/api/v1/pools", nil)
	w := httptest.NewRecorder()

	// Call handler
	server.handleGetPools(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var pools []PoolInfo
	err := json.NewDecoder(w.Body).Decode(&pools)
	require.NoError(t, err)
	assert.Len(t, pools, 2)

	// Verify pool data
	poolIDs := make(map[string]bool)
	for _, pool := range pools {
		poolIDs[pool.ID] = true
		assert.NotEmpty(t, pool.Asset0)
		assert.NotEmpty(t, pool.Asset1)
		assert.Greater(t, pool.Reserve0, uint64(0))
	}
	assert.True(t, poolIDs["pool-1"])
	assert.True(t, poolIDs["pool-2"])
}

func TestServer_handleGetPool_Existing(t *testing.T) {
	svc := NewService("http://localhost:4000", ":8081")

	// Add test pool to the default DEX reader
	dexReader := svc.readers[0].(*DexReadModel)
	testPool := PoolInfo{
		ID:       "test-pool-123",
		Asset0:   "HBD",
		Asset1:   "HIVE",
		Reserve0: 1000000,
		Reserve1: 500000,
		Fee:      0.08,
	}
	dexReader.pools["test-pool-123"] = testPool

	server := NewServer(svc, "8081")

	// Create request - need to set up mux vars for the test
	req := httptest.NewRequest("GET", "/api/v1/pools/test-pool-123", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "test-pool-123"})
	w := httptest.NewRecorder()

	// Call handler
	server.handleGetPool(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var pool PoolInfo
	err := json.NewDecoder(w.Body).Decode(&pool)
	require.NoError(t, err)
	assert.Equal(t, testPool, pool)
}

func TestServer_handleGetPool_NotFound(t *testing.T) {
	svc := NewService("http://localhost:4000", ":8081")
	server := NewServer(svc, "8081")

	// Create request for non-existing pool
	req := httptest.NewRequest("GET", "/api/v1/pools/nonexistent-pool", nil)
	w := httptest.NewRecorder()

	// Call handler
	server.handleGetPool(w, req)

	// Verify response
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "Pool not found")
}

func TestServer_handleHealth(t *testing.T) {
	svc := NewService("http://localhost:4000", ":8081")
	server := NewServer(svc, "8081")

	// Create request
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	// Call handler
	server.handleHealth(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response map[string]string
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	assert.Equal(t, "healthy", response["status"])
	assert.Equal(t, "dex-indexer", response["service"])
}

func TestServer_Start_Stop(t *testing.T) {
	svc := NewService("http://localhost:4000", ":0") // Use port 0 for auto-assignment
	server := NewServer(svc, "0")

	// Test that server can be created and has proper configuration
	assert.NotNil(t, server.http)
	assert.NotNil(t, server.http.Handler)

	// We can't easily test Start() without binding to a real port,
	// but we can verify the server struct is properly initialized
	assert.Equal(t, ":0", server.http.Addr)
}

// Test server with custom reader
func TestServer_WithCustomReader(t *testing.T) {
	svc := NewService("http://localhost:4000", ":8081")

	// Add a custom reader
	customReader := NewDexReadModel() // Use constructor to initialize properly
	customReader.pools["custom-pool"] = PoolInfo{
		ID:     "custom-pool",
		Asset0: "ETH",
		Asset1: "USDC",
	}
	svc.AddReader(customReader)

	server := NewServer(svc, "8081")

	// Test that pools from all readers are returned
	req := httptest.NewRequest("GET", "/api/v1/pools", nil)
	w := httptest.NewRecorder()

	server.handleGetPools(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var pools []PoolInfo
	err := json.NewDecoder(w.Body).Decode(&pools)
	require.NoError(t, err)

	// Should have pools from both readers
	assert.GreaterOrEqual(t, len(pools), 1)

	// Check if custom pool is included
	found := false
	for _, pool := range pools {
		if pool.ID == "custom-pool" {
			found = true
			assert.Equal(t, "ETH", pool.Asset0)
			assert.Equal(t, "USDC", pool.Asset1)
			break
		}
	}
	assert.True(t, found, "Custom pool should be included in response")
}

func TestServer_handleGetSchema(t *testing.T) {
	svc := NewService("http://localhost:4000", ":8081")

	// Add test assets and chains to the DEX reader
	dexReader := svc.readers[0].(*DexReadModel)
	dexReader.assets["BTC"] = "BTC"
	dexReader.assets["ETH"] = "ETH"
	dexReader.assets["HBD"] = "HIVE"
	dexReader.chains["BTC"] = true
	dexReader.chains["ETH"] = true
	dexReader.chains["HIVE"] = true

	server := NewServer(svc, "8081")

	// Create request
	req := httptest.NewRequest("GET", "/api/v1/schema", nil)
	w := httptest.NewRecorder()

	// Call handler
	server.handleGetSchema(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var schema map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&schema)
	require.NoError(t, err)

	// Verify schema structure
	assert.Equal(t, "object", schema["type"])

	// Verify supported chains
	supportedChains, ok := schema["supported_chains"].([]interface{})
	require.True(t, ok)
	assert.Contains(t, supportedChains, "BTC")
	assert.Contains(t, supportedChains, "ETH")
	assert.Contains(t, supportedChains, "HIVE")
}

func TestServer_handleGetSchema_NoReader(t *testing.T) {
	// Create service with no readers
	svc := &Service{
		readers: []ReadModel{},
	}
	server := NewServer(svc, "8081")

	// Create request
	req := httptest.NewRequest("GET", "/api/v1/schema", nil)
	w := httptest.NewRecorder()

	// Call handler
	server.handleGetSchema(w, req)

	// Should return error
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestServer_handleGetPoolAccounts tests GET /api/v1/pools/{id}/accounts (indexer-api.md)
func TestServer_handleGetPoolAccounts(t *testing.T) {
	svc := NewService("http://localhost:4000", ":8081")
	dexReader := svc.readers[0].(*DexReadModel)

	// Create pool and add liquidity positions via legacy events (user journey from docs)
	svc.readers[0].(*DexReadModel).HandleEvent(VSCEvent{
		Type:     "contract_output",
		Contract: "dex-router",
		Method:   "pool_created",
		Args:     json.RawMessage(`{"pool_id":"1","asset0":"HBD","asset1":"HIVE","fee":0.08}`),
	})
	dexReader.HandleEvent(VSCEvent{
		Type:     "contract_output",
		Contract: "dex-router",
		Method:   "liquidity_added",
		Args:     json.RawMessage(`{"pool_id":"1","amount0":500000,"amount1":250000,"lp_tokens":353553,"user":"alice"}`),
	})
	dexReader.HandleEvent(VSCEvent{
		Type:     "contract_output",
		Contract: "dex-router",
		Method:   "liquidity_added",
		Args:     json.RawMessage(`{"pool_id":"1","amount0":500000,"amount1":250000,"lp_tokens":353553,"user":"bob"}`),
	})

	server := NewServer(svc, "8081")
	req := httptest.NewRequest("GET", "/api/v1/pools/1/accounts", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "1"})
	w := httptest.NewRecorder()

	server.handleGetPoolAccounts(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "1", resp["pool_id"])
	accounts, ok := resp["accounts"].([]interface{})
	require.True(t, ok)
	assert.Len(t, accounts, 2)
}

// TestServer_handleGetPoolAccounts_NotFound tests 404 for non-existent pool
func TestServer_handleGetPoolAccounts_NotFound(t *testing.T) {
	svc := NewService("http://localhost:4000", ":8081")
	server := NewServer(svc, "8081")

	req := httptest.NewRequest("GET", "/api/v1/pools/nonexistent/accounts", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "nonexistent"})
	w := httptest.NewRecorder()

	server.handleGetPoolAccounts(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestServer_handleGetPoolRichList_NotFound tests 404 for non-existent pool
func TestServer_handleGetPoolRichList_NotFound(t *testing.T) {
	svc := NewService("http://localhost:4000", ":8081")
	server := NewServer(svc, "8081")

	req := httptest.NewRequest("GET", "/api/v1/pools/nonexistent/richlist", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "nonexistent"})
	w := httptest.NewRecorder()

	server.handleGetPoolRichList(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestServer_handleGetPoolRichList tests GET /api/v1/pools/{id}/richlist (indexer-api.md)
func TestServer_handleGetPoolRichList(t *testing.T) {
	svc := NewService("http://localhost:4000", ":8081")
	dexReader := svc.readers[0].(*DexReadModel)

	dexReader.HandleEvent(VSCEvent{
		Type:     "contract_output",
		Contract: "dex-router",
		Method:   "pool_created",
		Args:     json.RawMessage(`{"pool_id":"1","asset0":"HBD","asset1":"HIVE","fee":0.08}`),
	})
	dexReader.HandleEvent(VSCEvent{
		Type:     "contract_output",
		Contract: "dex-router",
		Method:   "liquidity_added",
		Args:     json.RawMessage(`{"pool_id":"1","amount0":500000,"amount1":250000,"lp_tokens":353553,"user":"alice"}`),
	})
	dexReader.HandleEvent(VSCEvent{
		Type:     "contract_output",
		Contract: "dex-router",
		Method:   "liquidity_added",
		Args:     json.RawMessage(`{"pool_id":"1","amount0":500000,"amount1":250000,"lp_tokens":353553,"user":"bob"}`),
	})

	server := NewServer(svc, "8081")
	req := httptest.NewRequest("GET", "/api/v1/pools/1/richlist?offset=0&limit=50", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "1"})
	w := httptest.NewRecorder()

	server.handleGetPoolRichList(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "1", resp["pool_id"])
	assert.Equal(t, float64(0), resp["offset"])
	assert.Equal(t, float64(50), resp["limit"])
	holders, ok := resp["holders"].([]interface{})
	require.True(t, ok)
	assert.GreaterOrEqual(t, len(holders), 1)
}

// TestServer_handleGetTransactions tests GET /api/v1/transactions (indexer-api.md)
func TestServer_handleGetTransactions(t *testing.T) {
	svc := NewService("http://localhost:4000", ":8081")
	dexReader := svc.readers[0].(*DexReadModel)

	dexReader.HandleEvent(VSCEvent{
		Type:     "contract_output",
		Contract: "dex-router",
		Method:   "pool_created",
		Args:     json.RawMessage(`{"pool_id":"1","asset0":"HBD","asset1":"HIVE","fee":0.08}`),
	})
	dexReader.HandleEvent(VSCEvent{
		Type:        "contract_output",
		Contract:    "dex-router",
		Method:      "swap_executed",
		BlockHeight: 1001,
		TxID:        "tx-swap-123",
		Args:        json.RawMessage(`{"pool_id":"1","amount0":-10000,"amount1":5000}`),
	})

	server := NewServer(svc, "8081")

	// Get all transactions
	req := httptest.NewRequest("GET", "/api/v1/transactions", nil)
	w := httptest.NewRecorder()
	server.handleGetTransactions(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	txs, ok := resp["transactions"].([]interface{})
	require.True(t, ok)
	assert.GreaterOrEqual(t, len(txs), 1)
	assert.GreaterOrEqual(t, int(resp["count"].(float64)), 1)

	// Filter by type=swap (indexer-api.md: type filter)
	req2 := httptest.NewRequest("GET", "/api/v1/transactions?type=swap&limit=10", nil)
	w2 := httptest.NewRecorder()
	server.handleGetTransactions(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)

	// Filter by pool_id (indexer-api.md: pool_id filter)
	req3 := httptest.NewRequest("GET", "/api/v1/transactions?pool_id=1&limit=5", nil)
	w3 := httptest.NewRecorder()
	server.handleGetTransactions(w3, req3)
	assert.Equal(t, http.StatusOK, w3.Code)
}

// TestServer_handleGetTransaction tests GET /api/v1/transactions/{id} (indexer-api.md)
func TestServer_handleGetTransaction(t *testing.T) {
	svc := NewService("http://localhost:4000", ":8081")
	dexReader := svc.readers[0].(*DexReadModel)

	dexReader.HandleEvent(VSCEvent{
		Type:        "contract_output",
		Contract:    "dex-router",
		Method:      "swap_executed",
		BlockHeight: 1002,
		TxID:        "tx-lookup-456",
		Args:        json.RawMessage(`{"pool_id":"1","amount0":-5000,"amount1":2500}`),
	})

	server := NewServer(svc, "8081")
	req := httptest.NewRequest("GET", "/api/v1/transactions/tx-lookup-456", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "tx-lookup-456"})
	w := httptest.NewRecorder()

	server.handleGetTransaction(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var tx TransactionInfo
	require.NoError(t, json.NewDecoder(w.Body).Decode(&tx))
	assert.Equal(t, "tx-lookup-456", tx.ID)
	assert.Equal(t, uint64(1002), tx.BlockHeight)
}

// TestServer_handleGetTransaction_NotFound tests 404 for non-existent transaction
func TestServer_handleGetTransaction_NotFound(t *testing.T) {
	svc := NewService("http://localhost:4000", ":8081")
	server := NewServer(svc, "8081")

	req := httptest.NewRequest("GET", "/api/v1/transactions/nonexistent-tx", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "nonexistent-tx"})
	w := httptest.NewRecorder()

	server.handleGetTransaction(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}
