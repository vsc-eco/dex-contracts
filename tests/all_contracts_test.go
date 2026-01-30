package tests

import (
	"testing"
	"vsc-node/lib/test_utils"
	"vsc-node/modules/db/vsc/contracts"
	state_engine "vsc-node/modules/state-processing"

	dexcontracts "vsc-dex-mapping"

	"github.com/CosmWasm/tinyjson"
	"github.com/stretchr/testify/assert"

	routerV2 "dex-router-v2/router-internal"
	dex "dex/dex-internal"
)

func TestContractLoading(t *testing.T) {
	assert.NotNil(t, dexcontracts.DexWasm, "dex wasm should load")
	assert.NotNil(t, dexcontracts.DexRouterWasm, "dex router v1 wasm should load")
	assert.NotNil(t, dexcontracts.DexRouterV2Wasm, "dex router v2 wasm should load")
	assert.NotNil(t, dexcontracts.BTCMappingWasm, "btc-mapping wasm should load")
}

func TestRegisterPoolRejectsUnregisteredTokens(t *testing.T) {
	ct := test_utils.NewContractTest()
	owner := "hive:milo-hpr"
	routerContractId := "router_v2_contract_id"

	ct.RegisterContract(routerContractId, owner, dexcontracts.DexRouterV2Wasm)
	router := &RouterInfo{ct: &ct, id: routerContractId}

	r := router.initRouterV2(t, 0, owner)
	if !r.Success {
		t.Fatalf("error initializing router: %s", r.Ret)
	}

	// Try to register pool without registering tokens first - should fail
	poolParams := routerV2.RegisterPoolParams{
		Asset0:        "BTC",
		Asset1:        "HBD",
		DexContractId: "dex-btc-hbd",
	}
	r = router.registerPool(t, 1, owner, poolParams)
	assert.False(t, r.Success, "register_pool should fail when assets not registered")
	assert.Contains(t, r.Ret, "not registered", "error should mention unregistered asset")
}

func TestRegisterToken(t *testing.T) {
	ct := test_utils.NewContractTest()
	owner := "hive:milo-hpr"

	routerContractId := "router_v2_contract_id"

	id := 0

	ct.RegisterContract(routerContractId, owner, dexcontracts.DexRouterV2Wasm)

	router := &RouterInfo{
		ct: &ct,
		id: routerContractId,
	}

	r := router.initRouterV2(t, id, owner)
	if !r.Success {
		t.Fatalf("error initializing router: %s", r.Ret)
	}
	id++

	tokenParams := &routerV2.RegisterTokenParams{
		Name: "BTC",
		TokenInfo: routerV2.TokenInfo{
			MappingContract: "",
			Chain:           "BTC",
		},
	}

	r = router.registerToken(t, owner, *tokenParams)

	if !r.Success {
		t.Error("error registering token", r.Ret, r.ErrMsg)
	}
	dumpLogs(t, r.Logs)
	logStateDiff(t, r.StateDiff)
}

func TestRegisterPool(t *testing.T) {
	ct := test_utils.NewContractTest()
	owner := "hive:milo-hpr"

	btchbdDexContractId := "btc_hbd_dex_contract_id"
	routerContractId := "router_v2_contract_id"

	id := 0

	ct.RegisterContract(routerContractId, owner, dexcontracts.DexRouterV2Wasm)

	router := &RouterInfo{
		ct: &ct,
		id: routerContractId,
	}

	r := router.initRouterV2(t, id, owner)
	if !r.Success {
		t.Fatalf("error initializing router: %s", r.Ret)
	}
	id++

	// Tokens MUST be registered before pool
	r = router.registerToken(t, owner, routerV2.RegisterTokenParams{
		Name: "BTC",
		TokenInfo: routerV2.TokenInfo{Chain: "BTC"},
	})
	if !r.Success {
		t.Fatalf("error registering BTC: %s", r.Ret)
	}
	r = router.registerToken(t, owner, routerV2.RegisterTokenParams{
		Name: "HBD",
		TokenInfo: routerV2.TokenInfo{Chain: "HIVE"},
	})
	if !r.Success {
		t.Fatalf("error registering HBD: %s", r.Ret)
	}

	poolParams := routerV2.RegisterPoolParams{
		Asset0:        "BTC",
		Asset1:        "HBD",
		DexContractId: btchbdDexContractId,
	}

	r = router.registerPool(t, id, owner, poolParams)
	if !r.Success {
		t.Fatalf("error registering pool: %s", r.Ret)
	}
	id++

	t.Log("registration return value:", r.Ret)

	poolState := ct.StateGet(routerContractId, "pool/"+poolParams.Asset0+"/"+poolParams.Asset1)
	t.Log("pool state value:", poolState)
	assert.Equal(t, btchbdDexContractId, poolState)

	asset0State := ct.StateGet(routerContractId, "asset/"+poolParams.Asset0)
	t.Log("asset 0 state:", asset0State)
	assert.NotEmpty(t, asset0State, "asset0 should be registered")

	asset1State := ct.StateGet(routerContractId, "asset/"+poolParams.Asset1)
	t.Log("asset 1 state:", asset1State)
	assert.NotEmpty(t, asset1State, "asset1 should be registered")
	logStateDiff(t, r.StateDiff)

	r = router.getSchema(t, id, owner)
	if !r.Success {
		t.Fatalf("error getting schema: %s", r.Ret)
	}
	t.Log("router schema:", r.Ret)
	dumpLogs(t, r.Logs)

	var routerSchema routerV2.SchemaReturn
	tinyjson.Unmarshal([]byte(r.Ret), &routerSchema)
	assert.Contains(t, routerSchema.SupportedChains, "BTC")
	assert.Contains(t, routerSchema.SupportedChains, "HIVE")
}

