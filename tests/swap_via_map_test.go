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

	"btc-mapping-contract/contract/constants"
	btcconstants "btc-mapping-contract/contract/constants"
	"btc-mapping-contract/contract/mapping"
)

// Well-known secp256k1 test vectors (same keys used by btc-mapping-contract tests).
const (
	swapTestPrimaryPubKeyHex = "0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798"
	swapTestBackupPubKeyHex  = "02c6047f9441ed7d6d3045406e95c07cd85c778e4b8cef3ca7abac09b95c709ee5"
)

func swapTestRegtestParams() *chaincfg.Params {
	return &chaincfg.RegressionNetParams
}

func swapTestDecodeHex(t *testing.T, s string) string {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal("decodeHex:", err)
	}
	return string(b)
}

// swapTestBuildBlockHeader mines a regtest block header whose hash satisfies
// the compact target 0x207fffff (regtest difficulty floor).
func swapTestBuildBlockHeader(prevBlock, merkleRoot chainhash.Hash, ts time.Time) *wire.BlockHeader {
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

// buildSwapMapFixture derives the deposit address for the given swap instruction,
// creates a tx paying amount sats to that address, then wraps it in a single-tx
// block (MerkleRoot = TxHash, empty proof, TxIndex = 0).
// Returns rawTxHex (for MapParams) and blockHeaderRaw (raw 80 bytes, for state seeding).
func buildSwapMapFixture(t *testing.T, instruction string, amount int64) (rawTxHex string, blockHeaderRaw string) {
	t.Helper()
	network := swapTestRegtestParams()

	address, _, err := mapping.DepositAddress(swapTestPrimaryPubKeyHex, swapTestBackupPubKeyHex, instruction, network)
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
	header := swapTestBuildBlockHeader(chainhash.Hash{}, txHash, time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))

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

// TestSwapViaMap exercises the full map → router → DEX swap path:
//  1. A Bitcoin tx pays to the deposit address derived from a swap_to instruction.
//  2. The oracle calls the BTC mapping contract's "map" action.
//  3. The mapping contract credits the oracle's BTC balance, then calls the DEX
//     router, which swaps BTC for HBD and delivers HBD to the recipient.
func TestSwapViaMap(t *testing.T) {
	const blockHeight = uint32(100)
	const swapAmount = int64(10_000_000) // 0.1 BTC in satoshis
	const recipient = "hive:milo-vsc"
	const oracle = "did:vsc:oracle:btc"
	const owner = "hive:milo-hpr"

	instruction := "swap_to=" + recipient + "&swap_asset_out=hbd"
	// instruction := constants.SwapToKey + "=" + recipient + "&" + constants.SwapAssetOut + "=" + "hbd"
	rawTxHex, blockHeaderRaw := buildSwapMapFixture(t, instruction, swapAmount)

	ct := test_utils.NewContractTest()
	t.Cleanup(func() { ct.DataLayer.Stop() })

	btcMappingId := "vsc1BpQYDaMwcfdsh9T7DSEHZvdma1XaSXMPPj"
	btchbdDexId := "vsc1BquGPy8B766YpstdcL5cSF2GkWVVsVxJS3"
	routerContractId := "vsc1Bpc3SgDqCRQxzeDrvV7T4XKV6BZuHmME5F"

	ct.RegisterContract(btcMappingId, owner, dexcontracts.BTCMappingWasm)
	ct.RegisterContract(btchbdDexId, owner, dexcontracts.DexWasm)
	ct.RegisterContract(routerContractId, owner, dexcontracts.DexRouterV2Wasm)

	router := &RouterInfo{ct: &ct, id: routerContractId}
	btchbdDex := &DexInfo{ct: &ct, id: btchbdDexId}

	// Register tokens in the router
	r := router.registerToken(t, owner, types.RegisterTokenParams{
		Name:      "BTC",
		TokenInfo: types.TokenInfo{Chain: "BTC", MappingContract: btcMappingId},
	})
	if !r.Success {
		t.Fatalf("register BTC token: %s", r.Ret)
	}
	r = router.registerToken(t, owner, types.RegisterTokenParams{
		Name:      "HBD",
		TokenInfo: types.TokenInfo{Chain: "HIVE"},
	})
	if !r.Success {
		t.Fatalf("register HBD token: %s", r.Ret)
	}

	// Register the BTC/HBD pool
	r = router.registerPool(t, owner, types.RegisterPoolParams{
		Asset0:        "btc",
		Asset1:        "hbd",
		DexContractId: btchbdDexId,
	})
	if !r.Success {
		t.Fatalf("register pool: %s", r.Ret)
	}

	// Initialize BTC/HBD DEX pool with BTC mapping contract as the mapped asset
	r = btchbdDex.initPool(t, owner, &types.InitParams{
		Asset0:                "btc",
		Asset1:                "hbd",
		FeeBps:                100,
		Asset0MappingContract: btcMappingId,
		RouterContract:        routerContractId,
	})
	if !r.Success {
		t.Fatalf("init pool: %s: %s", r.Err, r.ErrMsg)
	}

	// Seed owner's BTC mapping balance and approve DEX pool, then add liquidity
	ct.StateSet(btcMappingId, constants.BalancePrefix+owner, formatUintAsBytes(t, 2_00000000))
	ct.StateSet(btcMappingId, constants.AllowancePrefix+owner+constants.DirPathDelimiter+"contract:"+btchbdDexId, formatUintAsBytes(t, 2_00000000))
	r = btchbdDex.addLiquidity(t, owner, 1_49000000, 100000_000)
	if !r.Success {
		t.Fatalf("add liquidity: %s: %s", r.Err, r.ErrMsg)
	}

	// Seed BTC mapping contract state: supply, block headers, public keys, router ID
	ct.StateSet(btcMappingId, btcconstants.SupplyKey,
		string(mapping.MarshalSupply(&mapping.SystemSupply{BaseFeeRate: 1})))
	ct.StateSet(btcMappingId, constants.LastHeightKey, strconv.Itoa(int(blockHeight)))
	ct.StateSet(btcMappingId, btcconstants.BlockPrefix+strconv.Itoa(int(blockHeight)), blockHeaderRaw)
	ct.StateSet(btcMappingId, btcconstants.PrimaryPublicKeyStateKey,
		swapTestDecodeHex(t, swapTestPrimaryPubKeyHex))
	ct.StateSet(btcMappingId, btcconstants.BackupPublicKeyStateKey,
		swapTestDecodeHex(t, swapTestBackupPubKeyHex))
	ct.StateSet(btcMappingId, btcconstants.RouterContractIdKey, routerContractId)

	// Build the map action payload
	params := mapping.MapParams{
		TxData: &mapping.VerificationRequest{
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

	// Oracle calls the map action; the BTC mapping contract handles the swap internally:
	// it credits the oracle's BTC balance, then calls the router with a transfer.allow
	// intent so the DEX can pull BTC from the oracle's mapping balance.
	mapResult := ct.Call(state_engine.TxVscCallContract{
		Self: state_engine.TxSelf{
			TxId:                 "oracle_map_tx",
			BlockId:              "block:map",
			BlockHeight:          1,
			Index:                0,
			OpIndex:              0,
			Timestamp:            "2025-10-14T00:00:00",
			RequiredAuths:        []string{owner},
			RequiredPostingAuths: []string{},
		},
		ContractId: btcMappingId,
		Action:     "map",
		Payload:    payload,
		RcLimit:    100000,
		Intents: []contracts.Intent{
			{
				Type: "transfer.allow",
				Args: map[string]string{
					"token":       "btc",
					"limit":       "10000000",
					"contract_id": btcMappingId,
				},
			},
		},
		Caller: oracle,
	})

	dumpLogs(t, mapResult.Logs)
	dumpStateDiff(t, mapResult.StateDiff)

	assert.True(t, mapResult.Success, "map action should succeed: "+mapResult.Err+": "+mapResult.ErrMsg)
	t.Log("hbd balance:", ct.LedgerSession.GetBalance(recipient, 1, "hbd"))
	t.Log("return:", mapResult.Ret)
	t.Log("gas used:", mapResult.GasUsed)
}
