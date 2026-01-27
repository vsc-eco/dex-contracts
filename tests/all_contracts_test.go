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
	if r.Err != "" {
		t.Fatalf("error initializing router: %s", r.Err)
	}
	id++

	tokenParams := &routerV2.RegisterTokenParams{
		Name: "BTC",
		TokenInfo: routerV2.TokenInfo{
			MappingContract: "",
			Chain:           "BTC",
		},
	}

	payload, _ := tinyjson.Marshal(tokenParams)

	resp := ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(t, owner),
		NetId:      "vsc.mainnet",
		ContractId: routerContractId,
		Action:     "register_token",
		Payload:    payload,
		RcLimit:    1000,
		Intents:    []contracts.Intent{},
	})
	r = &resp

	if r.Err != "" {
		t.Error("error registering token", r.Err, r.ErrMsg)
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
	if r.Err != "" {
		t.Fatalf("error initializing router: %s", r.Err)
	}
	id++

	poolParams := routerV2.RegisterPoolParams{
		Asset0:        "BTC",
		Asset1:        "HBD",
		DexContractId: btchbdDexContractId,
		Asset0Chain:   &[]string{"BTC"}[0],
		Asset1Chain:   &[]string{"HIVE"}[0],
	}

	r = router.registerPool(t, id, owner, poolParams)
	if r.Err != "" {
		t.Fatalf("error registering pool: %s", r.Err)
	}
	id++

	t.Log("registration return value:", r.Ret)

	poolState := ct.StateGet(routerContractId, "pool/"+poolParams.Asset0+"/"+poolParams.Asset1)
	t.Log("pool state value:", poolState)
	assert.Equal(t, btchbdDexContractId, poolState)

	asset0State := ct.StateGet(routerContractId, "asset/"+poolParams.Asset0)
	t.Log("asset 0 state:", asset0State)
	assert.Equal(t, *poolParams.Asset0Chain, asset0State)

	asset1State := ct.StateGet(routerContractId, "asset/"+poolParams.Asset1)
	t.Log("asset 1 state:", asset1State)
	assert.Equal(t, *poolParams.Asset1Chain, asset1State)
	logStateDiff(t, r.StateDiff)

	r = router.getSchema(t, id, owner)
	if r.Err != "" {
		t.Fatalf("error getting schema: %s", r.Err)
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
	if r.Err != "" {
		t.Fatalf("error initializing HIVE/HBD pool: %s: %s", r.Err, r.ErrMsg)
	}
	id++

	input := dex.AddLiquidityParams{
		Amount0:   900,
		Amount1:   1000,
		Recipient: owner,
	}
	payload, _ := tinyjson.Marshal(input)

	rnp := ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(t, owner),
		Caller:     owner,
		ContractId: btchbdDexContractId,
		Action:     "add_liquidity",
		Payload:    payload,
		RcLimit:    10000,
		Intents: []contracts.Intent{
			{
				Type: "transfer.allow",
				Args: map[string]string{
					"limit": "1000.000",
					"token": "hbd",
				},
			},
			{
				Type: "transfer.allow",
				Args: map[string]string{
					"limit": "1000.000",
					"token": "hive",
				},
			},
		},
	})
	r = &rnp

	if r.Err != "" {
		t.Errorf("error adding liquidity: %s: %s", r.Err, r.ErrMsg)
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
	if r.Err != "" {
		t.Fatalf("error initializing BTC/HBD pool: %s: %s", r.Err, r.ErrMsg)
	}
	id++

	r = btchbdDex.addLiquidity(t, owner, 1000, 1000)

	if r.Err != "" {
		t.Errorf("error adding liquidity: %s: %s", r.Err, r.ErrMsg)
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

	poolParams := routerV2.RegisterPoolParams{
		Asset0:        "hive",
		Asset1:        "hbd",
		DexContractId: btchbdDexId,
		Asset0Chain:   &[]string{"hive"}[0],
		Asset1Chain:   &[]string{"hive"}[0],
	}

	r := router.registerPool(t, id, owner, poolParams)
	if r.Err != "" {
		t.Fatalf("error registering pool: %s", r.Err)
	}
	id++

	btchbdDex.initPool(t, owner, &dex.InitParams{
		Asset0: poolParams.Asset0,
		Asset1: poolParams.Asset1,
		FeeBps: 100,
	})
	btchbdDex.addLiquidity(t, owner, 1000, 1000)

	if r.Err != "" {
		t.Fatalf("error initializing BTC/HBD pool: %s", r.Err)
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
	if r.Err != "" {
		t.Errorf("error executing swap: %s: %s", r.Err, r.ErrMsg)
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

	poolParams := routerV2.RegisterPoolParams{
		Asset0:        "btc",
		Asset1:        "hbd",
		DexContractId: btchbdDexId,
		Asset0Chain:   &[]string{"btc"}[0],
		Asset1Chain:   &[]string{"hive"}[0],
	}

	r := router.registerPool(t, id, owner, poolParams)
	if r.Err != "" {
		t.Fatalf("error registering pool: %s", r.Err)
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

	if r.Err != "" {
		t.Fatalf("error initializing BTC/HBD pool: %s", r.Err)
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
	if r.Err != "" {
		t.Errorf("error executing swap: %s", r.Err)
	}

	logStateDiff(t, r.StateDiff)
	assert.True(t, r.Success)
	t.Log("return:", r.Ret)
}
