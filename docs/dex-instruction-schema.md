# AMM Instruction Schema

The instruction schema uses snake_case field names and follows JSON Schema Draft 2020-12.

---

## Operations

### 1. `init` — Initialize Pool

Creates a new AMM pool with two assets and a fee configuration. Optionally, mapping contracts can be provided for each asset. A mapping contract must be provided for all non-native assets (not HIVE or HBD).

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$ref": "#/$defs/InitParams",
  "$defs": {
    "InitParams": {
      "type": "object",
      "required": ["asset0", "asset1"],
      "properties": {
        "asset0": { "type": "string" },
        "asset1": { "type": "string" },
        "fee_bps": { "type": "integer", "minimum": 1, "default": 8 },
        "asset0_mapping_contract": { "type": "string" },
        "asset1_mapping_contract": { "type": "string" }
      }
    }
  }
}
```

#### Field Descriptions

**Required Fields**

- **`asset0`** (string): Symbol for the first asset in the pool.
- **`asset1`** (string): Symbol for the second asset in the pool.

**Optional Fields**

- **`fee_bps`** (integer): Swap fee in basis points (e.g., `8` = 0.08%). Must be > 0. Defaults to `8` if not provided.
- **`asset0_mapping_contract`** (string): Address of a price feed or oracle mapping contract for `asset0`. Omitted if not applicable.
- **`asset1_mapping_contract`** (string): Address of a price feed or oracle mapping contract for `asset1`. Omitted if not applicable.

---

### 2. `swap` — Swap Assets

Exchanges an input asset for an output asset. Supports optional slippage protection, a referral fee, and a beneficiary account.

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$ref": "#/$defs/SwapParams",
  "$defs": {
    "SwapParams": {
      "type": "object",
      "required": ["asset_in", "amount_in", "asset_out", "recipient"],
      "properties": {
        "asset_in": { "type": "string" },
        "amount_in": { "type": "integer" },
        "asset_out": { "type": "string" },
        "min_amount_out": { "type": ["integer", "null"] },
        "recipient": { "type": "string" },
        "beneficiary": { "type": ["string", "null"] },
        "ref_bps": { "type": ["integer", "null"] }
      }
    }
  }
}
```

#### Field Descriptions

**Required Fields**

- **`asset_in`** (string): Identifier of the asset being sold/sent into the pool.
- **`amount_in`** (integer): Amount of `asset_in` to swap, in the asset's smallest unit.
- **`asset_out`** (string): Identifier of the asset to receive from the pool.
- **`recipient`** (string): Account address that will receive the output tokens.

**Optional Fields**

- **`min_amount_out`** (integer | null): Minimum acceptable output amount in the smallest unit of `asset_out`. Acts as slippage protection — the swap will fail if the output would be less. Omit for no minimum.
- **`beneficiary`** (string | null): Account to receive the referral fee portion. Required if `ref_bps` is set.
- **`ref_bps`** (integer | null): Referral fee in basis points deducted from the output amount. Omit if no referral fee applies.

---

### 3. `add_liquidity` — Add Liquidity

Deposits both assets into the pool in exchange for LP tokens, which represent a proportional share of the pool.

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$ref": "#/$defs/AddLiquidityParams",
  "$defs": {
    "AddLiquidityParams": {
      "type": "object",
      "required": ["amount0", "amount1", "recipient"],
      "properties": {
        "amount0": { "type": "integer", "minimum": 0 },
        "amount1": { "type": "integer", "minimum": 0 },
        "recipient": { "type": "string" }
      }
    }
  }
}
```

#### Field Descriptions

**Required Fields**

- **`amount0`** (integer): Amount of `asset0` to deposit, in its smallest unit. Must be ≥ 0.
- **`amount1`** (integer): Amount of `asset1` to deposit, in its smallest unit. Must be ≥ 0.
- **`recipient`** (string): Account address that will receive the minted LP tokens.

---

### 4. `remove_liquidity` — Remove Liquidity

Burns LP tokens to withdraw a proportional share of both assets from the pool.

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$ref": "#/$defs/RemoveLiquidityParams",
  "$defs": {
    "RemoveLiquidityParams": {
      "type": "object",
      "required": ["lp_amount", "recipient"],
      "properties": {
        "lp_amount": { "type": "integer", "minimum": 0 },
        "recipient": { "type": "string" }
      }
    }
  }
}
```

#### Field Descriptions

**Required Fields**

- **`lp_amount`** (integer): Number of LP tokens to burn. Must be ≥ 0. The pool returns a proportional share of `asset0` and `asset1` based on this amount relative to the total LP supply.
- **`recipient`** (string): Account address that will receive the withdrawn assets.

---

## Notes

- All integer amounts are in the **smallest unit** of their respective asset (i.e., no decimal representation).
- Fields typed as `["integer", "null"]` correspond to Go pointer types (`*int64`, `*int`). A JSON `null` value is equivalent to the field being omitted.
- `uint64` fields (amounts, `fee_bps`, `lp_amount`) enforce `"minimum": 0` to reflect unsigned semantics.
- `omitempty` fields in Go are excluded from `required` — they will not be present in serialized output when zero or nil.
