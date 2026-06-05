package tests

// review7 M8 — getMappingContract must ABORT on a corrupted token-registry
// entry instead of returning "" (which makes a mapped asset look native, so
// settleToChain would HiveWithdraw it instead of unmapping — a silent misroute).
//
// preFundAsset calls getMappingContract on the swap's input asset before any
// transfer, so registering a mapped asset (BTC), corrupting its registry entry,
// then running a BTC-input swap exercises the guard directly.
//
// A/B: post-fix the swap aborts with "corrupted token registry entry"; pre-fix
// getMappingContract returns "" (BTC looks native) and the swap fails later
// with an unrelated error.

import (
	"strings"
	"testing"

	"vsc-node/lib/test_utils"
	"vsc-node/modules/db/vsc/contracts"

	dexcontracts "github.com/vsc-eco/dex-contracts"
	"github.com/vsc-eco/dex-contracts/contracts/types"
)

func TestAuditReview7_M8_CorruptRegistryAborts(t *testing.T) {
	requireWasm(t, "btc-mapping", dexcontracts.BTCMappingWasm)
	const owner = "hive:milo-hpr"
	user := "hive:m8-user"
	btcMappingId := "vsc1BpQYDaMwcfdsh9T7DSEHZvdma1XaSXMPPj"
	dexId := "vsc1BquGPy8B766YpstdcL5cSF2GkWVVsVxJS3"
	routerId := "vsc1Bpc3SgDqCRQxzeDrvV7T4XKV6BZuHmME5F"

	ct := test_utils.NewContractTest()
	whitelistPendulum(&ct)
	t.Cleanup(func() { ct.DataLayer.Stop() })

	ct.RegisterContract(btcMappingId, owner, dexcontracts.BTCMappingWasm)
	ct.RegisterContract(dexId, owner, dexcontracts.DexWasm)
	ct.RegisterContract(routerId, owner, dexcontracts.DexRouterV2Wasm)

	router := &RouterInfo{ct: &ct, id: routerId}
	dex := &DexInfo{ct: &ct, id: dexId}

	if r := router.initRouterV2(t, owner); !r.Success {
		t.Fatalf("init router: %s", r.Ret)
	}
	if r := router.registerToken(t, owner, types.RegisterTokenParams{
		Name: "BTC", TokenInfo: types.TokenInfo{Chain: "BTC", MappingContract: btcMappingId},
	}); !r.Success {
		t.Fatalf("register BTC: %s", r.Ret)
	}
	if r := router.registerToken(t, owner, types.RegisterTokenParams{
		Name: "HBD", TokenInfo: types.TokenInfo{Chain: "HIVE"},
	}); !r.Success {
		t.Fatalf("register HBD: %s", r.Ret)
	}
	if r := router.registerPool(t, owner, types.RegisterPoolParams{
		Asset0: "btc", Asset1: "hbd", DexContractId: dexId,
	}); !r.Success {
		t.Fatalf("register pool: %s", r.Ret)
	}
	if r := dex.initPool(t, owner, &types.InitParams{
		Asset0: "btc", Asset1: "hbd", FeeBps: 100, Asset0MappingContract: btcMappingId, RouterContract: routerId,
	}); !r.Success {
		t.Fatalf("init pool: %s: %s", r.Err, r.ErrMsg)
	}

	// Sanity: a valid registry entry exists after register_token.
	if ct.StateGet(routerId, "asset"+types.DirPathDelimiter+"btc") == "" {
		t.Fatal("expected a registry entry for btc after register_token")
	}

	// Corrupt the BTC registry entry.
	ct.StateSet(routerId, "asset"+types.DirPathDelimiter+"btc", "{ not valid token info json")

	r := router.execute(t, user, &types.DexInstruction{
		Type: "swap", Version: "1.0.0",
		AssetIn: "btc", AssetOut: "hbd", AmountIn: "10000", Recipient: user,
	}, []contracts.Intent{
		{Type: "transfer.allow", Args: map[string]string{"token": "btc", "limit": "10000"}},
	})

	if r.Success {
		t.Fatal("M8: a swap whose mapped asset has a corrupted registry entry must abort, not succeed")
	}
	if combined := r.ErrMsg + r.Ret; !strings.Contains(combined, "corrupted token registry entry") {
		t.Fatalf("M8: expected the corrupt-registry abort, got err=%q ret=%q", r.ErrMsg, r.Ret)
	}
}
