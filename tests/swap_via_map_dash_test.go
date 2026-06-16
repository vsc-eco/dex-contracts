package tests

import (
	"bytes"
	"encoding/hex"
	"strconv"
	"testing"
	"time"

	"vsc-node/lib/test_utils"
	"vsc-node/modules/db/vsc/contracts"
	state_engine "vsc-node/modules/state-processing"

	"github.com/CosmWasm/tinyjson"
	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/stretchr/testify/assert"
	dexcontracts "github.com/vsc-eco/dex-contracts"
	"github.com/vsc-eco/dex-contracts/contracts/types"

	dashconstants "dash-mapping-contract/contract/constants"
	dashmapping "dash-mapping-contract/contract/mapping"
)

// dashSwapTestParams returns the chain params the dash-mapping testnet
// wasm uses for deposit-address derivation. Mirrors dashTestNetParams
// in dash-mapping-contract/contract/mapping/init.go (not exported there
// so we inline it here). Dash testnet/regtest share the 'y' / '8'-'9'
// address prefixes.
func dashSwapTestParams() *chaincfg.Params {
	p := chaincfg.TestNet3Params
	p.PubKeyHashAddrID = 0x8c // 'y' prefix
	p.ScriptHashAddrID = 0x13 // '8'/'9' prefix
	return &p
}

// dashSwapTestBuildBlockHeader mines a regtest block header whose hash
// satisfies the compact target 0x207fffff (regtest difficulty floor).
// Same nonce-grind loop as the BTC version; Dash uses identical Bitcoin
// block-header layout.
func dashSwapTestBuildBlockHeader(prevBlock, merkleRoot chainhash.Hash, ts time.Time) *wire.BlockHeader {
	h := &wire.BlockHeader{
		Version:    1,
		PrevBlock:  prevBlock,
		MerkleRoot: merkleRoot,
		Timestamp:  ts,
		Bits:       0x207fffff,
		Nonce:      0,
	}
	target := blockchain.CompactToBig(0x207fffff)
	for {
		hash := h.BlockHash()
		if blockchain.HashToBig(&hash).Cmp(target) <= 0 {
			return h
		}
		h.Nonce++
	}
}

// buildDashSwapMapFixture is the Dash analogue of buildSwapMapFixture
// in swap_via_map_test.go. Derives the deposit address for the given
// instruction, builds a 1-input/1-output tx paying `amount` duffs to
// that address, then wraps it in a single-tx block (MerkleRoot = TxHash,
// empty proof, TxIndex = 0).
func buildDashSwapMapFixture(t *testing.T, instruction string, amount int64) (rawTxHex string, blockHeaderRaw string) {
	t.Helper()
	network := dashSwapTestParams()

	address, _, err := dashmapping.DepositAddress(swapTestPrimaryPubKeyHex, swapTestBackupPubKeyHex, instruction, network)
	if err != nil {
		t.Fatal("DepositAddress:", err)
	}

	tx := wire.NewMsgTx(wire.TxVersion)
	tx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Hash: chainhash.Hash{}, Index: 0xffffffff},
		SignatureScript:  []byte{0x00},
		Sequence:         wire.MaxTxInSequenceNum,
	})

	addr, err := btcutil.DecodeAddress(address, network)
	if err != nil {
		t.Fatal("DecodeAddress:", err)
	}
	script, err := txscript.PayToAddrScript(addr)
	if err != nil {
		t.Fatal("PayToAddrScript:", err)
	}
	tx.AddTxOut(&wire.TxOut{Value: amount, PkScript: script})

	txHash := tx.TxHash()
	header := dashSwapTestBuildBlockHeader(chainhash.Hash{}, txHash, time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))

	var txBuf bytes.Buffer
	if err := tx.Serialize(&txBuf); err != nil {
		t.Fatal("serialize tx:", err)
	}
	var hdrBuf bytes.Buffer
	if err := header.Serialize(&hdrBuf); err != nil {
		t.Fatal("serialize header:", err)
	}
	return hex.EncodeToString(txBuf.Bytes()), hdrBuf.String()
}

