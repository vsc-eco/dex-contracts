# V3 DEX System - Complete Summary

## System Overview

The V3 DEX system implements a **contracts-only architecture** — no off-chain services required:

- **One router contract** managing token registration, pool registration, and swap routing
- **One DEX contract per pool** handling AMM logic, liquidity, and fees
- Dynamic chain support with no hardcoded assets
- Cross-chain settlement via `destination_chain`

## Core Architecture

### Components

1. **Router Contract** (`contracts/dex-router-v2/`)
   - Single entry point for all user DEX operations (swap, deposit, withdrawal)
   - Token registry for asset and chain tracking
   - Pool registry linking asset pairs to DEX contracts
   - Direct and two-hop swap routing (via HBD as intermediate)
   - Cross-chain settlement bridging via `destination_chain`
   - On-chain schema query (`get_schema`)

2. **DEX Pool Contracts** (`contracts/dex/`)
   - One contract deployed per asset pair
   - Constant product AMM (x*y=k) with overflow protection
   - LP token minting and burning
   - Configurable fee in basis points
   - Referral fee system
   - Pre-deposit support for router-mediated mapped-asset transfers

### How They Interact

```
User  ──►  Router Contract  ──►  DEX Pool Contract(s)  ──►  Settlement
                 │
           Token Registry (on-chain state)
           Pool Registry  (on-chain state)
```

1. User calls `execute` on the router with a swap/deposit/withdrawal instruction
2. Router looks up the appropriate pool(s) from its registry
3. For mapped assets, router uses ERC-20 allowances to transfer tokens into pools
4. For native assets, router creates protocol-level transfer intents
5. Pool executes the AMM logic and returns results
6. Router settles output — on Magi by default, or bridged to an external chain if `destination_chain` is set

## Operations

### Admin Operations (Owner-Only)

| Action | Purpose |
|--------|---------|
| `init` | Initialize the router contract and set version |
| `register_token` | Register an asset in the token registry |
| `register_pool` | Link an asset pair to a deployed DEX contract |

### User Operations (via `execute`)

| Type | Purpose |
|------|---------|
| `swap` | Exchange one asset for another (direct or two-hop via HBD) |
| `deposit` | Add liquidity to a pool and receive LP tokens |
| `withdrawal` | Burn LP tokens and receive proportional pool assets |

### Query Operations

| Action | Purpose |
|--------|---------|
| `get_pool` | Query pool reserves, fee, and LP supply |
| `get_schema` | Get list of supported chains |

See [Router Instruction Schema](./router-instruction-schema.md) for complete JSON schemas for every operation.

## Cross-Chain Settlement

### Destination Chain

The `destination_chain` field on swap instructions specifies where to settle output tokens:

```json
{
  "type": "swap",
  "version": "1.0.0",
  "asset_in": "BTC",
  "asset_out": "HBD",
  "amount_in": "50000",
  "recipient": "hiveuser123",
  "destination_chain": "HIVE"
}
```

- **Omitted or `"MAGI"`**: Output settles on the Magi chain (default).
- **`"HIVE"`**: Router bridges output to the recipient's Hive account.
- **Other chains** (e.g., `"BTC"`): Router uses the appropriate mapping contract to bridge output.

### Return Address

For two-hop swaps, a `return_address` specifies where to return funds if the second hop fails:

```json
{
  "return_address": {
    "chain": "BTC",
    "address": "bc1qxy2kgdygjrsqtzq2n0yrf2493p83kkfjhx0wlh"
  }
}
```

When the second hop fails:
1. Router checks the return address chain
2. Attempts to swap intermediate HBD back to the original asset
3. Returns funds to the return address via the appropriate bridge
4. If swap-back fails, returns intermediate HBD instead

## Asset Registration

### Token Registration (Required First)

All assets must be registered via `register_token` before pools can use them:

**Native tokens** (no mapping contract):
```json
{"name": "HBD", "chain": "HIVE"}
{"name": "HIVE", "chain": "HIVE"}
```

**Mapped assets** (require mapping contract):
```json
{"name": "BTC", "chain": "BTC", "mapping_contract": "btc-mapping-def456"}
{"name": "ETH", "chain": "ETH", "mapping_contract": "eth-mapping-ghi789"}
```

Supported chains: `HIVE`, `MAGI`, `BTC`, `ETH`, `SOL`, `SUI`, `LTC`, `DASH`, `DOGE`, `BCH`.

### Pool Registration

After tokens are registered, link asset pairs to DEX contracts:
```json
{"asset0": "BTC", "asset1": "HBD", "dex_contract_id": "dex-btc-hbd-pool-123"}
```

Asset order is normalized alphabetically. Both assets must already be in the token registry.

### Adding New Chains

1. Deploy a mapping contract for the chain (utxo-mapping pattern)
2. Register the asset via `register_token` with its chain and mapping contract
3. Deploy a DEX contract for the new pair and register the pool
4. Chain automatically appears in `get_schema` results

No code changes required — fully dynamic.

## Intent Protection

VSC provides protocol-level protection for all DEX operations via **transfer intents**:

```json
{
  "intents": [
    {
      "type": "transfer.allow",
      "args": { "limit": "1000000", "token": "HBD" }
    }
  ]
}
```

- **Swap**: Intent limits the maximum input spend
- **Deposit**: Intent limits each asset being deposited
- **Withdrawal**: Intent covers the assets being returned

Intents are atomic — either the entire operation succeeds within the declared limits, or it fails completely.

## Implementation Status

### Completed
- Router contract with token/pool registry
- Direct and two-hop swap routing
- Cross-chain settlement via `destination_chain`
- Return address handling for failed two-hop swaps
- ERC-20 allowance flow for mapped assets
- Protocol-level intent protection for native assets
- On-chain schema generation
- DEX pool contracts with constant product AMM
- LP token management with minting/burning
- Referral fee system
- Multi-chain integration (BTC, LTC, DASH, DOGE, BCH, ETH, SOL)
- Comprehensive integration tests

### Documentation
- [Router Instruction Schema](./router-instruction-schema.md) — JSON schemas for all router operations
- [DEX Instruction Schema](./dex-instruction-schema.md) — JSON schemas for individual DEX pool operations
- [Setup Guide](./setup.md) — Deployment and configuration walkthrough
- [Examples](./examples.md) — Practical usage flows for swaps and liquidity
- [Asset and Chain Registration](./asset-chain-registration.md) — Registration patterns and adding new chains
