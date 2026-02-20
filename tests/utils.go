package tests

import (
	"encoding/json"
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

// CallResult wraps the contract call result for test assertions
type CallResult struct {
	state_engine.TxResult
	ErrMsg    string
	Logs      map[string][]string
	StateDiff map[string]contract_session.StateDiff
}

func callResult(txResult state_engine.TxResult, logs map[string][]string) *CallResult {
	cr := &CallResult{TxResult: txResult, Logs: logs, StateDiff: nil}
	if !txResult.Success && txResult.Ret != "" {
		cr.ErrMsg = txResult.Ret
	}
	return cr
}

type RouterInfo struct {
	ct *test_utils.ContractTest
	id string
}

type DexInfo struct {
	ct               *test_utils.ContractTest
	id               string
	asset0           string
	asset1           string
	asset0ContractId string
	asset1ContractId string
}

var txId int64 = 0

func dumpLogs(t *testing.T, logs map[string]contract_session.LogOutput) {
	t.Helper()
	for name, output := range logs {
		if len(output.Logs) > 0 {
			t.Log("logs for", name)
		}
		for _, log := range output.Logs {
			t.Logf("    %s\n", log)
		}
	}
}

func dumpStateDiff(t *testing.T, sdm map[string]contract_session.StateDiff) {
	t.Helper()
	for name, sd := range sdm {
		if len(sd.Deletions) > 0 || len(sd.KeyDiff) > 0 {
			t.Log("state diff for", name)
		}
		for del := range sd.Deletions {
			t.Logf("    %s\n", del)
		}
		for key, diff := range sd.KeyDiff {
			t.Logf("    %*s: %s -> %s\n", 16, key, diff.Previous, diff.Current)
		}
	}
}

func basicSelf(t *testing.T, caller string) *state_engine.TxSelf {
	t.Helper()
	self := state_engine.TxSelf{
		TxId:                 strconv.FormatInt(txId, 10),
		BlockId:              strconv.FormatInt(txId, 10),
		Index:                0,
		OpIndex:              0,
		Timestamp:            time.Now().String(),
		RequiredAuths:        []string{caller},
		RequiredPostingAuths: []string{},
	}
	txId++
	return &self
}

func (c RouterInfo) initRouterV2(
	t *testing.T,
	caller string,
) *test_utils.ContractTestCallResult {
	t.Helper()
	txResult := c.ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(t, caller),
		ContractId: c.id,
		Action:     "init",
		Payload:    []byte{},
		RcLimit:    1000,
		Intents:    []contracts.Intent{},
		Caller:     caller,
	})
	return &txResult
}

func (c *RouterInfo) registerToken(
	t *testing.T,
	caller string,
	token routerV2.RegisterTokenParams,
) *test_utils.ContractTestCallResult {
	t.Helper()
	payload, _ := tinyjson.Marshal(token)
	txResult := c.ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(t, caller),
		ContractId: c.id,
		Action:     "register_token",
		Payload:    payload,
		RcLimit:    1000,
		Intents:    []contracts.Intent{},
		Caller:     caller,
	})
	return &txResult
}

func (c *RouterInfo) registerPool(
	t *testing.T,
	caller string,
	pool routerV2.RegisterPoolParams,
) *test_utils.ContractTestCallResult {
	t.Helper()

	payload, _ := json.Marshal(pool)
	txResult := c.ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(t, caller),
		ContractId: c.id,
		Action:     "register_pool",
		Payload:    payload,
		RcLimit:    1000,
		Intents:    []contracts.Intent{},
		Caller:     caller,
	})
	return &txResult
}

func (c *RouterInfo) getSchema(
	t *testing.T,
	caller string,
) *test_utils.ContractTestCallResult {
	t.Helper()

	txResult := c.ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(t, caller),
		ContractId: c.id,
		Action:     "get_schema",
		Payload:    []byte{},
		RcLimit:    1000,
		Intents:    []contracts.Intent{},
		Caller:     caller,
	})
	return &txResult
}

func (c *RouterInfo) execute(
	t *testing.T,
	caller string,
	instruction *routerV2.DexInstruction,
	intents []contracts.Intent,
) *test_utils.ContractTestCallResult {
	t.Helper()

	payload, _ := tinyjson.Marshal(instruction)
	callOpts := state_engine.TxVscCallContract{
		Self:       *basicSelf(t, caller),
		ContractId: c.id,
		Action:     "execute",
		Payload:    payload,
		RcLimit:    10000,
		Intents:    intents,
		Caller:     caller,
	}

	txResult := c.ct.Call(callOpts)
	return &txResult
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
	d.asset0ContractId = instruction.Asset0MappingContract
	d.asset1ContractId = instruction.Asset1MappingContract
	r := d.ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(t, caller),
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
		Self:       *basicSelf(t, owner),
		Caller:     owner,
		ContractId: d.id,
		Action:     "add_liquidity",
		Payload:    payload,
		RcLimit:    10000,
		Intents: []contracts.Intent{
			{
				Type: "transfer.allow",
				Args: map[string]string{
					"limit":       strconv.FormatUint(amount0, 10),
					"token":       strings.ToLower(d.asset0),
					"contract_id": d.asset0ContractId,
				},
			},
			{
				Type: "transfer.allow",
				Args: map[string]string{
					"limit":       strconv.FormatUint(amount1, 10),
					"token":       strings.ToLower(d.asset1),
					"contract_id": d.asset1ContractId,
				},
			},
		},
	})

	return &r
}
