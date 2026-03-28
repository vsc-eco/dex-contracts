package tests

import (
	"btc-mapping-contract/contract/constants"
	"encoding/binary"
	"strconv"
	"strings"
	"testing"
	"vsc-node/lib/test_utils"
	"vsc-node/modules/db/vsc/contracts"
	ledger_db "vsc-node/modules/db/vsc/ledger"
	state_engine "vsc-node/modules/state-processing"

	"github.com/CosmWasm/tinyjson"
	"github.com/stretchr/testify/assert"
	dexcontracts "github.com/vsc-eco/dex-contracts"
	"github.com/vsc-eco/dex-contracts/contracts/types"
)

func TestRegisterPoolRejectsUnregisteredTokens(t *testing.T) {
	ct := test_utils.NewContractTest()
	t.Cleanup(func() { ct.DataLayer.Stop() })
	owner := "hive:milo-hpr"
	routerContractId := "vsc1BfqCB2b5ppiq4snQP74joWrJ3BMUN58pn9"

	ct.RegisterContract(routerContractId, owner, dexcontracts.DexRouterV2Wasm)
	router := &RouterInfo{ct: &ct, id: routerContractId}

	r := router.initRouterV2(t, owner)
	if !r.Success {
		t.Fatalf("error initializing router: %s", r.Ret)
	}

	// Try to register pool without registering tokens first - should fail
	poolParams := types.RegisterPoolParams{
		Asset0:        "BTC",
		Asset1:        "HBD",
		DexContractId: "dex-btc-hbd",
	}
	r = router.registerPool(t, owner, poolParams)
	assert.False(t, r.Success, "register_pool should fail when assets not registered")
	assert.Contains(t, r.ErrMsg, "not registered", "error should mention unregistered asset")
}

func TestRegisterToken(t *testing.T) {
	ct := test_utils.NewContractTest()
	t.Cleanup(func() { ct.DataLayer.Stop() })
	owner := "hive:milo-hpr"

	routerContractId := "vsc1BfqCB2b5ppiq4snQP74joWrJ3BMUN58pn9"

	ct.RegisterContract(routerContractId, owner, dexcontracts.DexRouterV2Wasm)

	router := &RouterInfo{
		ct: &ct,
		id: routerContractId,
	}

	r := router.initRouterV2(t, owner)
	if !r.Success {
		t.Fatalf("error initializing router: %s", r.Ret)
	}

	tokenParams := &types.RegisterTokenParams{
		Name: "BTC",
		TokenInfo: types.TokenInfo{
			MappingContract: "",
			Chain:           "BTC",
		},
	}

	r = router.registerToken(t, owner, *tokenParams)

	if !r.Success {
		t.Error("error registering token", r.Ret, r.ErrMsg)
	}
	dumpLogs(t, r.Logs)
	dumpStateDiff(t, r.StateDiff)
}

func TestRegisterPool(t *testing.T) {
	ct := test_utils.NewContractTest()
	t.Cleanup(func() { ct.DataLayer.Stop() })
	owner := "hive:milo-hpr"

	btchbdDexContractId := "vsc1Bnuikc8sJii5baG5gmxno4V2xTW7joi2vu"
	routerContractId := "vsc1BfqCB2b5ppiq4snQP74joWrJ3BMUN58pn9"

	ct.RegisterContract(routerContractId, owner, dexcontracts.DexRouterV2Wasm)

	router := &RouterInfo{
		ct: &ct,
		id: routerContractId,
	}

	r := router.initRouterV2(t, owner)
	if !r.Success {
		t.Fatalf("error initializing router: %s", r.Ret)
	}

	// Tokens MUST be registered before pool
	r = router.registerToken(t, owner, types.RegisterTokenParams{
		Name:      "BTC",
		TokenInfo: types.TokenInfo{Chain: "BTC"},
	})
	if !r.Success {
		t.Fatalf("error registering BTC: %s", r.Ret)
	}
	r = router.registerToken(t, owner, types.RegisterTokenParams{
		Name:      "HBD",
		TokenInfo: types.TokenInfo{Chain: "HIVE"},
	})
	if !r.Success {
		t.Fatalf("error registering HBD: %s", r.Ret)
	}

	poolParams := types.RegisterPoolParams{
		Asset0:        "BTC",
		Asset1:        "HBD",
		DexContractId: btchbdDexContractId,
	}

	r = router.registerPool(t, owner, poolParams)
	if !r.Success {
		t.Fatalf("error registering pool: %s", r.Ret)
	}

	t.Log("registration return value:", r.Ret)

	poolState := ct.StateGet(
		routerContractId,
		"pool-"+strings.ToLower(poolParams.Asset0)+"-"+strings.ToLower(poolParams.Asset1),
	)
	t.Log("pool state value:", poolState)
	assert.Equal(t, btchbdDexContractId, poolState)

	asset0State := ct.StateGet(routerContractId, "asset-"+strings.ToLower(poolParams.Asset0))
	t.Log("asset 0 state:", asset0State)
	assert.NotEmpty(t, asset0State, "asset0 should be registered")

	asset1State := ct.StateGet(routerContractId, "asset-"+strings.ToLower(poolParams.Asset1))
	t.Log("asset 1 state:", asset1State)
	assert.NotEmpty(t, asset1State, "asset1 should be registered")
	dumpStateDiff(t, r.StateDiff)

	r = router.getSchema(t, owner)
	if !r.Success {
		t.Fatalf("error getting schema: %s", r.Ret)
	}
	t.Log("router schema:", r.Ret)
	dumpLogs(t, r.Logs)

	var routerSchema types.SchemaReturn
	tinyjson.Unmarshal([]byte(r.Ret), &routerSchema)
	assert.Contains(t, routerSchema.SupportedChains, "BTC")
	assert.Contains(t, routerSchema.SupportedChains, "HIVE")
}

func TestAddLiquidityNativePool(t *testing.T) {
	ct := test_utils.NewContractTest()
	t.Cleanup(func() { ct.DataLayer.Stop() })
	owner := "hive:milo-hpr"
	ct.Deposit(owner, 1000, "hbd")
	ct.Deposit(owner, 1000, "hive")

	hivehbdDexContractId := "vsc1BmLNMQep1RaaUdYTPfEhqn1inESqNz4Ekt"

	ct.RegisterContract(hivehbdDexContractId, "milo.hpr", dexcontracts.DexWasm)

	id := 0

	hivehbdDex := &DexInfo{
		ct: &ct,
		id: hivehbdDexContractId,
	}

	r := hivehbdDex.initPool(t, owner, &types.InitParams{
		Asset0: "HIVE",
		Asset1: "HBD",
	})
	if !r.Success {
		t.Fatalf("error initializing HIVE/HBD pool: %s", r.Ret)
	}
	id++

	r = hivehbdDex.addLiquidity(t, owner, 900, 1000)

	if !r.Success {
		t.Errorf("error adding liquidity: %s", r.Ret)
	}
	dumpLogs(t, r.Logs)
	dumpStateDiff(t, r.StateDiff)
}

func TestAddLiquidityMappedPool(t *testing.T) {
	ct := test_utils.NewContractTest()
	t.Cleanup(func() { ct.DataLayer.Stop() })
	owner := "hive:milo-hpr"

	ct.Deposit(owner, 1000, "hbd")

	btchbdDexId := "vsc1BmLNMQep1RaaUdYTPfEhqn1inESqNz4Ekt"
	btcMappingId := "vsc1BpQYDaMwcfdsh9T7DSEHZvdma1XaSXMPPj"

	ct.RegisterContract(btchbdDexId, owner, dexcontracts.DexWasm)
	ct.RegisterContract(btcMappingId, owner, dexcontracts.BTCMappingWasm)
	ct.StateSet(btcMappingId, constants.BalancePrefix+owner, formatUintAsBytes(t, 1000000))
	s := ct.StateGet(btcMappingId, constants.BalancePrefix+owner)
	var buf [8]byte
	copy(buf[8-len(s):], s)
	t.Log(binary.BigEndian.Uint64(buf[:]))

	btchbdDex := &DexInfo{
		ct: &ct,
		id: btchbdDexId,
	}

	r := btchbdDex.initPool(t, owner, &types.InitParams{
		Asset0:                "btc",
		Asset1:                "hbd",
		FeeBps:                100,
		Asset0MappingContract: btcMappingId,
	})
	if !r.Success {
		t.Fatalf("error initializing BTC/HBD pool: %s: %s", r.Err, r.ErrMsg)
	}
	t.Log("init return:", r.Ret)
	dumpLogs(t, r.Logs)
	dumpStateDiff(t, r.StateDiff)

	// Approve DEX pool to spend owner's mapped BTC tokens
	ct.StateSet(
		btcMappingId,
		constants.AllowancePrefix+owner+constants.DirPathDelimiter+"contract:"+btchbdDexId,
		formatUintAsBytes(t, 1000000),
	)

	r = btchbdDex.addLiquidity(t, owner, 1000, 1000)

	if !r.Success {
		t.Errorf("%s: %s", r.Err, r.ErrMsg)
	}

	dumpLogs(t, r.Logs)
	dumpStateDiff(t, r.StateDiff)
}

