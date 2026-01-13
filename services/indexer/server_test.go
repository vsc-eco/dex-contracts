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
