package tests

import (
	"encoding/binary"
	"testing"
	"vsc-node/lib/test_utils"
	"vsc-node/modules/db/vsc/contracts"
	ledger_db "vsc-node/modules/db/vsc/ledger"

	"github.com/CosmWasm/tinyjson"
	"github.com/stretchr/testify/assert"
	dexcontracts "github.com/vsc-eco/dex-contracts"
	"github.com/vsc-eco/dex-contracts/contracts/types"
)

func TestContractLoading(t *testing.T) {
	assert.NotNil(t, dexcontracts.DexWasm, "dex wasm should load")
	// assert.NotNil(t, dexcontracts.DexRouterWasm, "dex router v1 wasm should load")
	assert.NotNil(t, dexcontracts.DexRouterV2Wasm, "dex router v2 wasm should load")
	assert.NotNil(t, dexcontracts.BTCMappingWasm, "btc-mapping wasm should load")
}

func TestRegisterPoolRejectsUnregisteredTokens(t *testing.T) {
	ct := test_utils.NewContractTest()
	owner := "hive:milo-hpr"
	routerContractId := "router_v2_contract_id"

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
	assert.Contains(t, r.Ret, "not registered", "error should mention unregistered asset")
}

func TestRegisterToken(t *testing.T) {
	ct := test_utils.NewContractTest()
	owner := "hive:milo-hpr"

	routerContractId := "router_v2_contract_id"

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
	owner := "hive:milo-hpr"

	btchbdDexContractId := "btc_hbd_dex_contract_id"
	routerContractId := "router_v2_contract_id"

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

	poolState := ct.StateGet(routerContractId, "pool-"+poolParams.Asset0+"-"+poolParams.Asset1)
	t.Log("pool state value:", poolState)
	assert.Equal(t, btchbdDexContractId, poolState)

	asset0State := ct.StateGet(routerContractId, "asset-"+poolParams.Asset0)
	t.Log("asset 0 state:", asset0State)
	assert.NotEmpty(t, asset0State, "asset0 should be registered")

	asset1State := ct.StateGet(routerContractId, "asset-"+poolParams.Asset1)
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
	owner := "hive:milo-hpr"
	ct.Deposit(owner, 1000, "hbd")
	ct.Deposit(owner, 1000, "hive")

	hivehbdDexContractId := "hive_hbd_dex_contract"

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
	owner := "hive:milo-hpr"

	ct.Deposit(owner, 1000, "hbd")

	btchbdDexId := "hive_hbd_dex_contract"
	btcMappingId := "btc_mapping_contract"

	ct.RegisterContract(btchbdDexId, owner, dexcontracts.DexWasm)
	ct.RegisterContract(btcMappingId, owner, dexcontracts.BTCMappingWasm)
	ct.StateSet(btcMappingId, balancePrefix+owner, formatUintAsBytes(t, 1000000))
	s := ct.StateGet(btcMappingId, balancePrefix+owner)
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

	r = btchbdDex.addLiquidity(t, owner, 1000, 1000)

	if !r.Success {
		t.Errorf("%s: %s", r.Err, r.ErrMsg)
	}

	dumpLogs(t, r.Logs)
	dumpStateDiff(t, r.StateDiff)
}

func TestOneHopNative(t *testing.T) {
	ct := test_utils.NewContractTest()

	owner := "hive:milo-hpr"

	hivehbdDexId := "hive_hbd_dex"
	routerId := "router_v2_contract"

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
		Asset0: poolParams.Asset0,
		Asset1: poolParams.Asset1,
		FeeBps: 100,
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
		Recipient: "hive:milo.vsc",
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

	owner := "hive:milo-hpr"

	btchbdDexId := "btc_hbd_dex"
	routerId := "router_v2_contract"
	btcMappingId := "btc_mapping_contract"

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
	})
	ct.StateSet(btcMappingId, balancePrefix+owner, formatUintAsBytes(t, 2_00000000))
	r = btchbdDex.addLiquidity(t, owner, 1_49000000, 100000_000)
	ct.Deposit(owner, 10000, ledger_db.AssetHbd)

	if !r.Success {
		t.Fatalf("error initializing BTC/HBD pool: %s: %s", r.Err, r.ErrMsg)
	}

	r = router.execute(t, owner, &types.DexInstruction{
		Type:      "swap",
		Version:   "1.0.0",
		AssetIn:   "btc",
		AssetOut:  "hbd",
		AmountIn:  "10000",
		Recipient: "hive:milo.vsc",
		ReturnAddress: &types.ReturnAddress{
			Chain:   "VSC",
			Address: "hive:milo-hpr",
		},
	}, []contracts.Intent{
		{
			Type: "transfer.allow",
			Args: map[string]string{
				"token":       "btc",
				"limit":       "10000",
				"contract_id": btcMappingId,
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

func TestTwoHop(t *testing.T) {
	ct := test_utils.NewContractTest()

	owner := "hive:milo-hpr"

	hivehbdDexId := "hive_hbd_dex"
	btchbdDexId := "btc_hbd_dex"
	routerId := "router_v2_contract"
	btcMappingId := "btc_mapping_contract"

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
		Asset0: hivehbdPoolParams.Asset0,
		Asset1: hivehbdPoolParams.Asset1,
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
	})
	if !r.Success {
		t.Fatalf("error initializing BTC/HBD pool: %s", r.Ret)
	}
	ct.StateSet(btcMappingId, balancePrefix+owner, formatUintAsBytes(t, 20_00000000))
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
		Recipient: "hive:milo.vsc",
		ReturnAddress: &types.ReturnAddress{
			Chain:   "VSC",
			Address: "hive:milo-hpr",
		},
	}, []contracts.Intent{
		{
			Type: "transfer.allow",
			Args: map[string]string{
				"token": "hive",
				"limit": "500000",
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
