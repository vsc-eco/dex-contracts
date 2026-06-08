package tests

// review8 DX-H4 — settleToChain received a `chain` argument but never used it:
// the settlement destination was derived solely from the output asset. So a swap
// asking to settle to chain X for an asset that lives on chain Y silently
// delivered on Y. Proven: an HBD->HIVE swap with destination_chain "BTC" and a
// valid Hive recipient SUCCEEDED and HiveWithdrew HIVE to that Hive account, the
// "BTC" request ignored. The fix validates the requested chain against the
// asset's registered chain and aborts on a mismatch.

import (
	"testing"

	"vsc-node/lib/test_utils"
	"vsc-node/modules/db/vsc/contracts"
	ledger_db "vsc-node/modules/db/vsc/ledger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dexcontracts "github.com/vsc-eco/dex-contracts"
	"github.com/vsc-eco/dex-contracts/contracts/types"
)

func setupRouterNativePool(t *testing.T) (*test_utils.ContractTest, *RouterInfo) {
	t.Helper()
	ct := test_utils.NewContractTest()
	whitelistPendulum(&ct)
	t.Cleanup(func() { ct.DataLayer.Stop() })

	owner := "hive:milo-hpr"
	dexId := "vsc1Bjn53csDr6wUoYsjXiN9Nhadu458Tw9wvR"
	routerId := "vsc1Bpc3SgDqCRQxzeDrvV7T4XKV6BZuHmME5F"
	ct.RegisterContract(dexId, owner, dexcontracts.DexWasm)
	ct.RegisterContract(routerId, owner, dexcontracts.DexRouterV2Wasm)

	router := &RouterInfo{ct: &ct, id: routerId}
	dex := &DexInfo{ct: &ct, id: dexId}

	require.True(t, router.registerToken(t, owner, types.RegisterTokenParams{
		Name: "HIVE", TokenInfo: types.TokenInfo{Chain: "HIVE"}}).Success)
	require.True(t, router.registerToken(t, owner, types.RegisterTokenParams{
		Name: "HBD", TokenInfo: types.TokenInfo{Chain: "HIVE"}}).Success)
	require.True(t, router.registerPool(t, owner, types.RegisterPoolParams{
		Asset0: "hive", Asset1: "hbd", DexContractId: dexId}).Success)

	dex.initPool(t, owner, &types.InitParams{
		Asset0: "hive", Asset1: "hbd", FeeBps: 100, RouterContract: routerId,
	})
	dex.addLiquidity(t, owner, 1_000_000, 1_000_000)
	return &ct, router
}

func TestReview8_DXH4_SettleRejectsMismatchedChain(t *testing.T) {
	ct, router := setupRouterNativePool(t)
	owner := "hive:milo-hpr"
	ct.Deposit(owner, 50_000, ledger_db.Asset("hbd"))

	// HBD->HIVE swap, but ask to settle the HIVE output on BTC. HIVE is registered
	// as a HIVE-chain asset, so this is a misrouting request and must abort.
	r := router.execute(t, owner, &types.DexInstruction{
		Type:             "swap",
		Version:          "1.0.0",
		AssetIn:          "hbd",
		AssetOut:         "hive",
		AmountIn:         "500",
		Recipient:        "hive:milo-vsc",
		DestinationChain: "BTC",
	}, []contracts.Intent{
		{Type: "transfer.allow", Args: map[string]string{"token": "hbd", "limit": "500"}},
	})
	assert.False(t, r.Success,
		"DX-H4: settling a HIVE-chain asset to destination_chain BTC must abort, not silently HiveWithdraw")
}

func TestReview8_DXH4_SettleAcceptsMatchingChain(t *testing.T) {
	ct, router := setupRouterNativePool(t)
	owner := "hive:milo-hpr"
	ct.Deposit(owner, 50_000, ledger_db.Asset("hbd"))

	// Same swap, but settle to the asset's actual chain (HIVE) — must succeed and
	// HiveWithdraw to the recipient.
	r := router.execute(t, owner, &types.DexInstruction{
		Type:             "swap",
		Version:          "1.0.0",
		AssetIn:          "hbd",
		AssetOut:         "hive",
		AmountIn:         "500",
		Recipient:        "hive:milo-vsc",
		DestinationChain: "HIVE",
	}, []contracts.Intent{
		{Type: "transfer.allow", Args: map[string]string{"token": "hbd", "limit": "500"}},
	})
	assert.True(t, r.Success, "matching destination_chain HIVE must succeed: %s: %s", r.Err, r.ErrMsg)
}
