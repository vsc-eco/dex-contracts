module github.com/vsc-eco/dex-contracts

go 1.24.0

toolchain go1.25.10

// replace vsc-node => github.com/vsc-eco/go-vsc-node v0.0.0-20260526171655-39d38f4a66ad
replace vsc-node => github.com/vsc-eco/go-vsc-node v0.0.0-20260526171655-39d38f4a66ad

replace github.com/agl/ed25519 => github.com/binance-chain/edwards25519 v0.0.0-20200305024217-f36fc4b53d43

require github.com/CosmWasm/tinyjson v0.9.0

require github.com/josharian/intern v1.0.0 // indirect