func TestAddLiquidityNativePool(t *testing.T) {
	ct := test_utils.NewContractTest()
	owner := "hive:milo-hpr"
	ct.Deposit(owner, 1000, "hbd")
	ct.Deposit(owner, 1000, "hive")

	btchbdDexContractId := "hive_hbd_dex_contract"

	ct.RegisterContract(btchbdDexContractId, "milo.hpr", dexcontracts.DexWasm)

	id := 0

	btchbdDex := &DexInfo{
		ct: &ct,
		id: btchbdDexContractId,
	}

	r := btchbdDex.initPool(t, owner, &dex.InitParams{
		Asset0: "HIVE",
		Asset1: "HBD",
		FeeBps: 100,
	})
	if !r.Success {
		t.Fatalf("error initializing HIVE/HBD pool: %s", r.Ret)
	}
	id++

	input := dex.AddLiquidityParams{
		Amount0:   900,
		Amount1:   1000,
		Recipient: owner,
	}
	payload, _ := tinyjson.Marshal(input)

	r = btchbdDex.addLiquidity(t, owner, 900, 1000)

	if !r.Success {
		t.Errorf("error adding liquidity: %s", r.Ret)
	}
	dumpLogs(t, r.Logs)
	logStateDiff(t, r.StateDiff)
}

func TestAddLiquidityMappedPool(t *testing.T) {
	ct := test_utils.NewContractTest()
	owner := "hive:milo-hpr"

	ct.Deposit(owner, 1000, "hbd")

	btchbdDexId := "hive_hbd_dex_contract"
	btcMappingId := "btc_mapping_contract"

	ct.RegisterContract(btchbdDexId, owner, dexcontracts.DexWasm)
	ct.RegisterContract(btcMappingId, owner, dexcontracts.BTCMappingWasm)
	ct.StateSet(btcMappingId, "bal/hive:milo-hpr", "1000000")

	id := 0

	btchbdDex := &DexInfo{
		ct: &ct,
		id: btchbdDexId,
	}

	r := btchbdDex.initPool(t, owner, &dex.InitParams{
		Asset0:                "btc",
		Asset1:                "hbd",
		FeeBps:                100,
		Asset0MappingContract: btcMappingId,
	})
	if !r.Success {
		t.Fatalf("error initializing BTC/HBD pool: %s", r.Ret)
	}
	id++

	r = btchbdDex.addLiquidity(t, owner, 1000, 1000)

	if !r.Success {
		t.Errorf("error adding liquidity: %s", r.Ret)
	}
	dumpLogs(t, r.Logs)
	logStateDiff(t, r.StateDiff)
}

