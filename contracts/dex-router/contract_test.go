package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strconv"
	"testing"

	"vsc-node/lib/test_utils"
	"vsc-node/modules/db/vsc/contracts"
	stateEngine "vsc-node/modules/state-processing"

	"github.com/stretchr/testify/assert"
)

//go:embed artifacts/main.wasm
var ContractWasm []byte

func printKeys(ct *test_utils.ContractTest, contractId string, keys []string) {
	for _, key := range keys {
		fmt.Printf("%s: %s\n", key, ct.StateGet(contractId, key))
	}
}

func TestDexRouterInit(t *testing.T) {
	ct := test_utils.NewContractTest()
	contractId := "dex_router"
	ct.RegisterContract(contractId, "hive:alice", ContractWasm)

	result, gasUsed, logs := ct.Call(stateEngine.TxVscCallContract{
		Self: stateEngine.TxSelf{
			TxId:                 "init_tx",
			BlockId:              "block:init",
			Index:                0,
			OpIndex:              0,
			Timestamp:            "2025-01-01T00:00:00Z",
			RequiredAuths:        []string{"hive:alice"},
			RequiredPostingAuths: []string{},
		},
		ContractId: contractId,
		Action:     "init",
		Payload:    json.RawMessage([]byte(`"1.0.0"`)),
		RcLimit:    10000,
		Intents:    []contracts.Intent{},
		Caller:     "hive:alice",
	})

	if result.Err != nil {
		fmt.Println("error:", *result.Err)
	}

	if len(logs) > 0 {
		fmt.Println("console logs:")
		for _, logArray := range logs {
			for _, log := range logArray {
				fmt.Println(log)
			}
		}
	}

	assert.True(t, result.Success)
	assert.LessOrEqual(t, gasUsed, uint(1000000000))

	// Check that version was set
	version := ct.StateGet(contractId, "version")
	assert.Equal(t, `"1.0.0"`, version)

	// Check that next pool ID was initialized
	nextPoolId := ct.StateGet(contractId, "next_pool_id")
	assert.Equal(t, `"1"`, nextPoolId)

	fmt.Println("Return value:", result.Ret)
}

func TestCreatePool(t *testing.T) {
	ct := test_utils.NewContractTest()
	contractId := "dex_router"
	ct.RegisterContract(contractId, "hive:alice", ContractWasm)

	// Initialize contract first
	ct.Call(stateEngine.TxVscCallContract{
		Self: stateEngine.TxSelf{
			TxId:                 "init_tx",
			BlockId:              "block:init",
			Index:                0,
			OpIndex:              0,
			Timestamp:            "2025-01-01T00:00:00Z",
			RequiredAuths:        []string{"hive:alice"},
			RequiredPostingAuths: []string{},
		},
		ContractId: contractId,
		Action:     "init",
		Payload:    json.RawMessage([]byte(`"1.0.0"`)),
		RcLimit:    10000,
		Intents:    []contracts.Intent{},
		Caller:     "hive:alice",
	})

	// Create HIVE-HBD pool
	result, gasUsed, logs := ct.Call(stateEngine.TxVscCallContract{
		Self: stateEngine.TxSelf{
			TxId:                 "create_pool_tx",
			BlockId:              "block:create_pool",
			Index:                1,
			OpIndex:              0,
			Timestamp:            "2025-01-01T00:00:01Z",
			RequiredAuths:        []string{"hive:alice"},
			RequiredPostingAuths: []string{},
		},
		ContractId: contractId,
		Action:     "create_pool",
		Payload: json.RawMessage([]byte(`{
			"asset0": "HBD",
			"asset1": "HIVE",
			"fee_bps": 8
		}`)),
		RcLimit: 10000,
		Intents: []contracts.Intent{},
		Caller:  "hive:alice",
	})

	if result.Err != nil {
		fmt.Println("error:", *result.Err)
	}

	if len(logs) > 0 {
		fmt.Println("console logs:")
		for _, logArray := range logs {
			for _, log := range logArray {
				fmt.Println(log)
			}
		}
	}

	assert.True(t, result.Success)
	assert.LessOrEqual(t, gasUsed, uint(1000000000))

	// Check pool was created
	poolAsset0 := ct.StateGet(contractId, "pool/1/asset0")
	poolAsset1 := ct.StateGet(contractId, "pool/1/asset1")
	poolFee := ct.StateGet(contractId, "pool/1/fee")

	assert.Equal(t, `"HBD"`, poolAsset0)
	assert.Equal(t, `"HIVE"`, poolAsset1)
	assert.Equal(t, `"8"`, poolFee)

	// Check next pool ID was incremented
	nextPoolId := ct.StateGet(contractId, "next_pool_id")
	assert.Equal(t, `"2"`, nextPoolId)

	fmt.Println("Return value:", result.Ret)
}