func TestOneHopNative(t *testing.T) {
	ct := test_utils.NewContractTest()
	t.Cleanup(func() { ct.DataLayer.Stop() })

	owner := "hive:milo-hpr"

	hivehbdDexId := "vsc1Bjn53csDr6wUoYsjXiN9Nhadu458Tw9wvR"
	routerId := "vsc1Bpc3SgDqCRQxzeDrvV7T4XKV6BZuHmME5F"

	ct.RegisterContract(hivehbdDexId, owner, dexcontracts.DexWasm)
	ct.RegisterContract(routerId, owner, dexcontracts.DexRouterV2Wasm)

	router := &RouterInfo{
		ct: &ct,
		id: routerId,
	}

	hivehbdDex := &DexInfo{
		ct: &ct,
		id: hivehbdDexId,
	}

	// Tokens MUST be registered before pool
	r := router.registerToken(t, owner, types.RegisterTokenParams{
		Name:      "HIVE",
		TokenInfo: types.TokenInfo{Chain: "HIVE"},
	})
	if !r.Success {
		t.Fatalf("error registering HIVE: %s", r.Ret)
	}
	r = router.registerToken(t, owner, types.RegisterTokenParams{
		Name:      "HBD",
		TokenInfo: types.TokenInfo{Chain: "HIVE"},
	})
	if !r.Success {
		t.Fatalf("error registering HBD: %s", r.Ret)
	}

	poolParams := types.RegisterPoolParams{
		Asset0:        "hive",
		Asset1:        "hbd",
		DexContractId: hivehbdDexId,
	}

	r = router.registerPool(t, owner, poolParams)
	if !r.Success {
		t.Fatalf("error registering pool: %s", r.Ret)
	}

	hivehbdDex.initPool(t, owner, &types.InitParams{
		Asset0:         poolParams.Asset0,
		Asset1:         poolParams.Asset1,
		FeeBps:         100,
		RouterContract: routerId,
	})
	hivehbdDex.addLiquidity(t, owner, 100000, 100000)

	if !r.Success {
		t.Fatalf("error initializing HIVE/HBD pool: %s", r.Ret)
	}

	ct.Deposit(owner, int64(50000), ledger_db.Asset("hive"))

	r = router.execute(t, owner, &types.DexInstruction{
		Type:      "swap",
		Version:   "1.0.0",
		AssetIn:   "hive",
		AssetOut:  "hbd",
		AmountIn:  "500",
		Recipient: "hive:milo-vsc",
		ReturnAddress: &types.ReturnAddress{
			Chain:   "VSC",
			Address: "hive:milo-hpr",
		},
	}, []contracts.Intent{
		{
			Type: "transfer.allow",
			Args: map[string]string{
				"token": "hive",
				"limit": "500",
			},
		},
	})
	if !r.Success {
		t.Errorf("%s: %s", r.Err, r.ErrMsg)
	}

	dumpLogs(t, r.Logs)
	dumpStateDiff(t, r.StateDiff)
	assert.True(t, r.Success)
	t.Log("return:", r.Ret)
}

func TestOneHopMapped(t *testing.T) {
	ct := test_utils.NewContractTest()
	t.Cleanup(func() { ct.DataLayer.Stop() })

	owner := "hive:milo-hpr"

	btchbdDexId := "vsc1BquGPy8B766YpstdcL5cSF2GkWVVsVxJS3"
	routerId := "vsc1Bpc3SgDqCRQxzeDrvV7T4XKV6BZuHmME5F"
	btcMappingId := "vsc1BpQYDaMwcfdsh9T7DSEHZvdma1XaSXMPPj"

	ct.RegisterContract(btchbdDexId, owner, dexcontracts.DexWasm)
	ct.RegisterContract(routerId, owner, dexcontracts.DexRouterV2Wasm)
	ct.RegisterContract(btcMappingId, owner, dexcontracts.BTCMappingWasm)

	router := &RouterInfo{
		ct: &ct,
		id: routerId,
	}

	btchbdDex := &DexInfo{
		ct: &ct,
		id: btchbdDexId,
	}

	// Tokens MUST be registered before pool
	r := router.registerToken(t, owner, types.RegisterTokenParams{
		Name:      "BTC",
		TokenInfo: types.TokenInfo{Chain: "BTC", MappingContract: btcMappingId},
	})
	if !r.Success {
		t.Fatalf("error registering BTC: %s", r.Ret)
	}
	r = router.registerToken(t, owner, types.RegisterTokenParams{
		Name:      "HBD",
		TokenInfo: types.TokenInfo{Chain: "HIVE"},
	})
	if !r.Success {
		t.Fatalf("error registering HBD: %s", r.Ret)
	}

	poolParams := types.RegisterPoolParams{
		Asset0:        "btc",
		Asset1:        "hbd",
		DexContractId: btchbdDexId,
	}

	r = router.registerPool(t, owner, poolParams)
	if !r.Success {
		t.Fatalf("error registering pool: %s", r.Ret)
	}

	btchbdDex.initPool(t, owner, &types.InitParams{
		Asset0:                poolParams.Asset0,
		Asset1:                poolParams.Asset1,
		Asset0MappingContract: btcMappingId,
		RouterContract:        routerId,
	})
	ct.StateSet(btcMappingId, constants.BalancePrefix+owner, formatUintAsBytes(t, 2_00000000))
	// Approve DEX pool to spend owner's mapped BTC tokens (for addLiquidity)
	ct.StateSet(
		btcMappingId,
		constants.AllowancePrefix+owner+constants.DirPathDelimiter+"contract:"+btchbdDexId,
		formatUintAsBytes(t, 2_00000000),
	)
	r = btchbdDex.addLiquidity(t, owner, 1_49000000, 100000_000)
	ct.Deposit(owner, 10000, ledger_db.AssetHbd)

	if !r.Success {
		t.Fatalf("error initializing BTC/HBD pool: %s: %s", r.Err, r.ErrMsg)
	}

	// Approve router to spend owner's mapped BTC tokens (for swap)
	ct.StateSet(
		btcMappingId,
		constants.AllowancePrefix+owner+constants.DirPathDelimiter+"contract:"+routerId,
		formatUintAsBytes(t, 50000),
	)

	r = router.execute(t, owner, &types.DexInstruction{
		Type:      "swap",
		Version:   "1.0.0",
		AssetIn:   "btc",
		AssetOut:  "hbd",
		AmountIn:  "10000",
		Recipient: "hive:milo-vsc",
		ReturnAddress: &types.ReturnAddress{
			Chain:   "VSC",
			Address: "hive:milo-hpr",
		},
	}, []contracts.Intent{})
	if !r.Success {
		t.Errorf("%s: %s", r.Err, r.ErrMsg)
	}

	dumpLogs(t, r.Logs)
	dumpStateDiff(t, r.StateDiff)
	assert.True(t, r.Success)
	t.Log("return:", r.Ret)
}

