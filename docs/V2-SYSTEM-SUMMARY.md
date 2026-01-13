# V2 DEX System - Complete Summary

## System Overview

The V2 DEX system implements a **fully dynamic, future-proof architecture** with:
- Chain-aware return addresses
- Dynamic schema generation
- No hardcoded chains or assets
- Support for any future chain (SUI, etc.) and VSC native tokens

## Core Architecture

### Components

1. **Router-V2 Contract** (`contracts/dex-router-v2/`)
   - Routes swaps between DEX contracts
   - Supports direct and two-hop swaps
   - Tracks assets and chains for schema generation
   - Handles return addresses with chain awareness

2. **Indexer Service** (`services/indexer/`)
   - Tracks pools, assets, and chains
   - Generates dynamic instruction schema
   - Provides `/api/v1/schema` endpoint
   - Updates schema automatically as pools are registered

3. **Token Registry** (in go-vsc-node)
   - Stores asset metadata
   - `ContractId == nil` → native VSC token (chain = "HIVE")
   - Router queries registry during pool registration

## Return Address System

### Structure (REQUIRED)
```json
{
  "return_address": {
    "chain": "BTC",
    "address": "bc1qxy2kgdygjrsqtzq2n0yrf2493p83kkfjhx0wlh"
  }
}
```

### Key Points
- ✅ **Always an object** with `chain` and `address` fields
- ✅ **Chain is required** - prevents cross-chain errors
- ✅ **Chain enum is dynamic** - generated from registered pools
- ✅ **Respects return address** in failure cases

### Two-Hop Swap Failure Handling

When a two-hop swap fails at the second hop:
1. System checks return address chain
2. If return address chain matches original asset chain:
   - Attempts to swap HBD back to original asset
   - Returns original asset to return address
3. If swap-back fails or chain doesn't match:
   - Returns intermediate HBD to return address
   - Bridge system handles cross-chain transfer if needed

## Dynamic Schema Generation

### How It Works

1. **Pool Registration**:
   ```json
   {
     "asset0": "BTC",
     "asset1": "HBD",
     "dex_contract_id": "dex-btc-hbd-123",
     "asset0_chain": "BTC",  // Optional
     "asset1_chain": "HIVE"  // Optional
   }
   ```

2. **Chain Determination**:
   - If chain provided → use it
   - If not provided → router queries token registry
   - If native asset (`ContractId == nil`) → chain = "HIVE"
   - If unknown → requires mapping contract registration

3. **Schema Update**:
   - Indexer tracks assets and chains
   - Schema API returns current enum values
   - No code changes needed for new chains

### Query Current Schema

```bash
curl http://localhost:8081/api/v1/schema
```

Returns:
```json
{
  "properties": {
    "return_address": {
      "properties": {
        "chain": {
          "type": "string",
          "enum": ["BTC", "ETH", "SOL", "HIVE", "SUI"]  // Dynamic!
        }
      }
    }
  },
  "supported_chains": ["BTC", "ETH", "SOL", "HIVE", "SUI"],
  "registered_assets": ["BTC", "ETH", "HBD", "HIVE"]
}
```

## Asset Registration

### Cross-Chain Assets (utxo-mapping pattern)
1. Deploy mapping contract (e.g., BTC, ETH, SOL, SUI)
2. Mapping contract registers asset with chain
3. Create pool with that asset
4. Chain appears in schema automatically

### VSC Native Tokens
1. Register in token registry: `RegisterToken("NEW_TOKEN", 3, nil, "Description")`
2. `ContractId = nil` marks as native
3. Router detects via `IsNativeAsset()` → chain = "HIVE"
4. Schema updates automatically

## Documentation Structure

### Core Docs
- `instruction-schema.md` - Complete schema specification
- `asset-chain-registration.md` - Registration flow and patterns
- `indexer-api.md` - API documentation including schema endpoint
- `architecture-v2.md` - System architecture
- `README.md` - Quick reference

### Verification
- `SCHEMA-CONSISTENCY.md` - Consistency checklist
- `test-coverage-summary.md` - Test coverage report

## Implementation Status

### ✅ Completed
- Dynamic schema generation
- Chain-aware return addresses
- Asset and chain tracking
- Schema API endpoint
- Two-hop swap failure handling
- Return address respect in failures
- Future-proof design (no hardcoding)
- Comprehensive tests

### Test Coverage
- Indexer: 52.3% coverage
- Router: 41.4% coverage
- Schemas: 76.6% coverage
- All tests passing ✅

## Usage Examples

### Get Current Schema
```bash
curl http://localhost:8081/api/v1/schema
```

### Register Pool with Chains
```json
{
  "action": "register_pool",
  "payload": "{\"asset0\":\"BTC\",\"asset1\":\"HBD\",\"dex_contract_id\":\"dex-btc-hbd-123\",\"asset0_chain\":\"BTC\",\"asset1_chain\":\"HIVE\"}"
}
```

### Swap with Return Address
```json
{
  "type": "swap",
  "version": "1.0.0",
  "asset_in": "BTC",
  "asset_out": "HIVE",
  "recipient": "user123",
  "return_address": {
    "chain": "BTC",
    "address": "bc1qxy2kgdygjrsqtzq2n0yrf2493p83kkfjhx0wlh"
  }
}
```

## Future-Proof Guarantees

1. **New Chains**: Work automatically when mapping contracts deployed
2. **New Native Tokens**: Work automatically when registered in token registry
3. **Schema Updates**: Happen automatically, no code changes
4. **Return Address Safety**: Always includes chain, prevents errors
