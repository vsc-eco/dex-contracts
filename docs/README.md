# VSC DEX Documentation

## Core Documentation

### [Setup](./setup.md)
How to deploy and configure the router and DEX contracts, including token and pool registration.

### [Examples](./examples.md)
Example flows for swapping and providing liquidity, for both native and mapped tokens.

### [Router Instruction Schema](./router-instruction-schema.md)
Complete specification for DEX router instructions, including dynamic schema generation based on registered pools.

### [DEX Instruction Schema](./dex-instruction-schema.md)
Schema for individual DEX pool operations (init, swap, add_liquidity, remove_liquidity).

### [Asset and Chain Registration](./asset-chain-registration.md)
How assets and chains are registered dynamically, following the utxo-mapping contract pattern. **Future-proof design** - no hardcoded chains.

### [V2 System Summary](./V2-SYSTEM-SUMMARY.md)
Complete summary of the V2 system with all features and implementation status.

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