func TestTwoHop(t *testing.T) {
	ct := test_utils.NewContractTest()
	t.Cleanup(func() { ct.DataLayer.Stop() })

	owner := "hive:milo-hpr"

	hivehbdDexId := "vsc1Bjn53csDr6wUoYsjXiN9Nhadu458Tw9wvR"
	btchbdDexId := "vsc1BquGPy8B766YpstdcL5cSF2GkWVVsVxJS3"
	routerId := "vsc1Bpc3SgDqCRQxzeDrvV7T4XKV6BZuHmME5F"
	btcMappingId := "vsc1BpQYDaMwcfdsh9T7DSEHZvdma1XaSXMPPj"

	ct.RegisterContract(hivehbdDexId, owner, dexcontracts.DexWasm)
	ct.RegisterContract(btchbdDexId, owner, dexcontracts.DexWasm)
	ct.RegisterContract(routerId, owner, dexcontracts.DexRouterV2Wasm)
	ct.RegisterContract(btcMappingId, owner, dexcontracts.BTCMappingWasm)

	router := &RouterInfo{
		ct: &ct,
		id: routerId,
	}

	hivehbdDex := &DexInfo{
		ct: &ct,
		id: hivehbdDexId,
	}

	btchbdDex := &DexInfo{
		ct: &ct,
		id: btchbdDexId,
	}

	// Tokens MUST be registered before pool
	r := router.registerToken(t, owner, types.RegisterTokenParams{
		Name:      "HIVE",
		TokenInfo: types.TokenInfo{Chain: "HIVE"},
	})
	if !r.Success {
		t.Fatalf("error registering HIVE: %s", r.Ret)
	}

	r = router.registerToken(t, owner, types.RegisterTokenParams{
		Name:      "BTC",
		TokenInfo: types.TokenInfo{Chain: "BTC", MappingContract: btcMappingId},
	})
	if !r.Success {
		t.Fatalf("error registering BTC: %s", r.Ret)
	}

	r = router.registerToken(t, owner, types.RegisterTokenParams{
		Name:      "HBD",
		TokenInfo: types.TokenInfo{Chain: "HIVE"},
	})
	if !r.Success {
		t.Fatalf("error registering HBD: %s", r.Ret)
	}

	hivehbdPoolParams := types.RegisterPoolParams{
		Asset0:        "hive",
		Asset1:        "hbd",
		DexContractId: hivehbdDexId,
	}
	r = router.registerPool(t, owner, hivehbdPoolParams)
	if !r.Success {
		t.Fatalf("error registering pool: %s", r.Ret)
	}
	t.Log("logs for adding HIVE/HBD pool")
	dumpLogs(t, r.Logs)
	dumpStateDiff(t, r.StateDiff)

	hivehbdDex.initPool(t, owner, &types.InitParams{
		Asset0:         hivehbdPoolParams.Asset0,
		Asset1:         hivehbdPoolParams.Asset1,
		RouterContract: routerId,
	})
	if !r.Success {
		t.Fatalf("error initializing HIVE/HBD pool: %s", r.Ret)
	}
	r = hivehbdDex.addLiquidity(t, owner, 14666412_440, 1000000_000)
	if !r.Success {
		t.Fatalf("error adding liquidity to HIVE/HBD pool: %s: %s", r.Err, r.ErrMsg)
	}

	// BTC/HBD pool
	btchbdPoolParams := types.RegisterPoolParams{
		Asset0:        "btc",
		Asset1:        "hbd",
		DexContractId: btchbdDexId,
	}

	r = router.registerPool(t, owner, btchbdPoolParams)
	if !r.Success {
		t.Fatalf("error registering pool: %s", r.Ret)
	}
	t.Log("logs for adding BTC/HBD pool")
	dumpLogs(t, r.Logs)
	dumpStateDiff(t, r.StateDiff)

	btchbdDex.initPool(t, owner, &types.InitParams{
		Asset0:                btchbdPoolParams.Asset0,
		Asset1:                btchbdPoolParams.Asset1,
		Asset0MappingContract: btcMappingId,
		RouterContract:        routerId,
	})
	if !r.Success {
		t.Fatalf("error initializing BTC/HBD pool: %s", r.Ret)
	}
	ct.StateSet(btcMappingId, constants.BalancePrefix+owner, formatUintAsBytes(t, 20_00000000))
	// Approve DEX pool to spend owner's mapped BTC tokens (for addLiquidity)
	ct.StateSet(
		btcMappingId,
		constants.AllowancePrefix+owner+constants.DirPathDelimiter+"contract:"+btchbdDexId,
		formatUintAsBytes(t, 20_00000000),
	)
	r = btchbdDex.addLiquidity(t, owner, 14_90000000, 1000000_000)
	if !r.Success {
		t.Fatalf("error adding liquidity to BTC/HBD pool: %s: %s", r.Err, r.ErrMsg)
	}

	ct.Deposit(owner, int64(10000_000), ledger_db.Asset("hive"))
	ct.Deposit(owner, int64(10000_000), ledger_db.Asset("hbd"))

	r = router.execute(t, owner, &types.DexInstruction{
		Type:      "swap",
		Version:   "1.0.0",
		AssetIn:   "hive",
		AssetOut:  "btc",
		AmountIn:  "1000000",
		Recipient: "hive:milo-vsc",
		ReturnAddress: &types.ReturnAddress{
			Chain:   "VSC",
			Address: "hive:milo-hpr",
		},
	}, []contracts.Intent{
		{
			Type: "transfer.allow",
			Args: map[string]string{
				"token": "hive",
				"limit": "5000000",
			},
		},
	})
	if !r.Success {
		t.Errorf("%s: %s", r.Err, r.ErrMsg)
	}

	dumpLogs(t, r.Logs)
	dumpStateDiff(t, r.StateDiff)
	assert.True(t, r.Success)
	t.Log("return:", r.Ret)
	t.Log("rc used:", r.RcUsed)
}

func strPtr(s string) *string { return &s }

// setupNativeHiveHbdPool creates a contract test with a HIVE/HBD DEX pool,
// adds the given liquidity amounts, and returns the ContractTest, DexInfo, and dex contract ID.
func setupNativeHiveHbdPool(t *testing.T, liq0, liq1 uint64) (test_utils.ContractTest, *DexInfo, string) {
	t.Helper()
	ct := test_utils.NewContractTest()
	t.Cleanup(func() { ct.DataLayer.Stop() })
	owner := "hive:milo-hpr"
	dexId := "vsc1Bjn53csDr6wUoYsjXiN9Nhadu458Tw9wvR"

	ct.RegisterContract(dexId, owner, dexcontracts.DexWasm)

	dex := &DexInfo{ct: &ct, id: dexId}

	r := dex.initPool(t, owner, &types.InitParams{
		Asset0: "hive",
		Asset1: "hbd",
		FeeBps: 30,
	})
	if !r.Success {
		t.Fatalf("error initializing HIVE/HBD pool: %s: %s", r.Err, r.ErrMsg)
	}

	r = dex.addLiquidity(t, owner, liq0, liq1)
	if !r.Success {
		t.Fatalf("error adding liquidity: %s: %s", r.Err, r.ErrMsg)
	}

	return ct, dex, dexId
}

func TestRemoveLiquidity(t *testing.T) {
	ct, _, dexId := setupNativeHiveHbdPool(t, 1000, 1000)
	owner := "hive:milo-hpr"

	// Remove half the LP tokens
	payload, _ := tinyjson.Marshal(types.RemoveLiquidityParams{
		LpAmount:  "500",
		Recipient: owner,
	})
	r := ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(t, owner),
		ContractId: dexId,
		Action:     "remove_liquidity",
		Payload:    payload,
		RcLimit:    1000,
		Intents:    []contracts.Intent{},
		Caller:     owner,
	})
	if !r.Success {
		t.Fatalf("remove_liquidity failed: %s: %s", r.Err, r.ErrMsg)
	}
	dumpLogs(t, r.Logs)
	dumpStateDiff(t, r.StateDiff)
	t.Log("remove_liquidity return:", r.Ret)

	// Check pool state after removal
	r = ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(t, owner),
		ContractId: dexId,
		Action:     "get_pool",
		Payload:    []byte("{}"),
		RcLimit:    1000,
		Intents:    []contracts.Intent{},
		Caller:     owner,
	})
	if !r.Success {
		t.Fatalf("get_pool failed: %s: %s", r.Err, r.ErrMsg)
	}
	t.Log("pool state after removal:", r.Ret)

	var pool types.PoolInfo
	tinyjson.Unmarshal([]byte(r.Ret), &pool)

	reserve0, _ := strconv.ParseUint(pool.Reserve0, 10, 64)
	reserve1, _ := strconv.ParseUint(pool.Reserve1, 10, 64)
	totalLp, _ := strconv.ParseUint(pool.TotalLp, 10, 64)

	assert.True(t, reserve0 < 1000, "reserve0 should decrease after removing liquidity, got %d", reserve0)
	assert.True(t, reserve1 < 1000, "reserve1 should decrease after removing liquidity, got %d", reserve1)
	assert.True(t, totalLp < 1000, "total LP should decrease after removing liquidity, got %d", totalLp)
	t.Logf("pool after removal: reserve0=%s, reserve1=%s, totalLp=%s", pool.Reserve0, pool.Reserve1, pool.TotalLp)
}

func TestRemoveLiquidityAll(t *testing.T) {
	ct, _, dexId := setupNativeHiveHbdPool(t, 1000, 1000)
	owner := "hive:milo-hpr"

	// First, get pool state to find total LP tokens
	r := ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(t, owner),
		ContractId: dexId,
		Action:     "get_pool",
		Payload:    []byte("{}"),
		RcLimit:    1000,
		Intents:    []contracts.Intent{},
		Caller:     owner,
	})
	if !r.Success {
		t.Fatalf("get_pool failed: %s: %s", r.Err, r.ErrMsg)
	}

	var poolBefore types.PoolInfo
	tinyjson.Unmarshal([]byte(r.Ret), &poolBefore)
	t.Log("pool state before removal:", r.Ret)

	// Remove ALL LP tokens
	payload, _ := tinyjson.Marshal(types.RemoveLiquidityParams{
		LpAmount:  poolBefore.TotalLp,
		Recipient: owner,
	})
	r = ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(t, owner),
		ContractId: dexId,
		Action:     "remove_liquidity",
		Payload:    payload,
		RcLimit:    1000,
		Intents:    []contracts.Intent{},
		Caller:     owner,
	})
	if !r.Success {
		t.Fatalf("remove_liquidity all failed: %s: %s", r.Err, r.ErrMsg)
	}
	dumpLogs(t, r.Logs)
	dumpStateDiff(t, r.StateDiff)
	t.Log("remove_liquidity all return:", r.Ret)

	// Check pool state after full removal
	r = ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(t, owner),
		ContractId: dexId,
		Action:     "get_pool",
		Payload:    []byte("{}"),
		RcLimit:    1000,
		Intents:    []contracts.Intent{},
		Caller:     owner,
	})
	if !r.Success {
		t.Fatalf("get_pool failed: %s: %s", r.Err, r.ErrMsg)
	}
	t.Log("pool state after full removal:", r.Ret)

	var poolAfter types.PoolInfo
	tinyjson.Unmarshal([]byte(r.Ret), &poolAfter)

	reserve0, _ := strconv.ParseUint(poolAfter.Reserve0, 10, 64)
	reserve1, _ := strconv.ParseUint(poolAfter.Reserve1, 10, 64)
	totalLp, _ := strconv.ParseUint(poolAfter.TotalLp, 10, 64)

	assert.Equal(t, uint64(0), reserve0, "reserve0 should be 0 after removing all liquidity")
	assert.Equal(t, uint64(0), reserve1, "reserve1 should be 0 after removing all liquidity")
	assert.Equal(t, uint64(0), totalLp, "total LP should be 0 after removing all liquidity")
}

