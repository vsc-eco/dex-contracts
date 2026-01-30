# Asset and Chain Registration

## Overview

The DEX system uses a **dynamic, future-proof approach** for asset and chain registration. Chains are not hardcoded - they are determined from actual registrations following the utxo-mapping contract pattern.

## Registration Flow

### 1. Cross-Chain Assets (via Mapping Contracts)

Cross-chain assets follow the **utxo-mapping pattern**:

1. **Mapping Contract Deployment**: Deploy a mapping contract for the chain (e.g., BTC, ETH, SOL, SUI)
2. **Asset Registration**: The mapping contract registers the asset and its chain
3. **Pool Registration**: When a pool is created with that asset, the chain is automatically tracked

**Example**: 
- Deploy BTC mapping contract → BTC asset registered with chain "BTC"
- Register BTC/HBD pool → "BTC" appears in schema's `return_address.chain` enum

### 2. VSC Native Assets

VSC native assets are determined by checking the **token registry**:
- If `ContractId == nil` in token registry → asset is native VSC token
- Chain: "HIVE" for all native VSC tokens
- No mapping contract required
- **Future-proof**: New VSC native tokens work automatically when registered in token registry

**How it works:**
1. **Token Registry**: Stores all assets with `ContractId` field
   - `ContractId == nil` → native VSC token (chain = "HIVE")
   - `ContractId != nil` → cross-chain or contract-based token
2. **Router Service** (has token registry access):
   - When pool is registered, router checks token registry
   - If `IsNativeAsset(asset)` returns true → chain = "HIVE"
   - Router can include chain info in pool registration event
3. **Contract/Indexer**:
   - Can receive chain info from pool registration payload
   - Or query token registry if accessible
   - **Future-proof**: New native tokens automatically recognized via token registry

### 3. Token Registration (Required First)

**Tokens MUST be registered before any pool can use them.** Call `register_token` first:

```json
{"symbol": "HBD", "chain": "HIVE"}
{"symbol": "HIVE", "chain": "HIVE"}
{"symbol": "BTC", "chain": "BTC"}
```

Supported chains: **HIVE** (for HIVE/HBD native), **MAGI** (for MAGI native tokens), **BTC**, **ETH**, **SOL**, **SUI**.

### 4. Pool Registration

After tokens are registered, register the pool:

```json
{
  "asset0": "BTC",
  "asset1": "HBD",
  "dex_contract_id": "dex-btc-hbd-123"
}
```

Pool registration will **reject** if either asset is not registered. Chain info is stored when tokens are registered via `register_token`.

## Implementation Details

### Registry Contract

**File**: `contracts/dex-router-v2/main.go`

- `RegisterPool()` accepts optional `asset0_chain` and `asset1_chain`
- Stores asset → chain mappings
- Updates supported chains list dynamically

### Indexer

**File**: `services/indexer/readmodels.go`

- Tracks assets and chains from pool registrations
- Generates dynamic schema with current chains
- Only includes chains that have registered pools

### Schema API

**Endpoint**: `GET /api/v1/schema`

Returns current schema with:
- Dynamic `return_address.chain` enum (only registered chains)
- List of registered assets
- List of supported chains

## Future-Proof Design

### Adding New Chains

To add a new chain (e.g., SUI):

1. **Deploy Mapping Contract**: Follow utxo-mapping pattern
2. **Register Asset**: Mapping contract registers SUI asset with chain "SUI"
3. **Create Pool**: Register a pool with SUI (e.g., SUI/HBD)
4. **Automatic Schema Update**: "SUI" appears in `return_address.chain` enum

**No code changes required** - fully dynamic.

### Adding VSC Native Tokens

To add new VSC native tokens (HIVE chain or MAGI chain):

1. **Register via `register_token`**:
   ```json
   {"symbol": "NEW_TOKEN", "chain": "HIVE"}
   ```
   For MAGI native tokens:
   ```json
   {"symbol": "MAGI_TOKEN", "chain": "MAGI"}
   ```

2. **Create Pool**: Register pool with new token (both assets must be registered first)
   ```json
   {
     "asset0": "NEW_TOKEN",
     "asset1": "HBD",
     "dex_contract_id": "dex-newtoken-hbd-123"
   }
   ```

**Enforcement**: Pools cannot use unregistered tokens. Call `register_token` before `register_pool`.

## Best Practices

1. **Always provide chain info** in pool registration if available
2. **Register mapping contracts first** before creating pools with cross-chain assets
3. **Query schema API** to get current supported chains
4. **Use return_address.chain** to ensure correct cross-chain returns

## Return Address Safety

The `return_address` field **always includes a chain identifier**:

```json
{
  "return_address": {
    "chain": "BTC",
    "address": "bc1qxy2kgdygjrsqtzq2n0yrf2493p83kkfjhx0wlh"
  }
}
```

This ensures:
- ✅ Returns go to the correct blockchain
- ✅ Prevents cross-chain errors
- ✅ Supports CEX hot wallets on any chain
- ✅ Works with any future chain (SUI, etc.)
