# Indexer Service API Documentation

The VSC DEX Indexer provides REST API endpoints for querying real-time DEX data, transaction history, and liquidity analytics.

## Base URL
```
http://localhost:8081/api/v1
```

## Endpoints

### Pool Endpoints

#### Get All Pools
```http
GET /api/v1/pools
```

Returns a list of all indexed liquidity pools.

**Response:**
```json
[
  {
    "id": "1",
    "asset0": "HBD",
    "asset1": "HIVE",
    "reserve0": 1000000,
    "reserve1": 500000,
    "fee": 8,
    "total_supply": 1000000
  }
]
```

#### Get Specific Pool
```http
GET /api/v1/pools/{poolId}
```

Returns detailed information for a specific pool.

**Parameters:**
- `poolId` (string): Pool identifier

**Response:**
```json
{
  "id": "1",
  "asset0": "HBD",
  "asset1": "HIVE",
  "reserve0": 1000000,
  "reserve1": 500000,
  "fee": 8,
  "total_supply": 1000000
}
```

#### Get Pool Liquidity Accounts
```http
GET /api/v1/pools/{poolId}/accounts
```

Returns all accounts that hold liquidity in the specified pool.

**Parameters:**
- `poolId` (string): Pool identifier

**Response:**
```json
{
  "pool_id": "1",
  "accounts": [
    {
      "user": "alice",
      "pool_id": "1",
      "amount": 500000,
      "share": 50.0
    },
    {
      "user": "bob",
      "pool_id": "1",
      "amount": 500000,
      "share": 50.0
    }
  ]
}
```

#### Get Pool Rich List
```http
GET /api/v1/pools/{poolId}/richlist?offset=0&limit=50
```

Returns a paginated list of top liquidity holders for the pool, sorted by LP token amount.

**Parameters:**
- `poolId` (string): Pool identifier
- `offset` (integer, optional): Pagination offset (default: 0)
- `limit` (integer, optional): Maximum results per page (default: 50, max: 100)

**Response:**
```json
{
  "pool_id": "1",
  "offset": 0,
  "limit": 50,
  "holders": [
    {
      "user": "alice",
      "pool_id": "1",
      "amount": 500000,
      "share": 50.0
    },
    {
      "user": "bob",
      "pool_id": "1",
      "amount": 500000,
      "share": 50.0
    }
  ]
}
```

### Transaction Endpoints

#### Get Transaction History
```http
GET /api/v1/transactions?pool_id=1&type=swap&limit=100
```

Returns transaction history with optional filtering.

**Query Parameters:**
- `pool_id` (string, optional): Filter by pool ID
- `type` (string, optional): Filter by transaction type (`swap`, `deposit`, `withdrawal`, `pool_created`)
- `limit` (integer, optional): Maximum transactions to return (default: 100, max: 1000)

**Response:**
```json
{
  "transactions": [
    {
      "id": "tx-12345",
      "type": "swap",
      "pool_id": "1",
      "user": "alice",
      "block_height": 101756761,
      "timestamp": "2024-12-04T15:30:00Z",
      "details": {
        "amount_in": 10000,
        "amount_out": 25000,
        "asset_in": "HBD",
        "asset_out": "HIVE"
      }
    }
  ],
  "count": 1
}
```

#### Get Specific Transaction
```http
GET /api/v1/transactions/{txId}
```

Returns detailed information for a specific transaction.

**Parameters:**
- `txId` (string): Transaction identifier

**Response:**
```json
{
  "id": "tx-12345",
  "type": "swap",
  "pool_id": "1",
  "user": "alice",
  "block_height": 101756761,
  "timestamp": "2024-12-04T15:30:00Z",
  "details": {
    "amount_in": 10000,
    "amount_out": 25000,
    "asset_in": "HBD",
    "asset_out": "HIVE"
  }
}
```

### Schema Endpoint

#### Get Instruction Schema
```http
GET /api/v1/schema
```