func TestSwapSlippageProtection(t *testing.T) {
	ct, _, dexId := setupNativeHiveHbdPool(t, 100000, 100000)
	owner := "hive:milo-hpr"

	// Deposit enough HIVE for the swap
	ct.Deposit(owner, 10000, ledger_db.Asset("hive"))

	// Try a swap with min_amount_out set impossibly high
	payload, _ := tinyjson.Marshal(types.SwapParams{
		AssetIn:      "hive",
		AmountIn:     "500",
		AssetOut:     "hbd",
		MinAmountOut: strPtr("9999"),
		From:         owner,
		To:           owner,
	})
	r := ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(t, owner),
		ContractId: dexId,
		Action:     "swap",
		Payload:    payload,
		RcLimit:    1000,
		Intents: []contracts.Intent{
			{
				Type: "transfer.allow",
				Args: map[string]string{
					"token": "hive",
					"limit": "500",
				},
			},
		},
		Caller: owner,
	})
	assert.False(t, r.Success, "swap should fail when min_amount_out exceeds AMM output")
	t.Log("slippage error:", r.ErrMsg)
	dumpLogs(t, r.Logs)
}

func TestSwapZeroAmountFails(t *testing.T) {
	ct, _, dexId := setupNativeHiveHbdPool(t, 100000, 100000)
	owner := "hive:milo-hpr"

	// Try a swap with amount "0"
	payload, _ := tinyjson.Marshal(types.SwapParams{
		AssetIn:  "hive",
		AmountIn: "0",
		AssetOut: "hbd",
		From:     owner,
		To:       owner,
	})
	r := ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(t, owner),
		ContractId: dexId,
		Action:     "swap",
		Payload:    payload,
		RcLimit:    1000,
		Intents: []contracts.Intent{
			{
				Type: "transfer.allow",
				Args: map[string]string{
					"token": "hive",
					"limit": "0",
				},
			},
		},
		Caller: owner,
	})
	assert.False(t, r.Success, "swap with zero amount should fail")
	t.Log("zero amount error:", r.ErrMsg)
	dumpLogs(t, r.Logs)
}

func TestSwapInsufficientLiquidityFails(t *testing.T) {
	ct, _, dexId := setupNativeHiveHbdPool(t, 1000, 1000)
	owner := "hive:milo-hpr"

	// Deposit more than the pool has
	ct.Deposit(owner, 100000, ledger_db.Asset("hive"))

	// Try to swap more than the pool's reserves
	payload, _ := tinyjson.Marshal(types.SwapParams{
		AssetIn:  "hive",
		AmountIn: "100000",
		AssetOut: "hbd",
		From:     owner,
		To:       owner,
	})
	r := ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(t, owner),
		ContractId: dexId,
		Action:     "swap",
		Payload:    payload,
		RcLimit:    1000,
		Intents: []contracts.Intent{
			{
				Type: "transfer.allow",
				Args: map[string]string{
					"token": "hive",
					"limit": "100000",
				},
			},
		},
		Caller: owner,
	})
	assert.False(t, r.Success, "swap exceeding pool liquidity should fail")
	t.Log("insufficient liquidity error:", r.ErrMsg)
	dumpLogs(t, r.Logs)
}

func TestAddLiquidityZeroAmountFails(t *testing.T) {
	ct := test_utils.NewContractTest()
	t.Cleanup(func() { ct.DataLayer.Stop() })
	owner := "hive:milo-hpr"
	dexId := "vsc1Bjn53csDr6wUoYsjXiN9Nhadu458Tw9wvR"

	ct.RegisterContract(dexId, owner, dexcontracts.DexWasm)

	dex := &DexInfo{ct: &ct, id: dexId}

	r := dex.initPool(t, owner, &types.InitParams{
		Asset0: "hive",
		Asset1: "hbd",
		FeeBps: 30,
	})
	if !r.Success {
		t.Fatalf("error initializing pool: %s: %s", r.Err, r.ErrMsg)
	}

	// Try to add liquidity with 0 for one amount
	payload, _ := tinyjson.Marshal(types.AddLiquidityParams{
		Amount0:   "0",
		Amount1:   "1000",
		Recipient: owner,
	})
	result := ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(t, owner),
		ContractId: dexId,
		Action:     "add_liquidity",
		Payload:    payload,
		RcLimit:    1000,
		Intents: []contracts.Intent{
			{
				Type: "transfer.allow",
				Args: map[string]string{
					"limit": "0",
					"token": "hive",
				},
			},
			{
				Type: "transfer.allow",
				Args: map[string]string{
					"limit": "1000",
					"token": "hbd",
				},
			},
		},
		Caller: owner,
	})
	assert.False(t, result.Success, "add_liquidity with zero amount should fail")
	t.Log("zero liquidity error:", result.ErrMsg)
	dumpLogs(t, result.Logs)
}

func TestSwapFeeAccumulation(t *testing.T) {
	ct, _, dexId := setupNativeHiveHbdPool(t, 1000000, 1000000)
	owner := "hive:milo-hpr"

	// Deposit enough for multiple swaps
	ct.Deposit(owner, 100000, ledger_db.Asset("hive"))

	// Get initial pool state
	r := ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(t, owner),
		ContractId: dexId,
		Action:     "get_pool",
		Payload:    []byte("{}"),
		RcLimit:    1000,
		Intents:    []contracts.Intent{},
		Caller:     owner,
	})
	if !r.Success {
		t.Fatalf("get_pool failed: %s: %s", r.Err, r.ErrMsg)
	}
	var poolBefore types.PoolInfo
	tinyjson.Unmarshal([]byte(r.Ret), &poolBefore)
	t.Log("pool before swaps:", r.Ret)

	initialReserve0, _ := strconv.ParseUint(poolBefore.Reserve0, 10, 64)
	initialReserve1, _ := strconv.ParseUint(poolBefore.Reserve1, 10, 64)
	initialProduct := initialReserve0 * initialReserve1

	// Do multiple swaps
	totalIn := uint64(0)
	totalOut := uint64(0)
	for i := 0; i < 5; i++ {
		swapPayload, _ := tinyjson.Marshal(types.SwapParams{
			AssetIn:  "hive",
			AmountIn: "1000",
			AssetOut: "hbd",
			From:     owner,
			To:       owner,
		})
		sr := ct.Call(state_engine.TxVscCallContract{
			Self:       *basicSelf(t, owner),
			ContractId: dexId,
			Action:     "swap",
			Payload:    swapPayload,
			RcLimit:    1000,
			Intents: []contracts.Intent{
				{
					Type: "transfer.allow",
					Args: map[string]string{
						"token": "hive",
						"limit": "1000",
					},
				},
			},
			Caller: owner,
		})
		if !sr.Success {
			t.Fatalf("swap %d failed: %s: %s", i, sr.Err, sr.ErrMsg)
		}
		totalIn += 1000

		var swapResult types.SwapResult
		tinyjson.Unmarshal([]byte(sr.Ret), &swapResult)
		amtOut, _ := strconv.ParseUint(swapResult.AmountOut, 10, 64)
		totalOut += amtOut
		t.Logf("swap %d: in=1000, out=%s", i, swapResult.AmountOut)
	}

	// Get pool state after all swaps
	r = ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(t, owner),
		ContractId: dexId,
		Action:     "get_pool",
		Payload:    []byte("{}"),
		RcLimit:    1000,
		Intents:    []contracts.Intent{},
		Caller:     owner,
	})
	if !r.Success {
		t.Fatalf("get_pool after swaps failed: %s: %s", r.Err, r.ErrMsg)
	}

	var poolAfter types.PoolInfo
	tinyjson.Unmarshal([]byte(r.Ret), &poolAfter)
	t.Log("pool after swaps:", r.Ret)

	afterReserve0, _ := strconv.ParseUint(poolAfter.Reserve0, 10, 64)
	afterReserve1, _ := strconv.ParseUint(poolAfter.Reserve1, 10, 64)
	afterProduct := afterReserve0 * afterReserve1

	// With fees, the constant product should increase (fees stay in the pool)
	assert.True(t, afterProduct >= initialProduct,
		"constant product should not decrease after swaps (fees accumulate): before=%d, after=%d",
		initialProduct, afterProduct)

	// Total output should be less than total input due to fees
	assert.True(t, totalOut < totalIn,
		"total output (%d) should be less than total input (%d) due to fees", totalOut, totalIn)

	t.Logf("fee accumulation check: totalIn=%d, totalOut=%d, fee captured=%d", totalIn, totalOut, totalIn-totalOut)
	t.Logf("constant product: before=%d, after=%d", initialProduct, afterProduct)
}

