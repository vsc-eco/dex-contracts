package tests

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"testing"
	"time"
	"vsc-node/lib/test_utils"
	contract_session "vsc-node/modules/contract/session"
	"vsc-node/modules/db/vsc/contracts"
	ledger_db "vsc-node/modules/db/vsc/ledger"
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
	ct     *test_utils.ContractTest
	id     string
	asset0 string
	asset1 string
}

var txId int64 = 0

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

func basicSelf(caller string) *state_engine.TxSelf {
	return &state_engine.TxSelf{
		TxId:                 strconv.FormatInt(txId, 10),
		BlockId:              strconv.FormatInt(txId, 10),
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
		Self:       *basicSelf(caller),
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
		Self:       *basicSelf(caller),
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
		Self:       *basicSelf(caller),
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
	t *testing.T,
	caller string,
	instruction *routerV2.DexInstruction,
) *test_utils.ContractTestCallResult {
	t.Helper()

	payload, _ := tinyjson.Marshal(instruction)
	callOpts := state_engine.TxVscCallContract{
		Self:       *basicSelf(caller),
		ContractId: c.id,
		Action:     "execute",
		Payload:    payload,
		RcLimit:    1000,
		Intents: []contracts.Intent{
			{
				Type: "transfer.allow",
				Args: map[string]string{
					"limit": strconv.FormatUint(uint64(instruction.AmountIn), 10) + ".000",
					"token": instruction.AssetIn,
				},
			},
		},
		Caller: caller,
	}

	fmt.Println("intents", callOpts.Intents)
	r := c.ct.Call(callOpts)
	return &r
}

func (d *DexInfo) initPool(
	t *testing.T,
	caller string,
	instruction *dex.InitParams,
) *test_utils.ContractTestCallResult {
	t.Helper()
	payload, _ := tinyjson.Marshal(instruction)
	d.asset0 = instruction.Asset0
	d.asset1 = instruction.Asset1
	r := d.ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(caller),
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
	t *testing.T,
	owner string,
	amount0, amount1 uint64,
) *test_utils.ContractTestCallResult {
	t.Helper()

	d.ct.Deposit(owner, int64(amount0), ledger_db.Asset(d.asset0))
	d.ct.Deposit(owner, int64(amount1), ledger_db.Asset(d.asset1))

	input := dex.AddLiquidityParams{
		Amount0:   amount0,
		Amount1:   amount1,
		Recipient: owner,
	}
	payload, _ := tinyjson.Marshal(input)

	r := d.ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(owner),
		Caller:     owner,
		ContractId: d.id,
		Action:     "add_liquidity",
		Payload:    payload,
		RcLimit:    10000,
		Intents: []contracts.Intent{
			{
				Type: "transfer.allow",
				Args: map[string]string{
					"limit": strconv.FormatUint(amount0, 10) + ".000",
					"token": strings.ToLower(d.asset0),
				},
			},
			{
				Type: "transfer.allow",
				Args: map[string]string{
					"limit": strconv.FormatUint(amount1, 10) + ".000",
					"token": strings.ToLower(d.asset1),
				},
			},
		},
	})

	if r.Err != "" {
		t.Fatalf("error adding liquidity: %s: %s", r.Err, r.ErrMsg)
	}

	return &r
}
