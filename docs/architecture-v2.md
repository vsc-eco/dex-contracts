# VSC DEX Architecture V2

## Overview

The VSC DEX system has been refactored to separate DEX contracts from the router. This new architecture provides better modularity, scalability, and allows for independent pool management.

## Architecture Changes

### Previous Architecture (V1)
- **Unified Router Contract**: Single contract (`dex-router`) managed all pools internally
- **Namespaced State**: Pools stored with keys like `pool/{poolId}/...`
- **Single Contract Indexing**: Indexer tracked one contract

### New Architecture (V2)
- **Separate DEX Contracts**: Each pool has its own DEX contract (`dex`)
- **Router-V2 Contract**: Routes between DEX contracts (`dex-router-v2`)
- **Multi-Contract Indexing**: Indexer tracks router and all DEX contracts

## Components

### 1. DEX Contract (`contracts/dex/`)
A standalone contract that manages a single liquidity pool.

**Methods:**
- `init` - Initialize pool with asset pair and fee
- `swap` - Execute swaps within the pool
- `add_liquidity` - Add liquidity to the pool
- `remove_liquidity` - Remove liquidity from the pool
- `get_pool` - Query pool information
- `claim_fees` - Claim accumulated fees (system only)

**State:**
- Single pool reserves (reserve0, reserve1)
- LP token balances per user
- Fee accumulators

### 2. Router-V2 Contract (`contracts/dex-router-v2/`)
Routes swaps between DEX contracts, supporting direct and two-hop swaps.

**Methods:**
- `init` - Initialize router
- `register_pool` - Register a DEX contract for an asset pair (with optional chain info)
- `execute` - Execute DEX operations (swap, deposit, withdrawal)
- `get_pool` - Query pool via router
- `get_schema` - Query current schema information (supported chains)

**State:**
- Pool registry: `pool/{asset0}/{asset1}` -> `dex_contract_id`

**Routing Logic:**
1. **Direct Swap**: If a pool exists for the asset pair, route directly
2. **Two-Hop Swap**: If no direct pool, route via HBD (e.g., BTC -> HBD -> HIVE)

**Note**: Two-hop swap implementation needs completion - currently requires pool querying to calculate intermediate amounts.

### 3. Indexer Service (`services/indexer/`)
Updated to track multiple contracts:
- Router-V2 contract (for pool registrations)
- Individual DEX contracts (for pool events)

**Event Handling:**
- `register_pool` - Creates pool mapping from router, tracks assets and chains
- `swap`, `add_liquidity`, `remove_liquidity` - Updates pool state from DEX contracts

**Read Models:**
- Maintains mapping: `dex_contract_id` -> `pool_id`
- Tracks pools, transactions, and liquidity positions
- Tracks assets and chains for dynamic schema generation
- Generates dynamic instruction schema with chain-aware return addresses

**API Endpoints:**
- `GET /api/v1/schema` - Returns current instruction schema with dynamic `return_address.chain` enum

### 4. Router Service (`services/router/`)
Updated to work with router-v2:
- Includes `amount_in` in swap payloads (required by router-v2)
- Calls router-v2 contract instead of unified router

## Deployment Workflow

1. **Deploy DEX Contracts**: Deploy one contract per pool
   ```bash
   # Deploy HBD/HIVE pool
   deploy dex-hbd-hive.wasm
   
   # Deploy BTC/HBD pool
   deploy dex-btc-hbd.wasm
   ```

2. **Initialize DEX Contracts**: Initialize each with asset pair
   ```json
   {
     "action": "init",
     "payload": "{\"asset0\": \"HBD\", \"asset1\": \"HIVE\", \"fee_bps\": 8}"
   }
   ```

3. **Deploy Router-V2**: Deploy the router contract
   ```bash
   deploy dex-router-v2.wasm
   ```

4. **Register Pools**: Register each DEX contract with router
   ```json
   {
     "action": "register_pool",
     "payload": "{\"asset0\": \"HBD\", \"asset1\": \"HIVE\", \"dex_contract_id\": \"dex-hbd-hive-123\"}"
   }
   ```

5. **Start Indexer**: Monitor router and all DEX contracts
   ```bash
   go run services/indexer/cmd/main.go \
     --contracts "dex-router-v2,dex-hbd-hive-123,dex-btc-hbd-456"
   ```

## Migration Notes

### From V1 to V2

1. **Pool Data**: Existing pools need to be migrated to separate DEX contracts
2. **Router Calls**: Update all router calls to use `dex-router-v2` instead of `dex-router`
3. **Indexer Configuration**: Add all DEX contract IDs to indexer monitoring
4. **SDK Updates**: SDK already compatible (uses generic interface)

### Backward Compatibility

The indexer maintains backward compatibility:
- Handles events from old `dex-router` contract
- Handles events from new `dex-router-v2` and individual DEX contracts
- Supports both architectures during migration

## Benefits

1. **Modularity**: Each pool is independent
2. **Scalability**: Can deploy pools independently
3. **Upgradeability**: Can upgrade individual pools without affecting others
4. **Isolation**: Pool issues don't affect other pools
5. **Flexibility**: Different pools can have different configurations

## Key Features

### Dynamic Schema Generation
- Instruction schema updates automatically as pools are registered
- `return_address.chain` enum is dynamically generated from registered chains
- No hardcoded chain assumptions - fully future-proof
- Query `GET /api/v1/schema` for current schema

### Chain-Aware Return Addresses
- `return_address` always includes `chain` identifier
- Prevents cross-chain return errors
- Supports CEX hot wallets on any chain
- Two-hop swap failures respect return address chain requirements

### Asset and Chain Registration
- Chains determined from mapping contracts (utxo-mapping pattern)
- VSC native tokens recognized via token registry (`ContractId == nil`)
- Pool registration can include explicit chain info
- Fully dynamic - new chains work automatically

## TODO / Future Improvements

1. ✅ **Two-Hop Swap**: Implemented with proper failure handling and return address respect
2. **Pool Discovery**: Add automatic pool discovery mechanism
3. **Multi-Hop Routing**: Support more complex routing paths (not just via HBD)
4. ✅ **Pool Registry Events**: Implemented - tracks assets and chains
5. ✅ **Schema Query**: Implemented - `get_schema` contract method and `/api/v1/schema` API endpoint

## Contract Structure

```
contracts/
├── dex/              # Individual DEX pool contract
│   ├── main.go       # Pool operations (swap, add/remove liquidity)
│   ├── types.go      # Contract types
│   └── utils.go      # State management utilities
└── dex-router-v2/    # Router contract
    ├── main.go       # Routing logic
    ├── types.go      # Router types
    └── utils.go      # Router utilities
```

## Service Configuration

### Router Service
```bash
go run services/router/cmd/main.go \
  --vsc-node http://localhost:4000 \
  --port 8080 \
  --indexer-endpoint http://localhost:8081 \
  --dex-router-contract dex-router-v2-123
```

### Indexer Service
```bash
go run services/indexer/cmd/main.go \
  --http-endpoint http://localhost:4000 \
  --http-port 8081 \
  --contracts "dex-router-v2-123,dex-hbd-hive-456,dex-btc-hbd-789"
```
