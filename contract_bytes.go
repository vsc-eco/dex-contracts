package dexcontracts

import (
	"embed"
	"fmt"
)

//go:embed bin
var artifactsFS embed.FS

const artifactsDir = "bin"

// Pre-loaded byte arrays (nil if file doesn't exist at package init)
var (
	DexRouterV2Wasm []byte
	DexRouterWasm   []byte
	DexWasm         []byte
	BTCMappingWasm  []byte
)

func init() {
	DexRouterV2Wasm, _ = loadWasmFile("dex-router-v2.wasm")
	DexRouterWasm, _ = loadWasmFile("dex-router.wasm")
	DexWasm, _ = loadWasmFile("dex.wasm")
	BTCMappingWasm, _ = loadWasmFile("btc-mapping.wasm")
}

// loadWasmFile reads a WASM file from the embedded artifacts directory
func loadWasmFile(filename string) ([]byte, error) {
	path := fmt.Sprintf("%s/%s", artifactsDir, filename)
	data, err := artifactsFS.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("wasm file not found: %s", filename)
	}
	return data, nil
}
