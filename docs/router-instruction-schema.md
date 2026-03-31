# Router Instruction Schema

The instruction schema uses snake_case field names and follows JSON Schema Draft 2020-12.

---

## Admin Operations

### 1. `init` — Initialize Router

Initializes the router contract and sets its version. The deploying account becomes the contract owner. Only the owner can call `register_token` and `register_pool`.

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "string",
  "description": "Contract version in semver format. Defaults to '1.0.0' if omitted.",
  "pattern": "^\\d+\\.\\d+\\.\\d+$",
  "default": "1.0.0"
}
```

#### Field Descriptions

- **payload** (string, optional): Version string in semver format (e.g., `"1.0.0"`). Defaults to `"1.0.0"` if not provided.

---

### 2. `register_token` — Register Token

Registers a new token in the router's token registry. Both assets in a pool must be registered before the pool can be created. Owner-only operation.

When a `mapping_contract` is provided, the router calls `getInfo` on it and validates that the returned `symbol` matches the provided `name`. The `decimals` value is read from the `getInfo` response and stored — if the mapping contract does not return a `decimals` field, registration fails. For native assets (no mapping contract), `decimals` comes from the payload.

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$ref": "#/$defs/RegisterTokenParams",
  "$defs": {
    "RegisterTokenParams": {
      "type": "object",
      "required": ["name", "chain"],
      "properties": {
        "name": { "type": "string" },
        "chain": {
          "type": "string",
          "enum": [
            "HIVE",
            "MAGI",
            "BTC",
            "ETH",
            "SOL",
            "SUI",
            "LTC",
            "DASH",
            "DOGE",
            "BCH"
          ]
        },
        "mapping_contract": { "type": "string" },
        "decimals": { "type": "integer", "minimum": 0 },
        "description": { "type": "string" }
      }
    }
  }
}
```

#### Field Descriptions

**Required Fields**

- **`name`** (string): Asset symbol (e.g., `"BTC"`, `"HBD"`). Case-insensitive; stored normalized. Must be unique across the registry. For mapped assets, must match the `symbol` returned by the mapping contract's `getInfo`.
- **`chain`** (string): Source chain for this asset. One of: `HIVE`, `MAGI`, `BTC`, `ETH`, `SOL`, `SUI`, `LTC`, `DASH`, `DOGE`, `BCH`.

**Optional Fields**

- **`mapping_contract`** (string): Contract ID of the mapping contract for cross-chain assets. Required for non-native assets (e.g., BTC, ETH). Must be a valid contract ID. The router calls `getInfo` on this contract to validate the symbol and read decimals.
- **`decimals`** (integer): Number of decimal places for the asset's smallest unit. For mapped assets this is read from the mapping contract's `getInfo` response (the payload value is ignored). For native assets (no mapping contract), this value is used directly.
- **`description`** (string): Human-readable description of the token.

---

### 3. `register_pool` — Register Pool

Links an asset pair to a deployed DEX contract. Both assets must already be registered via `register_token`. Owner-only operation.

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$ref": "#/$defs/RegisterPoolParams",
  "$defs": {
    "RegisterPoolParams": {
      "type": "object",
      "required": ["asset0", "asset1", "dex_contract_id"],
      "properties": {
        "asset0": { "type": "string" },
        "asset1": { "type": "string" },
        "dex_contract_id": { "type": "string" }
      }
    }
  }
}
```

#### Field Descriptions

**Required Fields**

- **`asset0`** (string): First asset symbol. Normalized alphabetically with `asset1` for storage.
- **`asset1`** (string): Second asset symbol. Must differ from `asset0`.
- **`dex_contract_id`** (string): Contract ID of the deployed DEX pool contract for this pair. Must be a valid contract ID.

---

## User Operations

All user operations go through the router's `execute` action. The `type` field determines the operation.

### 4. `execute` (swap) — Swap Assets

Exchanges an input asset for an output asset. The router finds the appropriate pool(s) and routes the swap — either directly or via a two-hop path through HBD. Supports optional slippage protection, referral fees, cross-chain return addresses, and external-chain settlement.

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$ref": "#/$defs/SwapInstruction",
  "$defs": {
    "SwapInstruction": {
      "type": "object",
      "required": [
        "type",
        "version",
        "asset_in",
        "amount_in",
        "asset_out",
        "recipient"
      ],
      "properties": {
        "type": { "type": "string", "const": "swap" },
        "version": { "type": "string", "pattern": "^\\d+\\.\\d+\\.\\d+$" },
        "asset_in": { "type": "string" },
        "amount_in": { "type": "string" },
        "asset_out": { "type": "string" },
        "recipient": { "type": "string" },
        "min_amount_out": { "type": ["string", "null"] },
        "beneficiary": { "type": ["string", "null"] },
        "ref_bps": { "type": ["integer", "null"] },
        "return_address": {
          "type": ["object", "null"],
          "properties": {
            "chain": { "type": "string" },
            "address": { "type": "string" }
          },
          "required": ["chain", "address"]
        },
        "destination_chain": { "type": "string" },
        "metadata": {
          "type": "object",
          "additionalProperties": { "type": "string" }
        }
      }
    }
  }
}
```

