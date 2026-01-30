# VSC DEX Documentation

## Core Documentation

### [Instruction Schema](./instruction-schema.md)
Complete specification for DEX instructions, including dynamic schema generation based on registered pools.

### [Asset and Chain Registration](./asset-chain-registration.md)
How assets and chains are registered dynamically, following the utxo-mapping contract pattern. **Future-proof design** - no hardcoded chains.

### [Indexer API](./indexer-api.md)
REST API documentation for querying DEX data, including the dynamic schema endpoint.

### [Architecture v2](./architecture-v2.md)
System architecture overview and component descriptions.

### [V2 System Summary](./V2-SYSTEM-SUMMARY.md)
Complete summary of the V2 system with all features and implementation status.

### [Schema Consistency](./SCHEMA-CONSISTENCY.md)
Verification checklist ensuring all documentation is consistent with chain-aware return addresses.

### [Docs to Tests Mapping](./docs-to-tests-mapping.md)
Maps published features and user journeys from the docs to their test coverage. Use to verify all documented functionality is tested.

## Key Features

### Dynamic Schema Generation
- Schema updates automatically as pools are registered
- Chains determined from mapping contracts (utxo-mapping pattern)
- No hardcoded chain assumptions
- Future-proof for any new chain (SUI, etc.)

### Return Address Safety
- Always includes chain identifier
- Prevents cross-chain return errors
- Supports CEX hot wallets on any chain
- Respects return address requirements in failure cases

### Two-Hop Swap Handling
- Proper failure handling with return address respect
- Attempts to swap back to original asset when appropriate
- Handles cross-chain returns via bridge system

## Quick Reference

### Get Current Schema
```bash
curl http://localhost:8081/api/v1/schema
```

### Register Pool (with chain info)
```json
{
  "asset0": "BTC",
  "asset1": "HBD",
  "dex_contract_id": "dex-btc-hbd-123",
  "asset0_chain": "BTC",  // Optional: from mapping contract
  "asset1_chain": "HIVE"  // Optional: VSC native
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

## Adding New Chains

1. Deploy mapping contract (utxo-mapping pattern)
2. Register asset via mapping contract
3. Create pool with that asset
4. Chain automatically appears in schema

**No code changes required** - fully dynamic.

## Adding New VSC Native Tokens

1. Register in token registry with `ContractId = nil`
2. Create pool with new token
3. Router automatically detects as native (chain = "HIVE")
4. Schema updates automatically

**No hardcoding required** - fully dynamic based on token registry.
