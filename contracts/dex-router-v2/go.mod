module dex-router-v2

go 1.24.0

toolchain go1.24.1

// replace vsc-node => github.com/vsc-eco/go-vsc-node v0.0.0-20251120092146-ea108c70b7f0
replace vsc-node => ../../../milo-go-vsc-node

replace github.com/agl/ed25519 => github.com/binance-chain/edwards25519 v0.0.0-20200305024217-f36fc4b53d43

require github.com/CosmWasm/tinyjson v0.9.0

require github.com/josharian/intern v1.0.0 // indirect