func TestAddLiquidity(t *testing.T) {
	ct := test_utils.NewContractTest()
	contractId := "dex_router"
	ct.RegisterContract(contractId, "hive:alice", ContractWasm)

	// Initialize and create pool
	setupDexTest(&ct, contractId)

	// Add liquidity with proper intents
	intents := []contracts.Intent{
		{
			Type: "transfer.allow",
			Args: map[string]string{
				"limit": strconv.FormatInt(1000000, 10),
				"token": "HBD",
			},
		},
		{
			Type: "transfer.allow",
			Args: map[string]string{
				"limit": strconv.FormatInt(500000, 10),
				"token": "HIVE",
			},
		},
	}

	result, gasUsed, logs := ct.Call(stateEngine.TxVscCallContract{
		Self: stateEngine.TxSelf{
			TxId:                 "add_liquidity_tx",
			BlockId:              "block:add_liquidity",
			Index:                2,
			OpIndex:              0,
			Timestamp:            "2025-01-01T00:00:02Z",
			RequiredAuths:        []string{"hive:alice"},
			RequiredPostingAuths: []string{},
		},
		ContractId: contractId,
		Action:     "execute",
		Payload: json.RawMessage([]byte(`{
			"type": "deposit",
			"version": "1.0.0",
			"asset_in": "HBD",
			"asset_out": "HIVE",
			"recipient": "hive:alice",
			"metadata": {
				"amount0": "1000000",
				"amount1": "500000"
			}
		}`)),
		RcLimit: 10000,
		Intents: intents,
		Caller:  "hive:alice",
	})

	if result.Err != nil {
		fmt.Println("error:", *result.Err)
	}

	if len(logs) > 0 {
		fmt.Println("console logs:")
		for _, logArray := range logs {
			for _, log := range logArray {
				fmt.Println(log)
			}
		}
	}

	assert.True(t, result.Success)
	assert.LessOrEqual(t, gasUsed, uint(1000000000))

	// Check reserves were updated
	reserve0 := ct.StateGet(contractId, "pool/1/reserve0")
	reserve1 := ct.StateGet(contractId, "pool/1/reserve1")
	totalLP := ct.StateGet(contractId, "pool/1/total_lp")

	assert.Equal(t, `"1000000"`, reserve0)
	assert.Equal(t, `"500000"`, reserve1)
	// LP tokens should be sqrt(1000000 * 500000) = sqrt(500000000000) ≈ 707106
	assert.Equal(t, `"707106"`, totalLP)

	fmt.Println("Return value:", result.Ret)
}