// TestSwapViaMapDash mirrors TestSwapViaMap (the BTC variant) for the
// Dash chain. Exercises the full map → router → DEX swap path:
//
//  1. A Dash tx pays to the deposit address derived from a swap_to
//     instruction.
//  2. The oracle calls the dash-mapping-contract's "map" action.
//  3. The mapping credits the contract's DASH balance (incAccBalance),
//     approves the DEX pool for the swap amount, then invokes the
//     router which swaps DASH for HBD and delivers HBD to the recipient
//     L2 account.
//
// Bypasses docker/devnet entirely — uses the in-process test_utils
// harness that drives wasm calls directly. Same pattern the BTC
// version has used for months; runs in <30s; catches contract-API
// regressions across the rebase.
//
// Run: go test -v -run TestSwapViaMapDash ./tests/
func TestSwapViaMapDash(t *testing.T) {
	requireWasm(t, "dash-mapping", dexcontracts.DASHMappingWasm)
	const blockHeight = uint32(100)
	const swapAmount = int64(1_000_000) // 0.01 DASH in duffs (above min-dust)
	const recipient = "hive:milo-vsc"
	const oracle = "did:vsc:oracle:dash"
	const owner = "hive:milo-hpr"

	instruction := "swap_to=" + recipient + "&swap_asset_out=hbd"
	rawTxHex, blockHeaderRaw := buildDashSwapMapFixture(t, instruction, swapAmount)

	ct := test_utils.NewContractTest()
	whitelistPendulum(&ct)
	t.Cleanup(func() { ct.DataLayer.Stop() })

	// Reuse the BTC swap test's contract IDs — they're valid base58
	// vsc IDs (sdk.VerifyAddress checks the checksum) and the BTC test
	// is skipped here because its wasm is absent. Using known-good IDs
	// avoids re-deriving valid ones for the Dash side.
	dashMappingId := "vsc1BpQYDaMwcfdsh9T7DSEHZvdma1XaSXMPPj"
	dashhbdDexId := "vsc1BquGPy8B766YpstdcL5cSF2GkWVVsVxJS3"
	routerContractId := "vsc1Bpc3SgDqCRQxzeDrvV7T4XKV6BZuHmME5F"

	ct.RegisterContract(dashMappingId, owner, dexcontracts.DASHMappingWasm)
	ct.RegisterContract(dashhbdDexId, owner, dexcontracts.DexWasm)
	ct.RegisterContract(routerContractId, owner, dexcontracts.DexRouterV2Wasm)

	router := &RouterInfo{ct: &ct, id: routerContractId}
	dashhbdDex := &DexInfo{ct: &ct, id: dashhbdDexId}

	// Register tokens in the router. DASH is registered with the
	// mapping contract pointer so the router can query getInfo for
	// decimals. HBD is native (no mapping_contract).
	r := router.registerToken(t, owner, types.RegisterTokenParams{
		Name:      "DASH",
		TokenInfo: types.TokenInfo{Chain: "DASH", MappingContract: dashMappingId},
	})
	if !r.Success {
		t.Fatalf("register DASH token: %s: %s", r.Err, r.ErrMsg)
	}
	r = router.registerToken(t, owner, types.RegisterTokenParams{
		Name:      "HBD",
		TokenInfo: types.TokenInfo{Chain: "HIVE"},
	})
	if !r.Success {
		t.Fatalf("register HBD token: %s: %s", r.Err, r.ErrMsg)
	}

	// Register the DASH/HBD pool. Pool address ordering is alphabetical
	// (asset0 < asset1), so the pool ends up as asset0=dash, asset1=hbd.
	r = router.registerPool(t, owner, types.RegisterPoolParams{
		Asset0:        "dash",
		Asset1:        "hbd",
		DexContractId: dashhbdDexId,
	})
	if !r.Success {
		t.Fatalf("register pool: %s: %s", r.Err, r.ErrMsg)
	}

	// Initialize the DASH/HBD pool. Asset0 is the mapped chain side
	// (dash), so it needs the dash-mapping-contract pointer.
	r = dashhbdDex.initPool(t, owner, &types.InitParams{
		Asset0:                "dash",
		Asset1:                "hbd",
		FeeBps:                100,
		Asset0MappingContract: dashMappingId,
		RouterContract:        routerContractId,
	})
	if !r.Success {
		t.Fatalf("init pool: %s: %s", r.Err, r.ErrMsg)
	}

	// Seed owner's DASH mapping-internal balance + grant the DEX pool
	// an allowance so add_liquidity can pull the deposit. Same shape
	// as the BTC version (BalancePrefix + owner = a-<owner>, etc.).
	// Pool must hold ≥ 2× swapAmount per the DEX 50% liquidity guard,
	// so seed 3 DASH (≈ 158× swap) of liquidity here.
	const liquidityDash = int64(3_00000000) // 3 DASH in duffs
	ct.StateSet(dashMappingId, dashconstants.BalancePrefix+owner, formatUintAsBytes(t, uint64(liquidityDash)))
	ct.StateSet(
		dashMappingId,
		dashconstants.AllowancePrefix+owner+dashconstants.DirPathDelimiter+"contract:"+dashhbdDexId,
		formatUintAsBytes(t, uint64(liquidityDash)),
	)
	r = dashhbdDex.addLiquidity(t, owner, uint64(liquidityDash), 100_000_000) // 3 DASH × 100 HBD
	if !r.Success {
		t.Fatalf("add liquidity: %s: %s", r.Err, r.ErrMsg)
	}

	// Seed the dash-mapping-contract state required by the map action:
	//   - supply (base fee rate)
	//   - last height + block header at that height (for tx proof)
	//   - primary + backup pubkeys (for deposit-address derivation)
	//   - router id (so MapSwap can dispatch into the router)
	ct.StateSet(dashMappingId, dashconstants.SupplyKey,
		string(dashmapping.MarshalSupply(&dashmapping.SystemSupply{BaseFeeRate: 1})))
	ct.StateSet(dashMappingId, dashconstants.LastHeightKey, strconv.Itoa(int(blockHeight)))
	ct.StateSet(dashMappingId, dashconstants.BlockPrefix+strconv.Itoa(int(blockHeight)), blockHeaderRaw)
	ct.StateSet(dashMappingId, dashconstants.PrimaryPublicKeyStateKey,
		swapTestDecodeHex(t, swapTestPrimaryPubKeyHex))
	ct.StateSet(dashMappingId, dashconstants.BackupPublicKeyStateKey,
		swapTestDecodeHex(t, swapTestBackupPubKeyHex))
	ct.StateSet(dashMappingId, dashconstants.RouterContractIdKey, routerContractId)

	// Build the map action payload.
	params := dashmapping.MapParams{
		TxData: &dashmapping.VerificationRequest{
			BlockHeight:    blockHeight,
			RawTxHex:       rawTxHex,
			MerkleProofHex: "",
			TxIndex:        0,
		},
		Instructions: []string{instruction},
	}
	payload, err := tinyjson.Marshal(params)
	if err != nil {
		t.Fatal("marshal params:", err)
	}

	// Oracle calls the map action. The dash-mapping handles the swap
	// internally: credits oracle's mapping balance, approves the DEX
	// pool, calls the router with a transfer.allow intent so the DEX
	// can pull DASH from the oracle's internal balance.
	mapResult := ct.Call(state_engine.TxVscCallContract{
		Self: state_engine.TxSelf{
			TxId:                 "oracle_dash_map_tx",
			BlockId:              "block:dash_map",
			BlockHeight:          1,
			Index:                0,
			OpIndex:              0,
			Timestamp:            "2025-10-14T00:00:00",
			RequiredAuths:        []string{owner},
			RequiredPostingAuths: []string{},
		},
		ContractId: dashMappingId,
		Action:     "map",
		Payload:    payload,
		RcLimit:    100_000,
		Intents: []contracts.Intent{
			{
				Type: "transfer.allow",
				Args: map[string]string{
					"token":       "dash",
					"limit":       strconv.FormatInt(swapAmount, 10),
					"contract_id": dashMappingId,
				},
			},
		},
		Caller: oracle,
	})

	dumpLogs(t, mapResult.Logs)
	dumpStateDiff(t, mapResult.StateDiff)

	assert.True(t, mapResult.Success,
		"map+swap should succeed: "+mapResult.Err+": "+mapResult.ErrMsg)
	hbdBalance := ct.LedgerSession.GetBalance(recipient, 1, "hbd")
	t.Logf("recipient hbd balance after swap: %d", hbdBalance)
	t.Log("return:", mapResult.Ret)
	t.Log("gas used:", mapResult.GasUsed)

	// Recipient should have received some HBD from the swap. The exact
	// amount depends on the pool's constant-product math + fee, but it
	// must be positive (a zero output would mean the router returned an
	// error or the mapping's BTC-C4 refund branch fired).
	assert.Greater(t, hbdBalance, int64(0),
		"recipient should have received HBD from the DASH→HBD swap")
}