Returns the current instruction schema dynamically generated from registered DEX pools. The schema includes all supported chains for `return_address` based on assets registered in pools.

**Response:**
```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "required": ["type", "version", "asset_in", "asset_out", "recipient"],
  "properties": {
    "type": {
      "type": "string",
      "enum": ["swap", "deposit", "withdrawal"]
    },
    "version": {
      "type": "string",
      "pattern": "^\\d+\\.\\d+\\.\\d+$"
    },
    "asset_in": {
      "type": "string"
    },
    "asset_out": {
      "type": "string"
    },
    "recipient": {
      "type": "string"
    },
    "return_address": {
      "type": "object",
      "properties": {
        "chain": {
          "type": "string",
          "enum": ["BTC", "ETH", "SOL", "HIVE", "SUI"]
        },
        "address": {
          "type": "string"
        }
      },
      "required": ["chain", "address"]
    }
  },
  "registered_assets": ["BTC", "ETH", "HBD", "HIVE"],
  "supported_chains": ["BTC", "ETH", "SOL", "HIVE", "SUI"],
  "note": "This schema is dynamically generated based on registered DEX pools. Chains are automatically added when pools are registered."
}
```

**Notes:**
- The `return_address.chain` enum is dynamically generated from registered pools
- Chains are automatically added when pools are registered
- Chains come from mapping contracts (utxo-mapping pattern) or pool registration
- Only chains that have registered pools appear in the enum
- The schema updates in real-time as pools are registered
- **Future-proof**: New chains (SUI, etc.) work automatically when mapping contracts are deployed and pools are registered

### Health Check

#### Service Health
```http
GET /health
```

Returns service health status.

**Response:**
```json
{
  "status": "healthy",
  "service": "dex-indexer"
}
```

## Data Types

### PoolInfo
```typescript
interface PoolInfo {
  id: string;          // Pool identifier
  asset0: string;      // First asset symbol (e.g., "HBD")
  asset1: string;      // Second asset symbol (e.g., "HIVE")
  reserve0: number;    // Reserve amount of asset0
  reserve1: number;    // Reserve amount of asset1
  fee: number;         // Fee in basis points (e.g., 8 = 0.08%)
  total_supply: number; // Total LP tokens minted
}
```

### LiquidityPosition
```typescript
interface LiquidityPosition {
  user: string;     // Account name
  pool_id: string;  // Pool identifier
  amount: number;   // LP tokens held
  share: number;    // Percentage share of pool (0-100)
}
```

### TransactionInfo
```typescript
interface TransactionInfo {
  id: string;              // Transaction identifier
  type: string;            // "swap" | "deposit" | "withdrawal" | "pool_created"
  pool_id: string;         // Associated pool ID
  user?: string;           // Account that initiated the transaction
  block_height: number;    // VSC block height
  timestamp: string;       // ISO 8601 timestamp
  details: object;         // Transaction-specific data
}
```

## Error Responses

All endpoints return standard HTTP status codes:

- `200` - Success
- `400` - Bad Request (invalid parameters)
- `404` - Not Found (pool/transaction doesn't exist)
- `500` - Internal Server Error

Error response format:
```json
{
  "error": "Description of the error"
}
```

## Rate Limiting

- No explicit rate limiting implemented
- Consider implementing client-side request throttling for production use

## Real-time Updates

The indexer polls the VSC GraphQL API every 5 seconds for new data. For real-time updates, consider implementing WebSocket subscriptions in your client application.

## Examples

### Get pool liquidity distribution
```bash
curl http://localhost:8081/api/v1/pools/1/accounts
```

### Monitor recent swaps
```bash
curl "http://localhost:8081/api/v1/transactions?type=swap&limit=10"
```

### Check transaction status
```bash
curl http://localhost:8081/api/v1/transactions/tx-12345
```

### Get top holders for analytics
```bash
curl "http://localhost:8081/api/v1/pools/1/richlist?limit=20"
```







