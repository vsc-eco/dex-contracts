package tests

import (
	"testing"
	"vsc-node/lib/test_utils"

	dexcontracts "vsc-dex-mapping"

	"github.com/CosmWasm/tinyjson"
	"github.com/stretchr/testify/assert"

	routerV2 "dex-router-v2/router-internal"
	dex "dex/dex-internal"
)

func TestRegisterPool(t *testing.T) {
	ct := test_utils.NewContractTest()
	owner := "hive:milo-hpr"

	btchbdDex := "btc_hbd_dex"
	routerContractId := "router_v2_contract"

	id := 0

	ct.RegisterContract(routerContractId, owner, dexcontracts.DexRouterV2Wasm)

	router := &RouterInfo{
		ct: &ct,
		id: routerContractId,
	}

	r := router.initRouterV2(id, owner)
	if r.Err != "" {
		t.Fatalf("error initializing router: %s", r.Err)
	}
	id++

	poolParams := routerV2.RegisterPoolParams{
		Asset0:        "BTC",
		Asset1:        "HBD",
		DexContractId: btchbdDex,
		Asset0Chain:   &[]string{"BTC"}[0],
		Asset1Chain:   &[]string{"HIVE"}[0],
	}

	r = router.registerPool(id, owner, poolParams)
	if r.Err != "" {
		t.Fatalf("error registering pool: %s", r.Err)
	}
	id++

	t.Log("registration return value:", r.Ret)

	poolState := ct.StateGet(routerContractId, "pool/"+poolParams.Asset0+"/"+poolParams.Asset1)
	t.Log("pool state value:", poolState)
	assert.Equal(t, btchbdDex, poolState)

	asset0State := ct.StateGet(routerContractId, "asset/"+poolParams.Asset0)
	t.Log("asset 0 state:", asset0State)
	assert.Equal(t, *poolParams.Asset0Chain, asset0State)

	asset1State := ct.StateGet(routerContractId, "asset/"+poolParams.Asset1)
	t.Log("asset 1 state:", asset1State)
	assert.Equal(t, *poolParams.Asset1Chain, asset1State)
	logStateDiff(r.StateDiff)

	r = router.getSchema(id, owner)
	if r.Err != "" {
		t.Fatalf("error getting schema: %s", r.Err)
	}
	t.Log("router schema:", r.Ret)
	dumpLogs(r.Logs)

	var routerSchema routerV2.SchemaReturn
	tinyjson.Unmarshal([]byte(r.Ret), &routerSchema)
	assert.Contains(t, routerSchema.SupportedChains, "BTC")
	assert.Contains(t, routerSchema.SupportedChains, "HIVE")
}

func TestAddLiquidity(t *testing.T) {
	ct := test_utils.NewContractTest()
	owner := "hive:milo-hpr"

	btchbdDexContractId := "btc_hbd_dex_contract"

	ct.RegisterContract(btchbdDexContractId, "milo.hpr", dexcontracts.DexWasm)

	id := 0

	btchbdDex := &DexInfo{
		ct: &ct,
		id: btchbdDexContractId,
	}

	r := btchbdDex.initPool(id, owner, &dex.InitParams{
		Asset0: "BTC",
		Asset1: "HBD",
		FeeBps: 100,
	})
	if r.Err != "" {
		t.Fatalf("error initializing BTC/HBD pool: %s", r.Err)
	}
	id++

	r = btchbdDex.addLiquidity(id, owner, &dex.AddLiquidityParams{
		Amount0:   1000,
		Amount1:   1000,
		Recipient: owner,
	})
	if r.Err != "" {
		t.Fatalf("error adding liquidity: %s", r.Err)
	}

}

func TestOneHop(t *testing.T) {
	ct := test_utils.NewContractTest()

	owner := "hive:milo-hpr"

	btchbdDexContractId := "btc_hbd_dex"
	routerContractId := "router_v2_contract"

	id := 0

	ct.RegisterContract(btchbdDexContractId, owner, dexcontracts.DexWasm)
	ct.RegisterContract(routerContractId, owner, dexcontracts.DexRouterV2Wasm)

	router := &RouterInfo{
		ct: &ct,
		id: routerContractId,
	}

	btchbdDex := &DexInfo{
		ct: &ct,
		id: btchbdDexContractId,
	}

	poolParams := routerV2.RegisterPoolParams{
		Asset0:        "BTC",
		Asset1:        "HBD",
		DexContractId: btchbdDexContractId,
		Asset0Chain:   &[]string{"BTC"}[0],
		Asset1Chain:   &[]string{"HIVE"}[0],
	}

	r := router.registerPool(id, owner, poolParams)
	if r.Err != "" {
		t.Fatalf("error registering pool: %s", r.Err)
	}
	id++

	btchbdDex.initPool(id, owner, &dex.InitParams{
		Asset0: poolParams.Asset0,
		Asset1: poolParams.Asset1,
		FeeBps: 100,
	})
	if r.Err != "" {
		t.Fatalf("error initializing BTC/HBD pool: %s", r.Err)
	}
	id++

	r = router.execute(id, owner, &routerV2.DexInstruction{
		Type:      "swap",
		Version:   "1.0.0",
		AssetIn:   "BTC",
		AssetOut:  "HBD",
		Recipient: "hive:milo.vsc",
		ReturnAddress: &routerV2.ReturnAddress{
			Chain:   "VSC",
			Address: "hive:milo.hpr",
		},
	})
	id++
	if r.Err != "" {
		t.Errorf("error executing swap: %s", r.Err)
	}

	logStateDiff(r.StateDiff)
	assert.True(t, r.Success)
	t.Log("return:", r.Ret)
}