// TestMultiChainDexIntegration verifies that LTC, DASH, DOGE, and BCH mapping
// contracts all work with the DEX — same flow as BTC (add liquidity + swap).
func TestMultiChainDexIntegration(t *testing.T) {
	chains := []struct {
		name  string
		asset string
		wasm  []byte
	}{
		{"ltc", "ltc", dexcontracts.LTCMappingWasm},
		{"dash", "dash", dexcontracts.DASHMappingWasm},
		{"doge", "doge", dexcontracts.DOGEMappingWasm},
		{"bch", "bch", dexcontracts.BCHMappingWasm},
	}

	for _, chain := range chains {
		t.Run(chain.name, func(t *testing.T) {
			ct := test_utils.NewContractTest()
			t.Cleanup(func() { ct.DataLayer.Stop() })
			owner := "hive:milo-hpr"

			// Use unique contract IDs per chain (hash of chain name gives valid vsc1)
			mappingId := "vsc1BpQYDaMwcfdsh9T7DSEHZvdma1XaSXMPPj"
			dexId := "vsc1BquGPy8B766YpstdcL5cSF2GkWVVsVxJS3"
			routerId := "vsc1Bpc3SgDqCRQxzeDrvV7T4XKV6BZuHmME5F"

			ct.RegisterContract(mappingId, owner, chain.wasm)
			ct.RegisterContract(dexId, owner, dexcontracts.DexWasm)
			ct.RegisterContract(routerId, owner, dexcontracts.DexRouterV2Wasm)

			// Initialize router
			router := &RouterInfo{ct: &ct, id: routerId}
			r := router.initRouterV2(t, owner)
			if !r.Success {
				t.Fatalf("router init failed: %s", r.ErrMsg)
			}

			// Register tokens
			r = router.registerToken(t, owner, types.RegisterTokenParams{
				Name:      strings.ToUpper(chain.asset),
				TokenInfo: types.TokenInfo{Chain: strings.ToUpper(chain.asset), MappingContract: mappingId},
			})
			if !r.Success {
				t.Fatalf("register %s token failed: %s", chain.asset, r.ErrMsg)
			}
			r = router.registerToken(t, owner, types.RegisterTokenParams{
				Name:      "HBD",
				TokenInfo: types.TokenInfo{Chain: "HIVE"},
			})
			if !r.Success {
				t.Fatalf("register HBD token failed: %s", r.ErrMsg)
			}

			// Register pool
			r = router.registerPool(t, owner, types.RegisterPoolParams{
				Asset0:        chain.asset,
				Asset1:        "hbd",
				DexContractId: dexId,
			})
			if !r.Success {
				t.Fatalf("register pool failed: %s", r.ErrMsg)
			}

			// Initialize DEX pool
			dex := &DexInfo{ct: &ct, id: dexId}
			r = dex.initPool(t, owner, &types.InitParams{
				Asset0:                chain.asset,
				Asset1:                "hbd",
				FeeBps:                100,
				Asset0MappingContract: mappingId,
				RouterContract:        routerId,
			})
			if !r.Success {
				t.Fatalf("init %s/HBD pool failed: %s", chain.asset, r.ErrMsg)
			}

			// Seed mapped asset balance and allowance
			ct.StateSet(mappingId, constants.BalancePrefix+owner, formatUintAsBytes(t, 1_00000000))
			ct.StateSet(
				mappingId,
				constants.AllowancePrefix+owner+constants.DirPathDelimiter+"contract:"+dexId,
				formatUintAsBytes(t, 1_00000000),
			)
			ct.Deposit(owner, 100000_000, "hbd")

			// Add liquidity
			r = dex.addLiquidity(t, owner, 1_00000000, 100000_000)
			if !r.Success {
				t.Fatalf("add %s/HBD liquidity failed: %s: %s", chain.asset, r.Err, r.ErrMsg)
			}
			t.Logf("%s/HBD pool: liquidity added successfully", chain.asset)

			// Swap HBD → chain asset via router
			ct.Deposit(owner, 1000, "hbd")
			r = router.execute(t, owner, &types.DexInstruction{
				Type:      "swap",
				Version:   "1.0.0",
				AssetIn:   "hbd",
				AssetOut:  chain.asset,
				AmountIn:  "500",
				Recipient: owner,
				ReturnAddress: &types.ReturnAddress{
					Chain:   "VSC",
					Address: owner,
				},
			}, []contracts.Intent{
				{
					Type: "transfer.allow",
					Args: map[string]string{
						"token": "hbd",
						"limit": "500",
					},
				},
			})
			if !r.Success {
				t.Fatalf("swap HBD→%s failed: %s: %s", chain.asset, r.Err, r.ErrMsg)
			}

			var swapResult types.SwapResult
			tinyjson.Unmarshal([]byte(r.Ret), &swapResult)
			t.Logf("%s swap result: amountOut=%s, reserves=%s/%s",
				chain.asset, swapResult.AmountOut, swapResult.PoolState.Reserve0, swapResult.PoolState.Reserve1)

			amountOut, _ := strconv.ParseUint(swapResult.AmountOut, 10, 64)
			assert.True(t, amountOut > 0, "%s swap should produce output", chain.asset)
		})
	}
}

func TestClaimFees(t *testing.T) {
	ct, _, dexId := setupNativeHiveHbdPool(t, 1000000, 1000000)
	owner := "hive:milo-hpr"

	// Deposit enough for swaps
	ct.Deposit(owner, 100000, ledger_db.Asset("hive"))

	// Do several swaps to accumulate fees
	for i := 0; i < 5; i++ {
		swapPayload, _ := tinyjson.Marshal(types.SwapParams{
			AssetIn:  "hive",
			AmountIn: "1000",
			AssetOut: "hbd",
			From:     owner,
			To:       owner,
		})
		sr := ct.Call(state_engine.TxVscCallContract{
			Self:       *basicSelf(t, owner),
			ContractId: dexId,
			Action:     "swap",
			Payload:    swapPayload,
			RcLimit:    1000,
			Intents: []contracts.Intent{
				{
					Type: "transfer.allow",
					Args: map[string]string{
						"token": "hive",
						"limit": "1000",
					},
				},
			},
			Caller: owner,
		})
		if !sr.Success {
			t.Fatalf("swap %d failed: %s: %s", i, sr.Err, sr.ErrMsg)
		}
	}

	// Call claim_fees as system sender
	systemCaller := "system:claim_fees"
	selfSystem := basicSelf(t, systemCaller)
	r := ct.Call(state_engine.TxVscCallContract{
		Self:       *selfSystem,
		ContractId: dexId,
		Action:     "claim_fees",
		Payload:    []byte("{}"),
		RcLimit:    1000,
		Intents:    []contracts.Intent{},
		Caller:     systemCaller,
	})
	assert.True(t, r.Success, "claim_fees should succeed for system sender, got: %s", r.ErrMsg)
	dumpLogs(t, r.Logs)
	dumpStateDiff(t, r.StateDiff)
}

func TestSwapWithReferralFee(t *testing.T) {
	ct, _, dexId := setupNativeHiveHbdPool(t, 1000000, 1000000)
	owner := "hive:milo-hpr"

	ct.Deposit(owner, 10000, ledger_db.Asset("hive"))

	refBps := uint64(50)
	beneficiary := "hive:referrer"
	swapPayload, _ := tinyjson.Marshal(types.SwapParams{
		AssetIn:     "hive",
		AmountIn:    "5000",
		AssetOut:    "hbd",
		From:        owner,
		To:          owner,
		Beneficiary: &beneficiary,
		RefBps:      &refBps,
	})
	r := ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(t, owner),
		ContractId: dexId,
		Action:     "swap",
		Payload:    swapPayload,
		RcLimit:    1000,
		Intents: []contracts.Intent{
			{
				Type: "transfer.allow",
				Args: map[string]string{
					"token": "hive",
					"limit": "5000",
				},
			},
		},
		Caller: owner,
	})
	assert.True(t, r.Success, "swap with referral should succeed, got: %s", r.ErrMsg)
	dumpLogs(t, r.Logs)
	dumpStateDiff(t, r.StateDiff)

	var swapResult types.SwapResult
	tinyjson.Unmarshal([]byte(r.Ret), &swapResult)
	amountOut, _ := strconv.ParseUint(swapResult.AmountOut, 10, 64)
	assert.True(t, amountOut > 0, "swap output should be > 0")
	t.Logf("swap with referral: amountOut=%s", swapResult.AmountOut)
}

func TestSecondLiquidityProvider(t *testing.T) {
	ct, dex, dexId := setupNativeHiveHbdPool(t, 10000, 10000)
	owner := "hive:milo-hpr"
	alice := "hive:alice"

	// Check pool state after owner's liquidity
	r := ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(t, owner),
		ContractId: dexId,
		Action:     "get_pool",
		Payload:    []byte("{}"),
		RcLimit:    1000,
		Intents:    []contracts.Intent{},
		Caller:     owner,
	})
	if !r.Success {
		t.Fatalf("get_pool failed: %s", r.ErrMsg)
	}
	var poolBefore types.PoolInfo
	tinyjson.Unmarshal([]byte(r.Ret), &poolBefore)
	t.Log("pool before alice:", r.Ret)

	// Alice adds liquidity
	ct.Deposit(alice, 5000, ledger_db.Asset("hive"))
	ct.Deposit(alice, 5000, ledger_db.Asset("hbd"))
	addPayload, _ := tinyjson.Marshal(types.AddLiquidityParams{
		Amount0:   "5000",
		Amount1:   "5000",
		Recipient: alice,
	})
	r = ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(t, alice),
		ContractId: dexId,
		Action:     "add_liquidity",
		Payload:    addPayload,
		RcLimit:    1000,
		Intents: []contracts.Intent{
			{
				Type: "transfer.allow",
				Args: map[string]string{
					"limit":       "5000",
					"token":       strings.ToLower(dex.asset0),
					"contract_id": dex.asset0ContractId,
				},
			},
			{
				Type: "transfer.allow",
				Args: map[string]string{
					"limit":       "5000",
					"token":       strings.ToLower(dex.asset1),
					"contract_id": dex.asset1ContractId,
				},
			},
		},
		Caller: alice,
	})
	assert.True(t, r.Success, "alice add_liquidity should succeed, got: %s", r.ErrMsg)
	dumpLogs(t, r.Logs)
	dumpStateDiff(t, r.StateDiff)

	// Check pool state after alice
	r = ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(t, owner),
		ContractId: dexId,
		Action:     "get_pool",
		Payload:    []byte("{}"),
		RcLimit:    1000,
		Intents:    []contracts.Intent{},
		Caller:     owner,
	})
	if !r.Success {
		t.Fatalf("get_pool failed: %s", r.ErrMsg)
	}
	var poolAfter types.PoolInfo
	tinyjson.Unmarshal([]byte(r.Ret), &poolAfter)
	t.Log("pool after alice:", r.Ret)

	afterReserve0, _ := strconv.ParseUint(poolAfter.Reserve0, 10, 64)
	afterReserve1, _ := strconv.ParseUint(poolAfter.Reserve1, 10, 64)
	afterTotalLp, _ := strconv.ParseUint(poolAfter.TotalLp, 10, 64)

	assert.Equal(t, uint64(15000), afterReserve0, "reserve0 should be 15000")
	assert.Equal(t, uint64(15000), afterReserve1, "reserve1 should be 15000")
	assert.True(t, afterTotalLp > 10000, "total LP should increase beyond initial, got %d", afterTotalLp)

	// Check that both owner and alice have LP tokens
	ownerLp := ct.StateGet(dexId, "lp"+types.DirPathDelimiter+owner)
	aliceLp := ct.StateGet(dexId, "lp"+types.DirPathDelimiter+alice)
	assert.NotEmpty(t, ownerLp, "owner should have LP tokens")
	assert.NotEmpty(t, aliceLp, "alice should have LP tokens")
	t.Logf("owner LP raw length: %d, alice LP raw length: %d", len(ownerLp), len(aliceLp))
}

