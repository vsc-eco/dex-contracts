# init-service

CLI tool that bootstraps a VSC DEX deployment: initializes the router contract, registers tokens, and creates + registers liquidity pools.

## Call sequence

Each call is broadcast as a Hive custom-json transaction, then the service polls the GraphQL endpoint until the tx is confirmed (or times out).

> Steps 1-2 are skipped when `--pools-only` is set.

1. **`router` &rarr; `init`** with payload `"1.0.0"`
2. For each unique token across all pools:
   - **`router` &rarr; `register_token`** with `{ name, chain, mapping_contract }`
3. For each pool:
   1. **`pool` &rarr; `init`** with `{ asset0, asset1, fee_bps, asset0_mapping_contract, asset1_mapping_contract, router_contract }`
   2. **`router` &rarr; `register_pool`** with `{ asset0, asset1, dex_contract_id }`

## Usage

```
go build -o init-service .
./init-service [-data-dir data] [-pools-only]
```

| Flag | Default | Description |
|------|---------|-------------|
| `-data-dir` | `data` | Directory containing `config/` with all config files |
| `-pools-only` | `false` | Skip router init and token registration, only init + register pools |

## Configuration

All config files live under `<data-dir>/config/`. On startup, any missing config
files are created as empty templates (one pool with all fields blank) and the
service exits so you can fill them in before re-running.

| File | Purpose |
|------|---------|
| `identityConfig.json` | Hive username and active key (matches go-vsc-node format) |
| `hiveConfig.json` | Hive API node URIs |
| `serviceConfig.json` | GraphQL URL, VSC net ID, chain ID, poll timing |
| `poolsConfig.json` | Router contract ID and pool definitions (assets, fees, mapping contracts) |
