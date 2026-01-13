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

### 3. Pool Registration

When registering a pool, you can optionally provide chain information:

```json
{
  "asset0": "BTC",
  "asset1": "HBD",
  "dex_contract_id": "dex-btc-hbd-123",
  "asset0_chain": "BTC",  // Optional: from mapping contract
  "asset1_chain": "HIVE"  // Optional: VSC native
}
```

If chains are not provided, the system:
1. **Router Service** (during pool registration):
   - Checks if asset is already registered (from mapping contract or previous pool)
   - Queries token registry: `IsNativeAsset(asset)` → checks if `ContractId == nil`
   - If native, sets chain to "HIVE" automatically
   - Includes chain in pool registration event
2. **Contract/Indexer**:
   - Receives chain info from pool registration event
   - Stores asset → chain mapping
   - Updates schema dynamically
3. **Fallback**: Returns empty if unknown (asset needs mapping contract or token registry registration first)

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

To add new VSC native tokens:

1. **Register in Token Registry**: Use `RegisterToken()` with `ContractId = nil`
   ```go
   tokenRegistry.RegisterToken("NEW_TOKEN", 3, nil, "Description")
   ```
   - `ContractId = nil` marks it as native VSC token
   - Router service automatically recognizes via `IsNativeAsset()`

2. **Create Pool**: Register pool with new token
   ```json
   {
     "asset0": "NEW_TOKEN",
     "asset1": "HBD",
     "dex_contract_id": "dex-newtoken-hbd-123"
     // asset0_chain optional - router will auto-detect as "HIVE"
   }
   ```

3. **Automatic Recognition**: 
   - Router queries token registry during pool registration
   - `IsNativeAsset("NEW_TOKEN")` returns true (ContractId == nil)
   - Chain automatically set to "HIVE"
   - Pool registration event includes chain info
   - Schema updates automatically

**No hardcoding required** - fully dynamic based on token registry. Any new VSC native token registered with `ContractId = nil` is automatically recognized.

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
