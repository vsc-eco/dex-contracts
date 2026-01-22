package tests

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"
	"vsc-node/lib/test_utils"
	contract_session "vsc-node/modules/contract/session"
	"vsc-node/modules/db/vsc/contracts"
	state_engine "vsc-node/modules/state-processing"

	"github.com/CosmWasm/tinyjson"

	routerV2 "dex-router-v2/router-internal"
	dex "dex/dex-internal"
)

type RouterInfo struct {
	ct *test_utils.ContractTest
	id string
}

type DexInfo struct {
	ct *test_utils.ContractTest
	id string
}

func dumpLogs(logs map[string]contract_session.LogOutput) {
	for name, output := range logs {
		log.Println("logs for", name)
		for _, log := range output.Logs {
			fmt.Printf("    %s\n", log)
		}
	}
}

func logStateDiff(sdm map[string]contract_session.StateDiff) {
	for name, sd := range sdm {
		log.Println("state diff for", name)
		for del := range sd.Deletions {
			fmt.Printf("    %s\n", del)
		}
		for key, diff := range sd.KeyDiff {
			fmt.Printf("    %*s: %s -> %s\n", 16, key, diff.Previous, diff.Current)
		}
	}
}

func basicSelf(id int, caller string) *state_engine.TxSelf {
	return &state_engine.TxSelf{
		TxId:                 strconv.FormatInt(int64(id), 10),
		BlockId:              strconv.FormatInt(int64(id), 10),
		Index:                0,
		OpIndex:              0,
		Timestamp:            time.Now().String(),
		RequiredAuths:        []string{caller},
		RequiredPostingAuths: []string{},
	}
}

func (c RouterInfo) initRouterV2(
	txId int,
	caller string,
) *test_utils.ContractTestCallResult {
	r := c.ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(txId, caller),
		ContractId: c.id,
		Action:     "init",
		Payload:    []byte{},
		RcLimit:    1000,
		Intents:    []contracts.Intent{},
		Caller:     caller,
	})
	return &r
}

func (c *RouterInfo) registerPool(
	txId int,
	caller string,
	pool routerV2.RegisterPoolParams,
) *test_utils.ContractTestCallResult {
	payload, _ := json.Marshal(pool)
	r := c.ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(txId, caller),
		ContractId: c.id,
		Action:     "register_pool",
		Payload:    payload,
		RcLimit:    1000,
		Intents:    []contracts.Intent{},
		Caller:     caller,
	})
	return &r
}

func (c *RouterInfo) getSchema(
	txId int,
	caller string,
) *test_utils.ContractTestCallResult {
	r := c.ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(txId, caller),
		ContractId: c.id,
		Action:     "get_schema",
		Payload:    []byte{},
		RcLimit:    1000,
		Intents:    []contracts.Intent{},
		Caller:     caller,
	})
	return &r
}

func (c *RouterInfo) execute(
	txId int,
	caller string,
	instruction *routerV2.DexInstruction,
) *test_utils.ContractTestCallResult {
	payload, _ := tinyjson.Marshal(instruction)
	r := c.ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(txId, caller),
		ContractId: c.id,
		Action:     "execute",
		Payload:    payload,
		RcLimit:    1000,
		Intents:    []contracts.Intent{},
		Caller:     caller,
	})
	return &r
}

func (d *DexInfo) initPool(
	txId int,
	caller string,
	instruction *dex.InitParams,
) *test_utils.ContractTestCallResult {
	payload, _ := tinyjson.Marshal(instruction)
	r := d.ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(txId, caller),
		ContractId: d.id,
		Action:     "init",
		Payload:    payload,
		RcLimit:    1000,
		Intents:    []contracts.Intent{},
		Caller:     caller,
	})
	return &r
}

func (d *DexInfo) addLiquidity(
	txId int,
	caller string,
	instruction *dex.AddLiquidityParams,
) *test_utils.ContractTestCallResult {
	payload, _ := tinyjson.Marshal(instruction)
	r := d.ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(txId, caller),
		ContractId: d.id,
		Action:     "add_liquidity",
		Payload:    payload,
		RcLimit:    10000,
		Intents:    []contracts.Intent{},
		Caller:     caller,
	})
	return &r
}
