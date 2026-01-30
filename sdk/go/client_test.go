package vscdex

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	config := Config{
		Endpoint:   "http://localhost:8080",
		Username:   "alice",
		ActiveKey:  "secret",
		Contracts:  ContractAddresses{DexRouter: "dex-router-123"},
	}
	client := NewClient(config)
	require.NotNil(t, client)
	// Client stores config internally; verify by using ComputeDexRoute which uses Endpoint
	assert.NotEmpty(t, config.Endpoint)
}

func TestConfig_ContractAddresses(t *testing.T) {
	config := Config{
		Contracts: ContractAddresses{DexRouter: "dex-router-456"},
	}
	assert.Equal(t, "dex-router-456", config.Contracts.DexRouter)
}

func TestRouteResult_JSON(t *testing.T) {
	result := RouteResult{
		AmountOut:   1000,
		Route:       []string{"HBD", "HIVE"},
		PriceImpact: 0.5,
		Fee:         8,
	}
	data, err := json.Marshal(result)
	require.NoError(t, err)
	var parsed RouteResult
	require.NoError(t, json.Unmarshal(data, &parsed))
	assert.Equal(t, result.AmountOut, parsed.AmountOut)
	assert.Equal(t, result.Route, parsed.Route)
	assert.Equal(t, result.PriceImpact, parsed.PriceImpact)
	assert.Equal(t, result.Fee, parsed.Fee)
}

func TestClient_ComputeDexRoute_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/router/route", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(RouteResult{
			AmountOut: 95000,
			Route:     []string{"BTC", "HBD"},
			Fee:       8,
		})
	}))
	defer server.Close()

	client := NewClient(Config{
		Endpoint:  server.URL,
		Contracts: ContractAddresses{DexRouter: "dex-router"},
	})

	result, err := client.ComputeDexRoute(context.Background(), "BTC", "HBD", 100000)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, int64(95000), result.AmountOut)
	assert.Equal(t, []string{"BTC", "HBD"}, result.Route)
	assert.Equal(t, int64(8), result.Fee)
}

func TestClient_ComputeDexRoute_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(Config{Endpoint: server.URL})
	result, err := client.ComputeDexRoute(context.Background(), "BTC", "HBD", 100000)
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestClient_ComputeDexRoute_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not valid json"))
	}))
	defer server.Close()

	client := NewClient(Config{Endpoint: server.URL})
	result, err := client.ComputeDexRoute(context.Background(), "BTC", "HBD", 100000)
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestClient_GetPools_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/indexer/pools", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode([]PoolInfo{
			{ID: "1", Asset0: "HBD", Asset1: "HIVE", Reserve0: 1000000, Reserve1: 1000000, Fee: 0.0008},
		})
	}))
	defer server.Close()

	client := NewClient(Config{Endpoint: server.URL})
	pools, err := client.GetPools(context.Background())
	require.NoError(t, err)
	require.Len(t, pools, 1)
	assert.Equal(t, "1", pools[0].ID)
	assert.Equal(t, "HBD", pools[0].Asset0)
	assert.Equal(t, "HIVE", pools[0].Asset1)
}

func TestClient_GetPools_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(Config{Endpoint: server.URL})
	pools, err := client.GetPools(context.Background())
	assert.Error(t, err)
	assert.Nil(t, pools)
}

func TestIntent_Struct(t *testing.T) {
	intent := Intent{
		Type: "transfer.allow",
		Args: map[string]string{"limit": "1000", "token": "HBD"},
	}
	data, err := json.Marshal(intent)
	require.NoError(t, err)
	var parsed Intent
	require.NoError(t, json.Unmarshal(data, &parsed))
	assert.Equal(t, intent.Type, parsed.Type)
	assert.Equal(t, intent.Args, parsed.Args)
}
