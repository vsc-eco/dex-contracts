package tests

import (
	"fmt"
	"testing"

	dexcontracts "github.com/vsc-eco/vsc-dex-mapping"
)

func TestContractLoading(t *testing.T) {
	dex := dexcontracts.DexWasm
	if dex != nil {
		fmt.Println("successfully imported dex")
	} else {
		fmt.Println("failed to load dex")
	}
	dexRouterV1 := dexcontracts.DexRouterWasm
	if dexRouterV1 != nil {
		fmt.Println("successfully imported dex router v1")
	} else {
		fmt.Println("failed to load dex router v1")
	}
	dexRouterV2 := dexcontracts.DexRouterV2Wasm
	if dexRouterV2 != nil {
		fmt.Println("successfully imported dex router v2")
	} else {
		fmt.Println("failed to load dex router v2")
	}
}
