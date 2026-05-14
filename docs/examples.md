# DEX Usage Examples

Practical examples for swapping and providing liquidity using the Router-V2 contract.

All operations go through the router's `execute` method. The router determines whether to do a direct swap or a two-hop swap via HBD automatically based on which pools are registered.

---

## Swapping Native Tokens (HIVE / HBD)

Native tokens (HIVE, HBD, HBD_SAVINGS) do not require pre-approval. The router uses a protocol-level transfer intent to draw funds from your account when the contract executes.

**Example: Swap 10 HBD for HIVE**

```json
// contract: dex-router-v2-abc123
// action: execute
{
  "type": "swap",
  "version": "1.0.0",
  "asset_in": "HBD",
  "asset_out": "HIVE",
  "amount_in": "10000",
  "recipient": "alice",
  "min_amount_out": "9500",
  "destination_chain": "HIVE"
}
```

The transaction must include your active auth. The router builds the protocol intent internally — you do not need to include it yourself.

`min_amount_out` is optional but recommended. If the pool's output falls below this value (due to slippage), the swap reverts.

---

## Swapping Mapped Tokens (BTC, ETH, etc.)

Mapped tokens are held by a mapping contract (e.g. `btc-mapping-def456`). The router uses an ERC-20-style allowance to pull your tokens into the pool. You must approve the router as a spender before the first swap.

### Step 1: Approve the Router

Call `approve` on the **mapping contract** for the asset you want to spend.

```json
// contract: btc-mapping-def456
// action: approve
{
  "spender": "contract:dex-router-v2-abc123",
  "amount": "100000"
}
```

Set `amount` to the total you want the router to be able to spend. You can set a large allowance once and reuse it across multiple swaps, or set the exact amount per swap.

### Step 2: Call Execute on the Router

```json
// contract: dex-router-v2-abc123
// action: execute
{
  "type": "swap",
  "version": "1.0.0",
  "asset_in": "BTC",
  "asset_out": "HBD",
  "amount_in": "50000",
  "recipient": "alice",
  "min_amount_out": "48000",
  "destination_chain": "HIVE"
}
```

The router calls `transferFrom` on the BTC mapping contract, moving your BTC directly into the pool. The pool sees the tokens as pre-deposited and skips its own draw.

---

## Two-Hop Swap (Mapped → Native, via HBD)

If no direct pool exists between two assets, the router routes through HBD automatically. No extra configuration is needed — the router finds the two pools (e.g. BTC/HBD and HBD/HIVE) and chains the swaps.

**Example: Swap BTC for HIVE**

### Step 1: Approve the Router on the Mapping Contract

Same as above — approve the router to spend your BTC.

```json
// contract: btc-mapping-def456
// action: approve
{
  "spender": "contract:dex-router-v2-abc123",
  "amount": "100000"
}
```

### Step 2: Call Execute

```json
// contract: dex-router-v2-abc123
// action: execute
{
  "type": "swap",
  "version": "1.0.0",
  "asset_in": "BTC",
  "asset_out": "HIVE",
  "amount_in": "50000",
  "recipient": "alice",
  "min_amount_out": "900000",
  "destination_chain": "HIVE"
}
```

Internally the router:
1. Transfers your BTC into the BTC/HBD pool via `transferFrom`
2. Receives HBD back into the router contract
3. Passes the HBD through the HBD/HIVE pool
4. Sends HIVE to `recipient`

The `min_amount_out` applies to the final output. You can also set `metadata.min_intermediate` if you want a floor on the HBD received between hops:

```json
{
  "type": "swap",
  "version": "1.0.0",
  "asset_in": "BTC",
  "asset_out": "HIVE",
  "amount_in": "50000",
  "recipient": "alice",
  "min_amount_out": "900000",
  "destination_chain": "HIVE",
  "metadata": {
    "min_intermediate": "48000"
  }
}
```

### Return Address (Optional but Recommended for Two-Hop Swaps)

If the second hop fails (e.g. slippage exceeded), the router needs to know where to return funds. For cross-chain assets, include a `return_address`:

```json
{
  "type": "swap",
  "version": "1.0.0",
  "asset_in": "BTC",
  "asset_out": "HIVE",
  "amount_in": "50000",
  "recipient": "alice",
  "min_amount_out": "900000",
  "destination_chain": "HIVE",
  "return_address": {
    "chain": "BTC",
    "address": "bc1qxy2kgdygjrsqtzq2n0yrf2493p83kkfjhx0wlh"
  }
}
```

---

## Adding Liquidity

Liquidity deposits also go through the router's `execute` method using `type: "deposit"`. The `asset0` and `asset1` fields identify which pool, and `amount0` / `amount1` carry the deposit amounts — all top-level fields.

### Native Token Pool (e.g. HBD/HIVE)

```json
// contract: dex-router-v2-abc123
// action: execute
{
  "type": "deposit",
  "version": "1.0.0",
  "asset0": "HBD",
  "asset1": "HIVE",
  "amount0": "1000000",
  "amount1": "500000",
  "recipient": "alice"
}
```

`asset0` / `amount0` and `asset1` / `amount1` correspond to the pool's assets in alphabetical order (HBD is asset0, HIVE is asset1 in this case). The router normalizes case and enforces the ordering, so you may pass the pair in any order. The router draws both assets from your account via protocol intents.

On the first deposit (empty pool), LP tokens are minted as `sqrt(amount0 * amount1)`. Subsequent deposits mint proportionally to the existing reserves.

### Pool with a Mapped Asset (e.g. BTC/HBD)

For each mapped asset in the pool, approve the router on the relevant mapping contract before depositing.

```json
// contract: btc-mapping-def456
// action: approve
{
  "spender": "contract:dex-router-v2-abc123",
  "amount": "100000"
}
```

Then deposit:

```json
// contract: dex-router-v2-abc123
// action: execute
{
  "type": "deposit",
  "version": "1.0.0",
  "asset0": "BTC",
  "asset1": "HBD",
  "amount0": "50000",
  "amount1": "2000000",
  "recipient": "alice"
}
```

The router transfers BTC from your account via the mapping contract's `transferFrom`, and draws HBD via a protocol intent.

---

## Removing Liquidity

Pass `type: "withdrawal"` with your LP token amount in the top-level `lp_amount` field. The caller must be the LP owner, so `recipient` must match the caller's address.

```json
// contract: dex-router-v2-abc123
// action: execute
{
  "type": "withdrawal",
  "version": "1.0.0",
  "asset0": "HBD",
  "asset1": "HIVE",
  "lp_amount": "250000",
  "recipient": "alice"
}
```

The pool burns the LP tokens and transfers your proportional share of both reserves to `recipient`. For pools with mapped assets, the mapped tokens are transferred out via the mapping contract's `transfer` method — no approval needed for withdrawals.

---

## Optional Fields Summary

| Field | Applies to | Description |
|---|---|---|
| `min_amount_out` | swap | Minimum output before revert (slippage protection) |
| `destination_chain` | swap | External chain for settlement (e.g., `"HIVE"`, `"BTC"`). Omit to settle on Magi |
| `return_address` | swap | Where to return funds if a two-hop swap fails |
| `metadata.min_intermediate` | two-hop swap | Minimum HBD between hops |
| `beneficiary` + `ref_bps` | swap | Referral fee recipient and basis points |
| `amount0` / `amount1` | deposit | Token amounts for liquidity deposit (required) |
| `lp_amount` | withdrawal | LP tokens to burn (required) |
