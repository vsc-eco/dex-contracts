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
	DexWasm         []byte
	BTCMappingWasm  []byte
	LTCMappingWasm  []byte
	DASHMappingWasm []byte
	DOGEMappingWasm []byte
	BCHMappingWasm  []byte
)

func init() {
	DexRouterV2Wasm, _ = loadWasmFile("dex-router-v2.wasm")
	DexWasm, _ = loadWasmFile("dex.wasm")
	BTCMappingWasm, _ = loadWasmFile("btc-mapping.wasm")
	LTCMappingWasm, _ = loadWasmFile("ltc-mapping.wasm")
	DASHMappingWasm, _ = loadWasmFile("dash-mapping.wasm")
	DOGEMappingWasm, _ = loadWasmFile("doge-mapping.wasm")
	BCHMappingWasm, _ = loadWasmFile("bch-mapping.wasm")
}

// loadWasmFile reads a WASM file from the embedded artifacts directory
func loadWasmFile(filename string) ([]byte, error) {
	path := fmt.Sprintf("%s/%s", artifactsDir, filename)
	data, err := artifactsFS.ReadFile(path)
	if err != nil {
		fmt.Printf("warning: wasm file not found: %s\n", filename)
		return nil, err
	}
	return data, nil
}
