# DEX Setup Guide

The system has **one Router-V2 contract** for the entire DEX, and **one DEX contract per pool**. The router is deployed once and never repeated — all pools register into the same router.

---

## Overview

The setup order matters:

1. Deploy and initialize the **Router-V2** contract *(once, ever)*
2. For each new pool: deploy and initialize a **DEX** contract
3. **Register tokens** in the router (owner only — must come before pools)
4. **Register pools** in the router (owner only — links asset pairs to DEX contracts)

---

## 1. Initialize the Router (once)

Deploy the `dex-router-v2` WASM contract once, then call `init`. The payload is optional — it sets the contract version string (defaults to `"1.0.0"`).

```json
// action: init
// payload (optional):
"1.0.0"
```

The deploying account becomes the contract owner. Only the owner can call `register_token` and `register_pool`.

---

## 2. Initialize a DEX Contract (once per pool)

Deploy a `dex` WASM contract for each new pool, then call `init`. The router contract ID is the same one across all pools.

**Payload fields:**

| Field | Required | Description |
|---|---|---|
| `asset0` | yes | First asset symbol (e.g. `"HBD"`) |
| `asset1` | yes | Second asset symbol (e.g. `"HIVE"`) |
| `fee_bps` | no | Fee in basis points, default `8` (0.08%), max `10000` |
| `router_contract` | recommended | Contract ID of the Router-V2. Required for mapped-asset swaps via the router. |
| `asset0_mapping_contract` | if asset0 is mapped | Contract ID of the mapping contract for asset0 |
| `asset1_mapping_contract` | if asset1 is mapped | Contract ID of the mapping contract for asset1 |

**Native-only pool (e.g. HBD/HIVE):**

```json
// action: init
{
  "asset0": "HBD",
  "asset1": "HIVE",
  "fee_bps": 8,
  "router_contract": "dex-router-v2-abc123"
}
```

**Pool with a mapped asset (e.g. BTC/HBD):**

```json
// action: init
{
  "asset0": "BTC",
  "asset1": "HBD",
  "fee_bps": 8,
  "router_contract": "dex-router-v2-abc123",
  "asset0_mapping_contract": "btc-mapping-def456"
}
```

> The `router_contract` field authorizes the router to set `pre_deposited: true` on swap and liquidity calls. Without it, the DEX will reject routed mapped-asset transfers and the swap will fail.

Asset order within a pool is normalized alphabetically — `"BTC"/"HBD"` and `"HBD"/"BTC"` resolve to the same pool.

---

## 3. Register Tokens in the Router

Before any pool can be registered, both of its assets must be registered in the router's token registry. This is an **owner-only** operation.

**Method:** `register_token`

**Payload fields:**

| Field | Required | Description |
|---|---|---|
| `name` | yes | Asset symbol (case-insensitive) |
| `chain` | yes | Source chain. One of: `HIVE`, `MAGI`, `BTC`, `ETH`, `SOL`, `SUI`, `LTC`, `DASH`, `DOGE`, `BCH` |
| `mapping_contract` | for mapped assets | Contract ID of the mapping contract for this asset |
| `description` | no | Human-readable description |

**Native Hive tokens** (HIVE, HBD — no mapping contract):

```json
// action: register_token
{"name": "HBD", "chain": "HIVE"}
```

```json
// action: register_token
{"name": "HIVE", "chain": "HIVE"}
```

**Mapped assets** (BTC, ETH, etc. — require a mapping contract):

```json
// action: register_token
{
  "name": "BTC",
  "chain": "BTC",
  "mapping_contract": "btc-mapping-def456"
}
```

```json
// action: register_token
{
  "name": "ETH",
  "chain": "ETH",
  "mapping_contract": "eth-mapping-ghi789"
}
```

Each asset can only be registered once. Registration is permanent — if a mapping contract address changes, you need a new asset symbol.

---

## 4. Register Pools in the Router

Once both assets are registered, link the asset pair to its DEX contract.

**Method:** `register_pool`

**Payload fields:**

| Field | Required | Description |
|---|---|---|
| `asset0` | yes | First asset symbol |
| `asset1` | yes | Second asset symbol |
| `dex_contract_id` | yes | Deployed DEX contract ID for this pool |

```json
// action: register_pool
{
  "asset0": "HBD",
  "asset1": "HIVE",
  "dex_contract_id": "dex-hbd-hive-pool-xyz"
}
```

```json
// action: register_pool
{
  "asset0": "BTC",
  "asset1": "HBD",
  "dex_contract_id": "dex-btc-hbd-pool-uvw"
}
```

Asset order is normalized automatically. The router stores the mapping as `pool/hbd/hive → dex-contract-id`.

> Two-hop swaps (e.g. BTC → HIVE) route via HBD automatically. They require both a BTC/HBD pool and an HBD/HIVE pool to be registered — no extra configuration.

---

## Full Example Setup

Below is the complete call sequence for a two-pool deployment supporting BTC → HIVE swaps.

```
// 1. Init router
router.init()                          // or "1.0.0"

// 2. Init DEX contracts
dex-hbd-hive.init({
  asset0: "HBD", asset1: "HIVE",
  fee_bps: 8,
  router_contract: "dex-router-v2-abc123"
})

dex-btc-hbd.init({
  asset0: "BTC", asset1: "HBD",
  fee_bps: 30,
  router_contract: "dex-router-v2-abc123",
  asset0_mapping_contract: "btc-mapping-def456"
})

// 3. Register tokens (owner only)
router.register_token({name: "HBD",  chain: "HIVE"})
router.register_token({name: "HIVE", chain: "HIVE"})
router.register_token({name: "BTC",  chain: "BTC", mapping_contract: "btc-mapping-def456"})

// 4. Register pools (owner only)
router.register_pool({asset0: "HBD",  asset1: "HIVE", dex_contract_id: "dex-hbd-hive-pool-xyz"})
router.register_pool({asset0: "BTC",  asset1: "HBD",  dex_contract_id: "dex-btc-hbd-pool-uvw"})
```

The system is now ready to accept swaps and liquidity operations.

---

## Querying the Router

**Get pool info:**

```json
// action: get_pool
{"asset0": "BTC", "asset1": "HBD"}
```

**Get supported chains and schema info:**

```json
// action: get_schema
```

Returns the list of registered chains (used to validate `return_address.chain` in swap instructions).
