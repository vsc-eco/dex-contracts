package tests

import (
	"testing"

	dexcontracts "github.com/vsc-eco/vsc-dex-mapping"

	"github.com/stretchr/testify/assert"
)

func TestContractLoading(t *testing.T) {
	assert.NotNil(t, dexcontracts.DexWasm, "dex wasm should load")
	assert.NotNil(t, dexcontracts.DexRouterWasm, "dex router v1 wasm should load")
	assert.NotNil(t, dexcontracts.DexRouterV2Wasm, "dex router v2 wasm should load")
	assert.NotNil(t, dexcontracts.BtcMappingWasm, "btc-mapping wasm should load")
}
