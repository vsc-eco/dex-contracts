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

// TestSwapViaMapToHive exercises the cross-chain settlement path:
// BTC is swapped for HBD, and the HBD is withdrawn to a Hive mainnet account
// via HiveWithdraw instead of settling on Magi.
//
// The instruction includes destination_chain=HIVE, which tells the router
// to bridge the swap output to Hive mainnet.
func TestSwapViaMapToHive(t *testing.T) {
	requireWasm(t, "btc-mapping", dexcontracts.BTCMappingWasm)
	const blockHeight = uint32(100)
	const swapAmount = int64(10_000_000) // 0.1 BTC in satoshis
	const recipient = "hive:milo-vsc"    // Hive mainnet destination
	const oracle = "did:vsc:oracle:btc"
	const owner = "hive:milo-hpr"

	instruction := "swap_to=" + recipient + "&swap_asset_out=hbd&destination_chain=HIVE"
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

	r = router.registerPool(t, owner, types.RegisterPoolParams{
		Asset0:        "btc",
		Asset1:        "hbd",
		DexContractId: btchbdDexId,
	})
	if !r.Success {
		t.Fatalf("register pool: %s", r.Ret)
	}

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

	// Seed liquidity
	ct.StateSet(btcMappingId, constants.BalancePrefix+owner, formatUintAsBytes(t, 2_00000000))
	ct.StateSet(
		btcMappingId,
		constants.AllowancePrefix+owner+constants.DirPathDelimiter+"contract:"+btchbdDexId,
		formatUintAsBytes(t, 2_00000000),
	)
	r = btchbdDex.addLiquidity(t, owner, 1_49000000, 100000_000)
	if !r.Success {
		t.Fatalf("add liquidity: %s: %s", r.Err, r.ErrMsg)
	}

	// Seed BTC mapping contract state
	ct.StateSet(btcMappingId, btcconstants.SupplyKey,
		string(mapping.MarshalSupply(&mapping.SystemSupply{BaseFeeRate: 1})))
	ct.StateSet(btcMappingId, constants.LastHeightKey, strconv.Itoa(int(blockHeight)))
	ct.StateSet(btcMappingId, btcconstants.BlockPrefix+strconv.Itoa(int(blockHeight)), blockHeaderRaw)
	ct.StateSet(btcMappingId, btcconstants.PrimaryPublicKeyStateKey,
		swapTestDecodeHex(t, swapTestPrimaryPubKeyHex))
	ct.StateSet(btcMappingId, btcconstants.BackupPublicKeyStateKey,
		swapTestDecodeHex(t, swapTestBackupPubKeyHex))
	ct.StateSet(btcMappingId, btcconstants.RouterContractIdKey, routerContractId)

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

	mapResult := ct.Call(state_engine.TxVscCallContract{
		Self: state_engine.TxSelf{
			TxId:                 "oracle_map_tx_hive",
			BlockId:              "block:map:hive",
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

	assert.True(t, mapResult.Success, "map+swap+withdraw should succeed: "+mapResult.Err+": "+mapResult.ErrMsg)
	t.Log("return:", mapResult.Ret)
	t.Log("gas used:", mapResult.GasUsed)

	// With destination_chain=HIVE, the HBD is withdrawn to Hive mainnet
	// via HiveWithdraw — so the Magi ledger balance should be 0
	// (the output went to the external chain, not Magi).
	magiBalance := ct.LedgerSession.GetBalance(recipient, 1, "hbd")
	t.Log("recipient magi hbd balance (should be 0):", magiBalance)
	assert.Equal(t, int64(0), magiBalance,
		"HBD should NOT land on Magi — it was withdrawn to Hive mainnet")
}

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
	requireWasm(t, "btc-mapping", dexcontracts.BTCMappingWasm)
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
	ct.StateSet(
		btcMappingId,
		constants.AllowancePrefix+owner+constants.DirPathDelimiter+"contract:"+btchbdDexId,
		formatUintAsBytes(t, 2_00000000),
	)
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

// swapTestBuildSeedHeaderRaw creates a minimal regtest block header for seeding
// into the BTC mapping contract state (the "block-100" key).
func swapTestBuildSeedHeaderRaw(t *testing.T) string {
	t.Helper()
	header := swapTestBuildBlockHeader(chainhash.Hash{}, chainhash.Hash{}, time.Unix(0, 0))
	var buf bytes.Buffer
	if err := header.Serialize(&buf); err != nil {
		t.Fatal("serialize seed header:", err)
	}
	return buf.String()
}

// swapTestChangeUtxoBinary builds a binary-encoded Utxo for a change output
// (nil tag) usable as seed state for the BTC mapping contract.
func swapTestChangeUtxoBinary(t *testing.T, txId string, vout uint32, amount int64) string {
	t.Helper()
	address, _, err := mapping.AddressWithBackup(
		swapTestPrimaryPubKeyHex, swapTestBackupPubKeyHex, nil, swapTestRegtestParams())
	if err != nil {
		t.Fatal("derive change address:", err)
	}
	addr, err := btcutil.DecodeAddress(address, swapTestRegtestParams())
	if err != nil {
		t.Fatal("decode change address:", err)
	}
	pkScript, err := txscript.PayToAddrScript(addr)
	if err != nil {
		t.Fatal("pkscript:", err)
	}
	utxo := mapping.Utxo{
		TxId: txId, Vout: vout, Amount: amount, PkScript: pkScript, Tag: nil,
	}
	return string(mapping.MarshalUtxo(&utxo))
}

// swapTestRegtestDestAddress returns a regtest P2WSH address that the
// BTC mapping contract's unmap action can pay to.
func swapTestRegtestDestAddress(t *testing.T) string {
	t.Helper()
	address, _, err := mapping.AddressWithBackup(
		swapTestPrimaryPubKeyHex, swapTestBackupPubKeyHex, nil, swapTestRegtestParams())
	if err != nil {
		t.Fatal("derive regtest dest address:", err)
	}
	return address
}

// TestHiveMainnetToBtcMainnet exercises the reverse cross-chain path:
// HIVE from Hive mainnet → swap to BTC → unmap BTC to Bitcoin mainnet.
//
// The user calls the router with HIVE as input and BTC as output with
// destination_chain=BTC. The router executes a two-hop swap
// (HIVE → HBD → BTC), then calls "unmap" on the BTC mapping contract
// to send the BTC to a Bitcoin address.
func TestHiveMainnetToBtcMainnet(t *testing.T) {
	requireWasm(t, "btc-mapping", dexcontracts.BTCMappingWasm)

	const owner = "hive:milo-hpr"
	btcMappingId := "vsc1BpQYDaMwcfdsh9T7DSEHZvdma1XaSXMPPj"
	btchbdDexId := "vsc1BquGPy8B766YpstdcL5cSF2GkWVVsVxJS3"
	hivehbdDexId := "vsc1Bjn53csDr6wUoYsjXiN9Nhadu458Tw9wvR"
	routerId := "vsc1Bpc3SgDqCRQxzeDrvV7T4XKV6BZuHmME5F"

	btcDestAddress := swapTestRegtestDestAddress(t)

	ct := test_utils.NewContractTest()
	t.Cleanup(func() { ct.DataLayer.Stop() })

	ct.RegisterContract(btcMappingId, owner, dexcontracts.BTCMappingWasm)
	ct.RegisterContract(btchbdDexId, owner, dexcontracts.DexWasm)
	ct.RegisterContract(hivehbdDexId, owner, dexcontracts.DexWasm)
	ct.RegisterContract(routerId, owner, dexcontracts.DexRouterV2Wasm)

	router := &RouterInfo{ct: &ct, id: routerId}
	btchbdDex := &DexInfo{ct: &ct, id: btchbdDexId}
	hivehbdDex := &DexInfo{ct: &ct, id: hivehbdDexId}

	// --- Register tokens ---
	r := router.registerToken(t, owner, types.RegisterTokenParams{
		Name:      "BTC",
		TokenInfo: types.TokenInfo{Chain: "BTC", MappingContract: btcMappingId},
	})
	if !r.Success {
		t.Fatalf("register BTC: %s", r.Ret)
	}
	r = router.registerToken(t, owner, types.RegisterTokenParams{
		Name:      "HBD",
		TokenInfo: types.TokenInfo{Chain: "HIVE"},
	})
	if !r.Success {
		t.Fatalf("register HBD: %s", r.Ret)
	}
	r = router.registerToken(t, owner, types.RegisterTokenParams{
		Name:      "HIVE",
		TokenInfo: types.TokenInfo{Chain: "HIVE"},
	})
	if !r.Success {
		t.Fatalf("register HIVE: %s", r.Ret)
	}

	// --- Register pools ---
	r = router.registerPool(t, owner, types.RegisterPoolParams{
		Asset0: "btc", Asset1: "hbd", DexContractId: btchbdDexId,
	})
	if !r.Success {
		t.Fatalf("register btc/hbd pool: %s", r.Ret)
	}
	r = router.registerPool(t, owner, types.RegisterPoolParams{
		Asset0: "hive", Asset1: "hbd", DexContractId: hivehbdDexId,
	})
	if !r.Success {
		t.Fatalf("register hive/hbd pool: %s", r.Ret)
	}

	// --- Initialize BTC/HBD pool ---
	r = btchbdDex.initPool(t, owner, &types.InitParams{
		Asset0:                "btc",
		Asset1:                "hbd",
		FeeBps:                100,
		Asset0MappingContract: btcMappingId,
		RouterContract:        routerId,
	})
	if !r.Success {
		t.Fatalf("init btc/hbd pool: %s: %s", r.Err, r.ErrMsg)
	}

	// Seed BTC liquidity via mapping contract state
	ct.StateSet(btcMappingId, constants.BalancePrefix+owner, formatUintAsBytes(t, 2_00000000))
	ct.StateSet(btcMappingId,
		constants.AllowancePrefix+owner+constants.DirPathDelimiter+"contract:"+btchbdDexId,
		formatUintAsBytes(t, 2_00000000))
	r = btchbdDex.addLiquidity(t, owner, 1_00000000, 100000_000)
	if !r.Success {
		t.Fatalf("add btc/hbd liquidity: %s: %s", r.Err, r.ErrMsg)
	}

	// --- Initialize HIVE/HBD pool ---
	r = hivehbdDex.initPool(t, owner, &types.InitParams{
		Asset0:         "hbd",
		Asset1:         "hive",
		FeeBps:         100,
		RouterContract: routerId,
	})
	if !r.Success {
		t.Fatalf("init hive/hbd pool: %s: %s", r.Err, r.ErrMsg)
	}
	r = hivehbdDex.addLiquidity(t, owner, 100000_000, 100000_000)
	if !r.Success {
		t.Fatalf("add hive/hbd liquidity: %s: %s", r.Err, r.ErrMsg)
	}

	// --- Seed BTC mapping contract state for unmap ---
	// The unmap action needs UTXOs to create a Bitcoin spend transaction.
	const fakeTxId0 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const fakeTxId1 = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	ct.StateSet(btcMappingId, constants.ObservedPrefix+fakeTxId0+":0", "1")
	ct.StateSet(btcMappingId, constants.ObservedPrefix+fakeTxId1+":0", "1")
	// Two confirmed UTXOs (IDs 64, 65) totaling 1 BTC
	ct.StateSet(btcMappingId, btcconstants.UtxoRegistryKey,
		string(mapping.MarshalUtxoRegistry(mapping.UtxoRegistry{
			{Id: 64, Amount: 50000000},
			{Id: 65, Amount: 50000000},
		})))
	ct.StateSet(btcMappingId, btcconstants.UtxoPrefix+"40",
		swapTestChangeUtxoBinary(t, fakeTxId0, 0, 50000000))
	ct.StateSet(btcMappingId, btcconstants.UtxoPrefix+"41",
		swapTestChangeUtxoBinary(t, fakeTxId1, 0, 50000000))
	ct.StateSet(btcMappingId, btcconstants.UtxoLastIdKey, string([]byte{66, 0}))
	ct.StateSet(btcMappingId, btcconstants.SupplyKey,
		string(mapping.MarshalSupply(&mapping.SystemSupply{
			ActiveSupply: 1_00000000,
			UserSupply:   1_00000000,
			BaseFeeRate:  1,
		})))
	ct.StateSet(btcMappingId, constants.LastHeightKey, "100")
	ct.StateSet(btcMappingId, btcconstants.BlockPrefix+"100",
		swapTestBuildSeedHeaderRaw(t))
	ct.StateSet(btcMappingId, btcconstants.PrimaryPublicKeyStateKey,
		swapTestDecodeHex(t, swapTestPrimaryPubKeyHex))
	ct.StateSet(btcMappingId, btcconstants.BackupPublicKeyStateKey,
		swapTestDecodeHex(t, swapTestBackupPubKeyHex))
	ct.StateSet(btcMappingId, btcconstants.RouterContractIdKey, routerId)

	// --- Fund user with HIVE and execute swap ---
	swapAmount := "5000000" // 5M HIVE units
	ct.Deposit(owner, 10000000, "hive")

	r = router.execute(t, owner, &types.DexInstruction{
		Type:             "swap",
		Version:          "1.0.0",
		AssetIn:          "hive",
		AssetOut:         "btc",
		AmountIn:         swapAmount,
		Recipient:        btcDestAddress,
		DestinationChain: "BTC",
	}, []contracts.Intent{
		{
			Type: "transfer.allow",
			Args: map[string]string{
				"token": "hive",
				"limit": swapAmount,
			},
		},
	})

	dumpLogs(t, r.Logs)
	dumpStateDiff(t, r.StateDiff)

	assert.True(t, r.Success,
		"HIVE→BTC swap with unmap should succeed: "+r.Err+": "+r.ErrMsg)
	t.Log("return:", r.Ret)
	t.Log("gas used:", r.GasUsed)
}