func TestRemoveLiquidityMappedPool(t *testing.T) {
	ct := test_utils.NewContractTest()
	t.Cleanup(func() { ct.DataLayer.Stop() })
	owner := "hive:milo-hpr"

	ct.Deposit(owner, 1000, "hbd")

	btchbdDexId := "vsc1BquGPy8B766YpstdcL5cSF2GkWVVsVxJS3"
	btcMappingId := "vsc1BpQYDaMwcfdsh9T7DSEHZvdma1XaSXMPPj"

	ct.RegisterContract(btchbdDexId, owner, dexcontracts.DexWasm)
	ct.RegisterContract(btcMappingId, owner, dexcontracts.BTCMappingWasm)
	ct.StateSet(btcMappingId, constants.BalancePrefix+owner, formatUintAsBytes(t, 1000000))

	btchbdDex := &DexInfo{ct: &ct, id: btchbdDexId}

	r := btchbdDex.initPool(t, owner, &types.InitParams{
		Asset0:                "btc",
		Asset1:                "hbd",
		FeeBps:                100,
		Asset0MappingContract: btcMappingId,
	})
	if !r.Success {
		t.Fatalf("error initializing BTC/HBD pool: %s: %s", r.Err, r.ErrMsg)
	}

	// Approve DEX pool to spend owner's mapped BTC tokens
	ct.StateSet(
		btcMappingId,
		constants.AllowancePrefix+owner+constants.DirPathDelimiter+"contract:"+btchbdDexId,
		formatUintAsBytes(t, 1000000),
	)

	r = btchbdDex.addLiquidity(t, owner, 1000, 1000)
	if !r.Success {
		t.Fatalf("add liquidity failed: %s: %s", r.Err, r.ErrMsg)
	}
	dumpLogs(t, r.Logs)

	// Remove half the LP tokens
	payload, _ := tinyjson.Marshal(types.RemoveLiquidityParams{
		LpAmount:  "500",
		Recipient: owner,
	})
	removeResult := ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(t, owner),
		ContractId: btchbdDexId,
		Action:     "remove_liquidity",
		Payload:    payload,
		RcLimit:    1000,
		Intents:    []contracts.Intent{},
		Caller:     owner,
	})
	assert.True(t, removeResult.Success, "remove_liquidity on mapped pool should succeed, got: %s", removeResult.ErrMsg)
	dumpLogs(t, removeResult.Logs)
	dumpStateDiff(t, removeResult.StateDiff)

	// Verify pool reserves decreased
	getPoolResult := ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(t, owner),
		ContractId: btchbdDexId,
		Action:     "get_pool",
		Payload:    []byte("{}"),
		RcLimit:    1000,
		Intents:    []contracts.Intent{},
		Caller:     owner,
	})
	if !getPoolResult.Success {
		t.Fatalf("get_pool failed: %s", getPoolResult.ErrMsg)
	}
	var pool types.PoolInfo
	tinyjson.Unmarshal([]byte(getPoolResult.Ret), &pool)
	reserve0, _ := strconv.ParseUint(pool.Reserve0, 10, 64)
	reserve1, _ := strconv.ParseUint(pool.Reserve1, 10, 64)
	assert.True(t, reserve0 < 1000, "reserve0 should decrease after removal, got %d", reserve0)
	assert.True(t, reserve1 < 1000, "reserve1 should decrease after removal, got %d", reserve1)
	t.Logf(
		"mapped pool after removal: reserve0=%s, reserve1=%s, totalLp=%s",
		pool.Reserve0,
		pool.Reserve1,
		pool.TotalLp,
	)
}

func TestRemoveMoreLpThanOwned(t *testing.T) {
	ct, _, dexId := setupNativeHiveHbdPool(t, 1000, 1000)
	owner := "hive:milo-hpr"

	// Try to remove way more LP tokens than owned
	payload, _ := tinyjson.Marshal(types.RemoveLiquidityParams{
		LpAmount:  "999999",
		Recipient: owner,
	})
	r := ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(t, owner),
		ContractId: dexId,
		Action:     "remove_liquidity",
		Payload:    payload,
		RcLimit:    1000,
		Intents:    []contracts.Intent{},
		Caller:     owner,
	})
	assert.False(t, r.Success, "remove_liquidity with more LP than owned should fail")
	t.Log("excess LP removal error:", r.ErrMsg)
}

func TestDuplicateTokenRegistration(t *testing.T) {
	ct := test_utils.NewContractTest()
	t.Cleanup(func() { ct.DataLayer.Stop() })
	owner := "hive:milo-hpr"
	routerContractId := "vsc1BfqCB2b5ppiq4snQP74joWrJ3BMUN58pn9"

	ct.RegisterContract(routerContractId, owner, dexcontracts.DexRouterV2Wasm)
	router := &RouterInfo{ct: &ct, id: routerContractId}

	r := router.initRouterV2(t, owner)
	if !r.Success {
		t.Fatalf("error initializing router: %s", r.Ret)
	}

	tokenParams := types.RegisterTokenParams{
		Name:      "BTC",
		TokenInfo: types.TokenInfo{Chain: "BTC"},
	}

	// First registration should succeed
	r = router.registerToken(t, owner, tokenParams)
	assert.True(t, r.Success, "first token registration should succeed, got: %s", r.ErrMsg)

	// Second registration should fail (asset already registered)
	r = router.registerToken(t, owner, tokenParams)
	assert.False(t, r.Success, "duplicate token registration should fail")
	t.Log("duplicate token error:", r.ErrMsg)
}

func TestDuplicatePoolRegistration(t *testing.T) {
	ct := test_utils.NewContractTest()
	t.Cleanup(func() { ct.DataLayer.Stop() })
	owner := "hive:milo-hpr"
	routerContractId := "vsc1BfqCB2b5ppiq4snQP74joWrJ3BMUN58pn9"
	dexContractId := "vsc1Bjn53csDr6wUoYsjXiN9Nhadu458Tw9wvR"

	ct.RegisterContract(routerContractId, owner, dexcontracts.DexRouterV2Wasm)
	router := &RouterInfo{ct: &ct, id: routerContractId}

	r := router.initRouterV2(t, owner)
	if !r.Success {
		t.Fatalf("error initializing router: %s", r.Ret)
	}

	// Register tokens first
	r = router.registerToken(t, owner, types.RegisterTokenParams{
		Name:      "HIVE",
		TokenInfo: types.TokenInfo{Chain: "HIVE"},
	})
	if !r.Success {
		t.Fatalf("error registering HIVE: %s", r.ErrMsg)
	}
	r = router.registerToken(t, owner, types.RegisterTokenParams{
		Name:      "HBD",
		TokenInfo: types.TokenInfo{Chain: "HIVE"},
	})
	if !r.Success {
		t.Fatalf("error registering HBD: %s", r.ErrMsg)
	}

	poolParams := types.RegisterPoolParams{
		Asset0:        "hive",
		Asset1:        "hbd",
		DexContractId: dexContractId,
	}

	// First registration
	r = router.registerPool(t, owner, poolParams)
	assert.True(t, r.Success, "first pool registration should succeed, got: %s", r.ErrMsg)

	// Second registration - should fail (duplicate rejected)
	r = router.registerPool(t, owner, poolParams)
	assert.False(t, r.Success, "duplicate pool registration should be rejected")
	assert.Contains(t, r.ErrMsg, "already registered")
}

