package tests

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"math/bits"
	"strconv"
	"strings"
	"testing"
	"time"
	"vsc-node/lib/test_utils"
	contract_session "vsc-node/modules/contract/session"
	"vsc-node/modules/db/vsc/contracts"
	ledger_db "vsc-node/modules/db/vsc/ledger"
	pendulumoracle "vsc-node/modules/incentive-pendulum/oracle"
	state_engine "vsc-node/modules/state-processing"

	"github.com/CosmWasm/tinyjson"

	"github.com/vsc-eco/dex-contracts/contracts/types"
)

// rcHeadroomHBD is spare HBD granted to an account so it can provide RC
// (1:1 with HBD balance post-update) and cover the RC-backing exclusion on
// HBD draws. Comfortably exceeds any single-op gas/exclusion need (≤100k) and
// is negligible relative to pool reserves.
const rcHeadroomHBD = 1_000_000

// knownDexPoolIDs is the full set of pool contract IDs used across the test
// suite. whitelistPendulum always whitelists all of them so a single uniform
// call works regardless of which pool(s) a given test wires up.
var knownDexPoolIDs = []string{
	"vsc1Bjn53csDr6wUoYsjXiN9Nhadu458Tw9wvR",
	"vsc1BmLNMQep1RaaUdYTPfEhqn1inESqNz4Ekt",
	"vsc1Bnuikc8sJii5baG5gmxno4V2xTW7joi2vu",
	"vsc1BquGPy8B766YpstdcL5cSF2GkWVVsVxJS3",
}

// whitelistPendulum wires a deterministic, balanced pendulum geometry (s = 0.5,
// no §9 redirect cliff) into the contract test and whitelists every known pool
// contract ID (plus any extras passed). Swap tests must call this (directly or
// via a setup helper) or the applier rejects every swap with "contract not
// whitelisted" / "snapshot unavailable". Geometry mirrors go-vsc-node's
// balancedGeometry fixture.
func whitelistPendulum(ct *test_utils.ContractTest, extraPools ...string) {
	ct.SetPendulumGeometry(pendulumoracle.GeometryOutputs{
		OK:   true,
		V:    500_000,
		P:    250_000,
		E:    1_000_000,
		T:    1_000_000,
		SBps: 5000, // s = V/E = 0.5
	}, append(append([]string{}, knownDexPoolIDs...), extraPools...))
}

// requireWasm skips the test if the wasm binary is nil (file not found at build time).
func requireWasm(t *testing.T, name string, wasm []byte) {
	t.Helper()
	if len(wasm) == 0 {
		t.Skipf("skipping: %s wasm not available", name)
	}
}

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
			t.Logf("    %*s: %s -> %s\n", 16, key, fmtStoredVal(diff.Previous), fmtStoredVal(diff.Current))
		}
	}
}

func fmtStoredVal(s []byte) string {
	for _, c := range s {
		if c < 0x20 || c > 0x7e {
			return hex.EncodeToString(s)
		}
	}
	return string(s)
}

func formatUintAsBytes(t *testing.T, amount uint64) string {
	t.Helper()
	if amount == 0 {
		return ""
	}
	n := (bits.Len64(amount) + 7) / 8
	t.Log("n:", n)
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], amount)
	t.Log("buf:", buf)
	return string(buf[8-n:])
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
		RcLimit:    2000,
		Intents:    []contracts.Intent{},
		Caller:     caller,
	})
	return &txResult
}

func (c *RouterInfo) registerToken(
	t *testing.T,
	caller string,
	token types.RegisterTokenParams,
) *test_utils.ContractTestCallResult {
	t.Helper()
	payload, _ := tinyjson.Marshal(token)
	txResult := c.ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(t, caller),
		ContractId: c.id,
		Action:     "register_token",
		Payload:    payload,
		RcLimit:    2000,
		Intents:    []contracts.Intent{},
		Caller:     caller,
	})
	return &txResult
}

func (c *RouterInfo) registerPool(
	t *testing.T,
	caller string,
	pool types.RegisterPoolParams,
) *test_utils.ContractTestCallResult {
	t.Helper()

	payload, _ := json.Marshal(pool)
	txResult := c.ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(t, caller),
		ContractId: c.id,
		Action:     "register_pool",
		Payload:    payload,
		RcLimit:    2000,
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
		RcLimit:    2000,
		Intents:    []contracts.Intent{},
		Caller:     caller,
	})
	return &txResult
}

func (c *RouterInfo) execute(
	t *testing.T,
	caller string,
	instruction *types.DexInstruction,
	intents []contracts.Intent,
) *test_utils.ContractTestCallResult {
	t.Helper()

	payload, _ := tinyjson.Marshal(instruction)
	callOpts := state_engine.TxVscCallContract{
		Self:       *basicSelf(t, caller),
		ContractId: c.id,
		Action:     "execute",
		Payload:    payload,
		// Two-hop / cross-chain (unmap) swaps fan out into several inter-contract
		// calls; 2000 caps them. Callers are funded with rcHeadroomHBD, so the
		// larger limit is covered by RC and the exclusion reserve. NOTE: a
		// HIVE→BTC swap+unmap+BTC-tx-build (TestHiveMainnetToBtcMainnet) needs
		// ~17.4k RC, above the 10k per-op cap, so that one test remains flagged.
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
	instruction *types.InitParams,
) *test_utils.ContractTestCallResult {
	t.Helper()
	payload, _ := tinyjson.Marshal(instruction)
	// Mirror the pool's alphabetical normalization so test helpers
	// (addLiquidity, etc.) use the correct asset ordering.
	a0 := strings.ToLower(instruction.Asset0)
	a1 := strings.ToLower(instruction.Asset1)
	m0 := instruction.Asset0MappingContract
	m1 := instruction.Asset1MappingContract
	if a0 > a1 {
		a0, a1 = a1, a0
		m0, m1 = m1, m0
	}
	d.asset0 = a0
	d.asset1 = a1
	d.asset0ContractId = m0
	d.asset1ContractId = m1
	r := d.ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(t, caller),
		ContractId: d.id,
		Action:     "init",
		Payload:    payload,
		RcLimit:    2000,
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
	// RC headroom: post-update, RC is 1:1 with HBD balance (plus a 10k free
	// tier) and HBD draws reserve `rcLimit - freeRemaining` to back RC. Pool
	// setup drains the owner's HBD into reserves, leaving nothing to provide
	// RC or cover that reserve on later ops. Grant spare HBD that stays with
	// the owner (never drawn into the pool, so reserves/LP math are unaffected).
	d.ct.Deposit(owner, rcHeadroomHBD, ledger_db.Asset("hbd"))

	input := types.AddLiquidityParams{
		Amount0:   strconv.FormatUint(amount0, 10),
		Amount1:   strconv.FormatUint(amount1, 10),
		Recipient: owner,
	}
	payload, _ := tinyjson.Marshal(input)

	r := d.ct.Call(state_engine.TxVscCallContract{
		Self:       *basicSelf(t, owner),
		Caller:     owner,
		ContractId: d.id,
		Action:     "add_liquidity",
		Payload:    payload,
		RcLimit:    2000,
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