#### Field Descriptions

**Required Fields**

- **`type`** (string): Must be `"swap"`.
- **`version`** (string): Schema version in semver format (e.g., `"1.0.0"`).
- **`asset_in`** (string): Symbol of the asset being sold (e.g., `"BTC"`, `"HBD"`).
- **`amount_in`** (string): Amount of `asset_in` to swap, in the asset's smallest unit.
- **`asset_out`** (string): Symbol of the asset to receive (e.g., `"HIVE"`, `"HBD"`).
- **`recipient`** (string): Account address that will receive the output tokens.

**Optional Fields**

- **`min_amount_out`** (string | null): Minimum acceptable output amount in the smallest unit of `asset_out`. Acts as slippage protection — the swap reverts if the output would be less. Omit for no minimum.
- **`beneficiary`** (string | null): Account to receive the referral fee portion. Required if `ref_bps` is set.
- **`ref_bps`** (integer | null): Referral fee in basis points deducted from the output amount. Omit if no referral fee applies.
- **`return_address`** (object | null): Where to return funds if a two-hop swap fails at the second hop.
  - **`chain`** (string): Chain for the return address (e.g., `"BTC"`, `"ETH"`).
  - **`address`** (string): Address on the specified chain.
- **`destination_chain`** (string): External chain for settlement (e.g., `"HIVE"`, `"BTC"`). When set to a non-MAGI chain, the router bridges the swap output to the `recipient` address on that chain instead of settling on Magi. Omit to settle on Magi.
- **`metadata`** (object): Additional string key-value pairs for extensibility (e.g., `{"min_intermediate": "48000"}` for a floor on HBD received between hops in a two-hop swap).

---

### 5. `execute` (deposit) — Add Liquidity

Deposits both assets into a pool in exchange for LP tokens. The `asset0` and `asset1` fields identify the pool using its canonical alphabetical ordering.

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$ref": "#/$defs/DepositInstruction",
  "$defs": {
    "DepositInstruction": {
      "type": "object",
      "required": ["type", "version", "asset0", "asset1", "amount0", "amount1", "recipient"],
      "properties": {
        "type": { "type": "string", "const": "deposit" },
        "version": { "type": "string", "pattern": "^\\d+\\.\\d+\\.\\d+$" },
        "asset0": { "type": "string" },
        "asset1": { "type": "string" },
        "amount0": { "type": "string" },
        "amount1": { "type": "string" },
        "recipient": { "type": "string" }
      }
    }
  }
}
```

#### Field Descriptions

**Required Fields**

- **`type`** (string): Must be `"deposit"`.
- **`version`** (string): Schema version in semver format (e.g., `"1.0.0"`).
- **`asset0`** (string): First asset symbol of the pool. The router normalizes to lowercase and enforces lexicographic ordering (`asset0 < asset1`), swapping `asset0`/`asset1` and `amount0`/`amount1` if needed. Callers may pass assets in any order.
- **`asset1`** (string): Second asset symbol of the pool.
- **`amount0`** (string): Amount of `asset0` to deposit, in its smallest unit.
- **`amount1`** (string): Amount of `asset1` to deposit, in its smallest unit.
- **`recipient`** (string): Account address that will receive the minted LP tokens.

---

### 6. `execute` (withdrawal) — Remove Liquidity

Burns LP tokens to withdraw a proportional share of both assets from a pool. The caller must be the LP owner (i.e., `recipient` must match the caller's address).

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$ref": "#/$defs/WithdrawalInstruction",
  "$defs": {
    "WithdrawalInstruction": {
      "type": "object",
      "required": ["type", "version", "asset0", "asset1", "lp_amount", "recipient"],
      "properties": {
        "type": { "type": "string", "const": "withdrawal" },
        "version": { "type": "string", "pattern": "^\\d+\\.\\d+\\.\\d+$" },
        "asset0": { "type": "string" },
        "asset1": { "type": "string" },
        "lp_amount": { "type": "string" },
        "recipient": { "type": "string" }
      }
    }
  }
}
```

#### Field Descriptions

**Required Fields**