func TestNonOwnerRegisterToken(t *testing.T) {
	ct := test_utils.NewContractTest()
	t.Cleanup(func() { ct.DataLayer.Stop() })
	owner := "hive:milo-hpr"
	nonOwner := "hive:intruder"
	routerContractId := "vsc1BfqCB2b5ppiq4snQP74joWrJ3BMUN58pn9"

	ct.RegisterContract(routerContractId, owner, dexcontracts.DexRouterV2Wasm)
	router := &RouterInfo{ct: &ct, id: routerContractId}

	r := router.initRouterV2(t, owner)
	if !r.Success {
		t.Fatalf("error initializing router: %s", r.Ret)
	}

	// Non-owner tries to register a token
	r = router.registerToken(t, nonOwner, types.RegisterTokenParams{
		Name:      "BTC",
		TokenInfo: types.TokenInfo{Chain: "BTC"},
	})
	assert.False(t, r.Success, "non-owner should not be able to register tokens")
	t.Log("non-owner register token error:", r.ErrMsg)
}

func TestNonOwnerRegisterPool(t *testing.T) {
	ct := test_utils.NewContractTest()
	t.Cleanup(func() { ct.DataLayer.Stop() })
	owner := "hive:milo-hpr"
	nonOwner := "hive:intruder"
	routerContractId := "vsc1BfqCB2b5ppiq4snQP74joWrJ3BMUN58pn9"

	ct.RegisterContract(routerContractId, owner, dexcontracts.DexRouterV2Wasm)
	router := &RouterInfo{ct: &ct, id: routerContractId}

	r := router.initRouterV2(t, owner)
	if !r.Success {
		t.Fatalf("error initializing router: %s", r.Ret)
	}

	// Register tokens as owner first
	r = router.registerToken(t, owner, types.RegisterTokenParams{
		Name:      "HIVE",
		TokenInfo: types.TokenInfo{Chain: "HIVE"},
	})
	if !r.Success {
		t.Fatalf("error registering HIVE: %s", r.ErrMsg)
	}
	r = router.registerToken(t, owner, types.RegisterTokenParams{
		Name:      "HBD",
		TokenInfo: types.TokenInfo{Chain: "HIVE"},
	})
	if !r.Success {
		t.Fatalf("error registering HBD: %s", r.ErrMsg)
	}

	// Non-owner tries to register a pool
	r = router.registerPool(t, nonOwner, types.RegisterPoolParams{
		Asset0:        "hive",
		Asset1:        "hbd",
		DexContractId: "vsc1Bjn53csDr6wUoYsjXiN9Nhadu458Tw9wvR",
	})
	assert.False(t, r.Success, "non-owner should not be able to register pools")
	t.Log("non-owner register pool error:", r.ErrMsg)
}

func TestRouterGetSchema(t *testing.T) {
	ct := test_utils.NewContractTest()
	t.Cleanup(func() { ct.DataLayer.Stop() })
	owner := "hive:milo-hpr"
	routerContractId := "vsc1BfqCB2b5ppiq4snQP74joWrJ3BMUN58pn9"

	ct.RegisterContract(routerContractId, owner, dexcontracts.DexRouterV2Wasm)
	router := &RouterInfo{ct: &ct, id: routerContractId}

	r := router.initRouterV2(t, owner)
	if !r.Success {
		t.Fatalf("error initializing router: %s", r.Ret)
	}

	// Register a token so chains list is populated
	r = router.registerToken(t, owner, types.RegisterTokenParams{
		Name:      "BTC",
		TokenInfo: types.TokenInfo{Chain: "BTC"},
	})
	if !r.Success {
		t.Fatalf("error registering BTC: %s", r.ErrMsg)
	}

	r = router.getSchema(t, owner)
	assert.True(t, r.Success, "get_schema should succeed, got: %s", r.ErrMsg)
	assert.NotEmpty(t, r.Ret, "get_schema should return non-empty result")
	t.Log("router schema:", r.Ret)

	var schema types.SchemaReturn
	tinyjson.Unmarshal([]byte(r.Ret), &schema)
	assert.True(t, len(schema.SupportedChains) > 0, "should have supported chains")
	assert.Contains(t, schema.SupportedChains, "BTC", "BTC should be in supported chains")
}

func TestRouterGetPool(t *testing.T) {
	ct := test_utils.NewContractTest()
	t.Cleanup(func() { ct.DataLayer.Stop() })
	owner := "hive:milo-hpr"
	routerContractId := "vsc1Bpc3SgDqCRQxzeDrvV7T4XKV6BZuHmME5F"
	hivehbdDexId := "vsc1Bjn53csDr6wUoYsjXiN9Nhadu458Tw9wvR"

	ct.RegisterContract(routerContractId, owner, dexcontracts.DexRouterV2Wasm)
	ct.RegisterContract(hivehbdDexId, owner, dexcontracts.DexWasm)

	router := &RouterInfo{ct: &ct, id: routerContractId}

	r := router.initRouterV2(t, owner)
	if !r.Success {
		t.Fatalf("error initializing router: %s", r.Ret)
	}

	// Register tokens
	r = router.registerToken(t, owner, types.RegisterTokenParams{
		Name:      "HIVE",
		TokenInfo: types.TokenInfo{Chain: "HIVE"},
	})
	if !r.Success {
		t.Fatalf("error registering HIVE: %s", r.ErrMsg)
	}
	r = router.registerToken(t, owner, types.RegisterTokenParams{
		Name:      "HBD",
		TokenInfo: types.TokenInfo{Chain: "HIVE"},
	})
	if !r.Success {
		t.Fatalf("error registering HBD: %s", r.ErrMsg)
	}

	// Register pool
	r = router.registerPool(t, owner, types.RegisterPoolParams{
		Asset0:        "hive",
		Asset1:        "hbd",
		DexContractId: hivehbdDexId,
	})
	if !r.Success {
		t.Fatalf("error registering pool: %s", r.ErrMsg)
	}

	// Init the DEX pool
	hivehbdDex := &DexInfo{ct: &ct, id: hivehbdDexId}
	r = hivehbdDex.initPool(t, owner, &types.InitParams{
		Asset0: "hive",
		Asset1: "hbd",
		FeeBps: 30,
	})
	if !r.Success {
		t.Fatalf("error initializing pool: %s: %s", r.Err, r.ErrMsg)
	}

	// Add liquidity so pool has state
	r = hivehbdDex.addLiquidity(t, owner, 10000, 10000)
	if !r.Success {
		t.Fatalf("error adding liquidity: %s: %s", r.Err, r.ErrMsg)
	}

	// Call get_pool on the router
	getPoolPayload, _ := tinyjson.Marshal(types.GetPoolParams{
		Asset0: "hive",
		Asset1: "hbd",
	})
	getPoolResult := ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(t, owner),
		ContractId: routerContractId,
		Action:     "get_pool",
		Payload:    getPoolPayload,
		RcLimit:    1000,
		Intents:    []contracts.Intent{},
		Caller:     owner,
	})
	assert.True(t, getPoolResult.Success, "router get_pool should succeed, got: %s", getPoolResult.ErrMsg)
	assert.NotEmpty(t, getPoolResult.Ret, "get_pool should return pool info")
	t.Log("router get_pool result:", getPoolResult.Ret)

	var poolInfo types.PoolInfo
	tinyjson.Unmarshal([]byte(getPoolResult.Ret), &poolInfo)
	assert.NotEmpty(t, poolInfo.Reserve0, "pool should have reserve0")
	assert.NotEmpty(t, poolInfo.Reserve1, "pool should have reserve1")
}

func TestSwapMappedToNative(t *testing.T) {
	ct := test_utils.NewContractTest()
	t.Cleanup(func() { ct.DataLayer.Stop() })
	owner := "hive:milo-hpr"

	btchbdDexId := "vsc1BquGPy8B766YpstdcL5cSF2GkWVVsVxJS3"
	btcMappingId := "vsc1BpQYDaMwcfdsh9T7DSEHZvdma1XaSXMPPj"

	ct.RegisterContract(btchbdDexId, owner, dexcontracts.DexWasm)
	ct.RegisterContract(btcMappingId, owner, dexcontracts.BTCMappingWasm)

	btchbdDex := &DexInfo{ct: &ct, id: btchbdDexId}

	r := btchbdDex.initPool(t, owner, &types.InitParams{
		Asset0:                "btc",
		Asset1:                "hbd",
		FeeBps:                100,
		Asset0MappingContract: btcMappingId,
	})
	if !r.Success {
		t.Fatalf("error initializing BTC/HBD pool: %s: %s", r.Err, r.ErrMsg)
	}

	// Seed mapped BTC balance and allowance for adding liquidity
	ct.StateSet(btcMappingId, constants.BalancePrefix+owner, formatUintAsBytes(t, 2_00000000))
	ct.StateSet(
		btcMappingId,
		constants.AllowancePrefix+owner+constants.DirPathDelimiter+"contract:"+btchbdDexId,
		formatUintAsBytes(t, 2_00000000),
	)

	r = btchbdDex.addLiquidity(t, owner, 1_00000000, 100000_000)
	if !r.Success {
		t.Fatalf("error adding liquidity: %s: %s", r.Err, r.ErrMsg)
	}

	// Now swap BTC -> HBD directly on the DEX pool
	// Need BTC balance and allowance for the swap
	ct.StateSet(btcMappingId, constants.BalancePrefix+owner, formatUintAsBytes(t, 50000))
	ct.StateSet(
		btcMappingId,
		constants.AllowancePrefix+owner+constants.DirPathDelimiter+"contract:"+btchbdDexId,
		formatUintAsBytes(t, 50000),
	)

	swapPayload, _ := tinyjson.Marshal(types.SwapParams{
		AssetIn:  "btc",
		AmountIn: "10000",
		AssetOut: "hbd",
		From:     owner,
		To:       owner,
	})
	swapR := ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(t, owner),
		ContractId: btchbdDexId,
		Action:     "swap",
		Payload:    swapPayload,
		RcLimit:    1000,
		Intents:    []contracts.Intent{},
		Caller:     owner,
	})
	assert.True(t, swapR.Success, "BTC->HBD swap should succeed, got: %s", swapR.ErrMsg)
	dumpLogs(t, swapR.Logs)
	dumpStateDiff(t, swapR.StateDiff)

	var swapResult types.SwapResult
	tinyjson.Unmarshal([]byte(swapR.Ret), &swapResult)
	amountOut, _ := strconv.ParseUint(swapResult.AmountOut, 10, 64)
	assert.True(t, amountOut > 0, "swap should produce output, got %d", amountOut)
	t.Logf("BTC->HBD swap: amountOut=%s", swapResult.AmountOut)
}