func TestDirectSwap(t *testing.T) {
	ct := test_utils.NewContractTest()
	contractId := "dex_router"
	ct.RegisterContract(contractId, "hive:alice", ContractWasm)

	// Setup pool with liquidity
	setupDexTest(&ct, contractId)
	addLiquidityToPool(&ct, contractId, "1", 2000000, 1000000) // 2000 HBD : 1000 HIVE

	// Execute swap with proper intents
	intents := []contracts.Intent{
		{
			Type: "transfer.allow",
			Args: map[string]string{
				"limit": strconv.FormatInt(100000, 10),
				"token": "HBD",
			},
		},
	}

	result, gasUsed, logs := ct.Call(stateEngine.TxVscCallContract{
		Self: stateEngine.TxSelf{
			TxId:                 "swap_tx",
			BlockId:              "block:swap",
			Index:                3,
			OpIndex:              0,
			Timestamp:            "2025-01-01T00:00:03Z",
			RequiredAuths:        []string{"hive:bob"},
			RequiredPostingAuths: []string{},
		},
		ContractId: contractId,
		Action:     "execute",
		Payload: json.RawMessage([]byte(`{
			"type": "swap",
			"version": "1.0.0",
			"asset_in": "HBD",
			"asset_out": "HIVE",
			"recipient": "hive:bob",
			"min_amount_out": 47500,
			"slippage_bps": 50
		}`)),
		RcLimit: 10000,
		Intents: intents,
		Caller:  "hive:bob",
	})

	if result.Err != nil {
		fmt.Println("error:", *result.Err)
	}

	if len(logs) > 0 {
		fmt.Println("console logs:")
		for _, logArray := range logs {
			for _, log := range logArray {
				fmt.Println(log)
			}
		}
	}

	assert.True(t, result.Success)
	assert.LessOrEqual(t, gasUsed, uint(1000000000))

	// Check reserves were updated correctly
	reserve0 := ct.StateGet(contractId, "pool/1/reserve0") // HBD
	reserve1 := ct.StateGet(contractId, "pool/1/reserve1") // HIVE

	// Input: 100000 HBD, expected output: ~47619 HIVE (after 0.08% fee)
	// New reserves: HBD: 2000000 + 99920 = 2099920, HIVE: 1000000 - 47619 = 952381
	assert.Equal(t, `"2099920"`, reserve0)
	assert.Equal(t, `"952381"`, reserve1)

	fmt.Println("Return value:", result.Ret)
}

// Helper functions

func setupDexTest(ct *test_utils.ContractTest, contractId string) {
	// Initialize contract
	ct.Call(stateEngine.TxVscCallContract{
		Self: stateEngine.TxSelf{
			TxId:                 "init_tx",
			BlockId:              "block:init",
			Index:                0,
			OpIndex:              0,
			Timestamp:            "2025-01-01T00:00:00Z",
			RequiredAuths:        []string{"hive:alice"},
			RequiredPostingAuths: []string{},
		},
		ContractId: contractId,
		Action:     "init",
		Payload:    json.RawMessage([]byte(`"1.0.0"`)),
		RcLimit:    10000,
		Intents:    []contracts.Intent{},
		Caller:     "hive:alice",
	})

	// Create HIVE-HBD pool
	ct.Call(stateEngine.TxVscCallContract{
		Self: stateEngine.TxSelf{
			TxId:                 "create_pool_tx",
			BlockId:              "block:create_pool",
			Index:                1,
			OpIndex:              0,
			Timestamp:            "2025-01-01T00:00:01Z",
			RequiredAuths:        []string{"hive:alice"},
			RequiredPostingAuths: []string{},
		},
		ContractId: contractId,
		Action:     "create_pool",
		Payload: json.RawMessage([]byte(`{
			"asset0": "HBD",
			"asset1": "HIVE",
			"fee_bps": 8
		}`)),
		RcLimit: 10000,
		Intents: []contracts.Intent{},
		Caller:  "hive:alice",
	})
}

func addLiquidityToPool(ct *test_utils.ContractTest, contractId, poolId string, amt0, amt1 uint64) {
	intents := []contracts.Intent{
		{
			Type: "transfer.allow",
			Args: map[string]string{
				"limit": strconv.FormatUint(amt0, 10),
				"token": "HBD",
			},
		},
		{
			Type: "transfer.allow",
			Args: map[string]string{
				"limit": strconv.FormatUint(amt1, 10),
				"token": "HIVE",
			},
		},
	}

	ct.Call(stateEngine.TxVscCallContract{
		Self: stateEngine.TxSelf{
			TxId:                 "add_liq_tx_" + poolId,
			BlockId:              "block:add_liq_" + poolId,
			Index:                100,
			OpIndex:              0,
			Timestamp:            "2025-01-01T00:01:00Z",
			RequiredAuths:        []string{"hive:alice"},
			RequiredPostingAuths: []string{},
		},
		ContractId: contractId,
		Action:     "execute",
		Payload: json.RawMessage(fmt.Sprintf(`{
			"type": "deposit",
			"version": "1.0.0",
			"asset_in": "HBD",
			"asset_out": "HIVE",
			"recipient": "hive:alice",
			"metadata": {
				"amount0": "%d",
				"amount1": "%d"
			}
		}`, amt0, amt1)),
		RcLimit: 10000,
		Intents: intents,
		Caller:  "hive:alice",
	})
}