- **`type`** (string): Must be `"withdrawal"`.
- **`version`** (string): Schema version in semver format (e.g., `"1.0.0"`).
- **`asset0`** (string): First asset symbol of the pool. The router normalizes to lowercase and enforces lexicographic ordering (`asset0 < asset1`). Callers may pass assets in any order.
- **`asset1`** (string): Second asset symbol of the pool.
- **`lp_amount`** (string): Number of LP tokens to burn. The pool returns a proportional share of both assets based on this amount relative to the total LP supply.
- **`recipient`** (string): Account address that will receive the withdrawn assets. Must match the caller's address (only the LP owner can withdraw).

---

## Query Operations

### 7. `get_pool` — Get Pool Info

Queries pool information for an asset pair. Returns reserves, fee configuration, and total LP supply.

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$ref": "#/$defs/GetPoolParams",
  "$defs": {
    "GetPoolParams": {
      "type": "object",
      "required": ["asset0", "asset1"],
      "properties": {
        "asset0": { "type": "string" },
        "asset1": { "type": "string" }
      }
    }
  }
}
```

#### Field Descriptions

**Required Fields**

- **`asset0`** (string): First asset symbol.
- **`asset1`** (string): Second asset symbol.

#### Response

```json
{
  "asset0": "HBD",
  "asset1": "HIVE",
  "reserve0": "1000000",
  "reserve1": "500000",
  "fee": 8,
  "total_lp": "707106"
}
```

---

### 8. `get_token` — Get Token Info

Queries a single registered token by name. Returns its chain, decimals, and mapping contract (if any).

#### Payload

Token name as a plain string (e.g., `"BTC"`, `"HBD"`). Case-insensitive.

#### Response

```json
{
  "chain": "BTC",
  "mapping_contract": "vsc1BkWohDf5fPcwn7V9B9ar6TyiWc3A2ZGJ4t",
  "decimals": 8
}
```

For a native token:

```json
{
  "chain": "HIVE",
  "decimals": 3
}
```

---

### 9. `get_schema` — Get Schema

Returns all supported chains and every registered token with its chain, decimals, and mapping contract. No payload required.

#### Response

```json
{
  "supported_chains": ["BTC", "HIVE"],
  "return_address_chains": ["BTC", "HIVE"],
  "tokens": [
    {
      "name": "btc",
      "chain": "BTC",
      "mapping_contract": "vsc1BkWohDf5fPcwn7V9B9ar6TyiWc3A2ZGJ4t",
      "decimals": 8
    },
    { "name": "hbd", "chain": "HIVE", "decimals": 3 },
    { "name": "hive", "chain": "HIVE", "decimals": 3 }
  ],
  "note": "Schema dynamically generated from registered tokens and pools."
}
```

#### Field Descriptions

- **`supported_chains`** (string[]): All chains that have at least one registered token.
- **`return_address_chains`** (string[]): Valid values for `return_address.chain` in swap instructions.
- **`tokens`** (object[]): Every registered token, each containing:
  - **`name`** (string): Asset symbol (lowercase).
  - **`chain`** (string): Source chain.
  - **`decimals`** (integer): Number of decimal places for the asset's smallest unit.
  - **`mapping_contract`** (string, optional): Contract ID of the mapping contract for cross-chain assets.

---

## Notes

- All string amounts are in the **smallest unit** of their respective asset (no decimal representation).
- Fields typed as `["string", "null"]` or `["integer", "null"]` correspond to Go pointer types. A JSON `null` value is equivalent to the field being omitted.
- `omitempty` fields in Go are excluded from `required` — they will not be present in serialized output when zero or nil.
- **Lexicographic asset ordering:** Both pools and the router enforce `asset0 < asset1` (lowercase, lexicographic). The pool normalizes at `init`; the router normalizes deposit/withdrawal inputs automatically. Swap instructions use directional `asset_in`/`asset_out` and are unaffected by ordering — the pool resolves direction internally.
- **Instruction field naming by type:**
  - `swap` — uses `asset_in`, `asset_out`, `amount_in` (directional).
  - `deposit` — uses `asset0`, `asset1`, `amount0`, `amount1` (pool-ordered).
  - `withdrawal` — uses `asset0`, `asset1`, `lp_amount` (pool-ordered).
- Two-hop swaps (e.g., BTC → HIVE) route through HBD automatically when no direct pool exists. Both a BTC/HBD pool and an HBD/HIVE pool must be registered.
- When `destination_chain` is set, the router bridges swap output to the recipient address on that external chain instead of settling on Magi.
- `claim_fees` is owner-only — only the contract owner can trigger fee claims.