func TestRouterExecuteDeposit(t *testing.T) {
	ct := test_utils.NewContractTest()
	t.Cleanup(func() { ct.DataLayer.Stop() })
	owner := "hive:milo-hpr"

	hivehbdDexId := "vsc1Bjn53csDr6wUoYsjXiN9Nhadu458Tw9wvR"
	routerId := "vsc1Bpc3SgDqCRQxzeDrvV7T4XKV6BZuHmME5F"

	ct.RegisterContract(hivehbdDexId, owner, dexcontracts.DexWasm)
	ct.RegisterContract(routerId, owner, dexcontracts.DexRouterV2Wasm)

	router := &RouterInfo{ct: &ct, id: routerId}
	hivehbdDex := &DexInfo{ct: &ct, id: hivehbdDexId}

	r := router.initRouterV2(t, owner)
	if !r.Success {
		t.Fatalf("error initializing router: %s", r.Ret)
	}

	// Register tokens
	r = router.registerToken(t, owner, types.RegisterTokenParams{
		Name:      "HIVE",
		TokenInfo: types.TokenInfo{Chain: "HIVE"},
	})
	if !r.Success {
		t.Fatalf("error registering HIVE: %s", r.ErrMsg)
	}
	r = router.registerToken(t, owner, types.RegisterTokenParams{
		Name:      "HBD",
		TokenInfo: types.TokenInfo{Chain: "HIVE"},
	})
	if !r.Success {
		t.Fatalf("error registering HBD: %s", r.ErrMsg)
	}

	// Register pool
	r = router.registerPool(t, owner, types.RegisterPoolParams{
		Asset0:        "hive",
		Asset1:        "hbd",
		DexContractId: hivehbdDexId,
	})
	if !r.Success {
		t.Fatalf("error registering pool: %s", r.ErrMsg)
	}

	// Init the DEX pool with router set
	r = hivehbdDex.initPool(t, owner, &types.InitParams{
		Asset0:         "hive",
		Asset1:         "hbd",
		FeeBps:         30,
		RouterContract: routerId,
	})
	if !r.Success {
		t.Fatalf("error initializing pool: %s: %s", r.Err, r.ErrMsg)
	}

	// Fund owner
	ct.Deposit(owner, 10000, ledger_db.Asset("hive"))
	ct.Deposit(owner, 10000, ledger_db.Asset("hbd"))

	// Execute deposit via router
	r = router.execute(t, owner, &types.DexInstruction{
		Type:      "deposit",
		Version:   "1.0.0",
		AssetIn:   "hive",
		AssetOut:  "hbd",
		AmountIn:  "5000",
		Recipient: owner,
		Metadata: map[string]string{
			"amount0": "5000",
			"amount1": "5000",
		},
	}, []contracts.Intent{
		{
			Type: "transfer.allow",
			Args: map[string]string{
				"token": "hive",
				"limit": "5000",
			},
		},
		{
			Type: "transfer.allow",
			Args: map[string]string{
				"token": "hbd",
				"limit": "5000",
			},
		},
	})
	assert.True(t, r.Success, "router execute deposit should succeed, got: %s", r.ErrMsg)
	dumpLogs(t, r.Logs)
	dumpStateDiff(t, r.StateDiff)

	// Verify pool has liquidity
	getPoolR := ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(t, owner),
		ContractId: hivehbdDexId,
		Action:     "get_pool",
		Payload:    []byte("{}"),
		RcLimit:    1000,
		Intents:    []contracts.Intent{},
		Caller:     owner,
	})
	if !getPoolR.Success {
		t.Fatalf("get_pool failed: %s", getPoolR.ErrMsg)
	}
	var poolInfo types.PoolInfo
	tinyjson.Unmarshal([]byte(getPoolR.Ret), &poolInfo)
	reserve0, _ := strconv.ParseUint(poolInfo.Reserve0, 10, 64)
	assert.True(t, reserve0 > 0, "pool should have reserves after deposit, got %d", reserve0)
	t.Log("pool after deposit:", getPoolR.Ret)
}

func TestRouterExecuteWithdrawal(t *testing.T) {
	ct := test_utils.NewContractTest()
	t.Cleanup(func() { ct.DataLayer.Stop() })
	owner := "hive:milo-hpr"

	hivehbdDexId := "vsc1Bjn53csDr6wUoYsjXiN9Nhadu458Tw9wvR"
	routerId := "vsc1Bpc3SgDqCRQxzeDrvV7T4XKV6BZuHmME5F"

	ct.RegisterContract(hivehbdDexId, owner, dexcontracts.DexWasm)
	ct.RegisterContract(routerId, owner, dexcontracts.DexRouterV2Wasm)

	router := &RouterInfo{ct: &ct, id: routerId}
	hivehbdDex := &DexInfo{ct: &ct, id: hivehbdDexId}

	r := router.initRouterV2(t, owner)
	if !r.Success {
		t.Fatalf("error initializing router: %s", r.Ret)
	}

	// Register tokens
	r = router.registerToken(t, owner, types.RegisterTokenParams{
		Name:      "HIVE",
		TokenInfo: types.TokenInfo{Chain: "HIVE"},
	})
	if !r.Success {
		t.Fatalf("error registering HIVE: %s", r.ErrMsg)
	}
	r = router.registerToken(t, owner, types.RegisterTokenParams{
		Name:      "HBD",
		TokenInfo: types.TokenInfo{Chain: "HIVE"},
	})
	if !r.Success {
		t.Fatalf("error registering HBD: %s", r.ErrMsg)
	}

	// Register pool
	r = router.registerPool(t, owner, types.RegisterPoolParams{
		Asset0:        "hive",
		Asset1:        "hbd",
		DexContractId: hivehbdDexId,
	})
	if !r.Success {
		t.Fatalf("error registering pool: %s", r.ErrMsg)
	}

	// Init the DEX pool with router set
	r = hivehbdDex.initPool(t, owner, &types.InitParams{
		Asset0:         "hive",
		Asset1:         "hbd",
		FeeBps:         30,
		RouterContract: routerId,
	})
	if !r.Success {
		t.Fatalf("error initializing pool: %s: %s", r.Err, r.ErrMsg)
	}

	// Add liquidity directly to get LP tokens
	r = hivehbdDex.addLiquidity(t, owner, 10000, 10000)
	if !r.Success {
		t.Fatalf("error adding liquidity: %s: %s", r.Err, r.ErrMsg)
	}

	// Get pool state to find total LP
	getPoolR := ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(t, owner),
		ContractId: hivehbdDexId,
		Action:     "get_pool",
		Payload:    []byte("{}"),
		RcLimit:    1000,
		Intents:    []contracts.Intent{},
		Caller:     owner,
	})
	if !getPoolR.Success {
		t.Fatalf("get_pool failed: %s", getPoolR.ErrMsg)
	}
	var poolBefore types.PoolInfo
	tinyjson.Unmarshal([]byte(getPoolR.Ret), &poolBefore)
	t.Log("pool before withdrawal:", getPoolR.Ret)

	// Execute withdrawal via router - remove half LP
	totalLp, _ := strconv.ParseUint(poolBefore.TotalLp, 10, 64)
	halfLp := strconv.FormatUint(totalLp/2, 10)

	r = router.execute(t, owner, &types.DexInstruction{
		Type:      "withdrawal",
		Version:   "1.0.0",
		AssetIn:   "hive",
		AssetOut:  "hbd",
		AmountIn:  halfLp,
		Recipient: owner,
		Metadata: map[string]string{
			"lp_amount": halfLp,
		},
	}, []contracts.Intent{})
	assert.True(t, r.Success, "router execute withdrawal should succeed, got: %s", r.ErrMsg)
	dumpLogs(t, r.Logs)
	dumpStateDiff(t, r.StateDiff)

	// Verify reserves decreased
	getPoolR = ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(t, owner),
		ContractId: hivehbdDexId,
		Action:     "get_pool",
		Payload:    []byte("{}"),
		RcLimit:    1000,
		Intents:    []contracts.Intent{},
		Caller:     owner,
	})
	if !getPoolR.Success {
		t.Fatalf("get_pool failed: %s", getPoolR.ErrMsg)
	}
	var poolAfter types.PoolInfo
	tinyjson.Unmarshal([]byte(getPoolR.Ret), &poolAfter)
	afterReserve0, _ := strconv.ParseUint(poolAfter.Reserve0, 10, 64)
	assert.True(t, afterReserve0 < 10000, "reserve0 should decrease after withdrawal, got %d", afterReserve0)
	t.Log("pool after withdrawal:", getPoolR.Ret)
}