func TestOneHopNative(t *testing.T) {
	ct := test_utils.NewContractTest()

	owner := "hive:milo-hpr"

	btchbdDexId := "btc_hbd_dex"
	routerId := "router_v2_contract"

	id := 0

	ct.RegisterContract(btchbdDexId, owner, dexcontracts.DexWasm)
	ct.RegisterContract(routerId, owner, dexcontracts.DexRouterV2Wasm)

	router := &RouterInfo{
		ct: &ct,
		id: routerId,
	}

	btchbdDex := &DexInfo{
		ct: &ct,
		id: btchbdDexId,
	}

	// Tokens MUST be registered before pool
	r := router.registerToken(t, owner, routerV2.RegisterTokenParams{
		Name: "HIVE",
		TokenInfo: routerV2.TokenInfo{Chain: "HIVE"},
	})
	if !r.Success {
		t.Fatalf("error registering HIVE: %s", r.Ret)
	}
	r = router.registerToken(t, owner, routerV2.RegisterTokenParams{
		Name: "HBD",
		TokenInfo: routerV2.TokenInfo{Chain: "HIVE"},
	})
	if !r.Success {
		t.Fatalf("error registering HBD: %s", r.Ret)
	}

	poolParams := routerV2.RegisterPoolParams{
		Asset0:        "hive",
		Asset1:        "hbd",
		DexContractId: btchbdDexId,
	}

	r = router.registerPool(t, id, owner, poolParams)
	if !r.Success {
		t.Fatalf("error registering pool: %s", r.Ret)
	}
	id++

	btchbdDex.initPool(t, owner, &dex.InitParams{
		Asset0: poolParams.Asset0,
		Asset1: poolParams.Asset1,
		FeeBps: 100,
	})
	btchbdDex.addLiquidity(t, owner, 1000, 1000)

	if !r.Success {
		t.Fatalf("error initializing BTC/HBD pool: %s", r.Ret)
	}
	id++

	r = router.execute(t, owner, &routerV2.DexInstruction{
		Type:      "swap",
		Version:   "1.0.0",
		AssetIn:   "hive",
		AssetOut:  "hbd",
		AmountIn:  500,
		Recipient: "hive:milo.vsc",
		ReturnAddress: &routerV2.ReturnAddress{
			Chain:   "VSC",
			Address: "hive:milo-hpr",
		},
	})
	id++
	if !r.Success {
		t.Errorf("error executing swap: %s", r.Ret)
	}

	logStateDiff(t, r.StateDiff)
	assert.True(t, r.Success)
	t.Log("return:", r.Ret)
}

func TestOneHopMapped(t *testing.T) {
	ct := test_utils.NewContractTest()

	owner := "hive:milo-hpr"

	btchbdDexId := "btc_hbd_dex"
	routerId := "router_v2_contract"
	btcMappingId := "btc_mapping_contract"

	id := 0

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
	r := router.registerToken(t, owner, routerV2.RegisterTokenParams{
		Name: "BTC",
		TokenInfo: routerV2.TokenInfo{Chain: "BTC"},
	})
	if !r.Success {
		t.Fatalf("error registering BTC: %s", r.Ret)
	}
	r = router.registerToken(t, owner, routerV2.RegisterTokenParams{
		Name: "HBD",
		TokenInfo: routerV2.TokenInfo{Chain: "HIVE"},
	})
	if !r.Success {
		t.Fatalf("error registering HBD: %s", r.Ret)
	}

	poolParams := routerV2.RegisterPoolParams{
		Asset0:        "btc",
		Asset1:        "hbd",
		DexContractId: btchbdDexId,
	}

	r = router.registerPool(t, id, owner, poolParams)
	if !r.Success {
		t.Fatalf("error registering pool: %s", r.Ret)
	}
	id++

	btchbdDex.initPool(t, owner, &dex.InitParams{
		Asset0:                poolParams.Asset0,
		Asset1:                poolParams.Asset1,
		FeeBps:                100,
		Asset0MappingContract: btcMappingId,
	})
	ct.StateSet(btcMappingId, "bal/hive:milo-hpr", "1000000")
	btchbdDex.addLiquidity(t, owner, 1000, 1000)

	if !r.Success {
		t.Fatalf("error initializing BTC/HBD pool: %s", r.Ret)
	}
	id++

	r = router.execute(t, owner, &routerV2.DexInstruction{
		Type:      "swap",
		Version:   "1.0.0",
		AssetIn:   "btc",
		AssetOut:  "hbd",
		AmountIn:  500,
		Recipient: "hive:milo.vsc",
		ReturnAddress: &routerV2.ReturnAddress{
			Chain:   "VSC",
			Address: "hive:milo-hpr",
		},
	})
	id++
	if !r.Success {
		t.Errorf("error executing swap: %s", r.Ret)
	}

	logStateDiff(t, r.StateDiff)
	assert.True(t, r.Success)
	t.Log("return:", r.Ret)
}
