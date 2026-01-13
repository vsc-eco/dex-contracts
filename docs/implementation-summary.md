# Implementation Summary: Intent-Based Bag Checking for Two-Hop Swaps

## What We've Implemented

### ✅ Approach 1: DEX Returns Pool State

**Modified Files**:
- `contracts/dex/types.go` - Added `SwapResult` type with `PoolState`
- `contracts/dex/main.go` - Modified `Swap()` to return `SwapResult` with pool state

**Implementation**:
```go
// DEX now returns:
type SwapResult struct {
    AmountOut uint64   `json:"amount_out"`
    PoolState PoolInfo `json:"pool_state"` // Current reserves after swap
}
```

**Benefits**:
- Router receives accurate pool state with every swap
- No separate query needed
- State is guaranteed accurate (from actual transaction)

### ✅ Approach 3: Intent-Based Execution with Automatic Rollback

**Modified Files**:
- `contracts/dex-router-v2/main.go` - Implemented `executeTwoHopSwap()` with intent validation
- `contracts/dex-router-v2/utils.go` - Added pool state cache functions
- `contracts/dex-router-v2/types.go` - Added `SwapResult` and `PoolInfo` types

**Implementation**:
```go
func executeTwoHopSwap(instruction DexInstruction) *string {
    // 1. Execute first swap - VSC validates intents
    firstResult := sdk.ContractCall(pool1Id, "swap", ...)
    
    // 2. Parse result (includes pool state)
    var swapResult1 SwapResult
    tinyjson.Unmarshal(firstResult, &swapResult1)
    
    // 3. Update cache
    setPoolState(pool1Id, swapResult1.PoolState)
    
    // 4. Execute second swap with actual amount
    secondResult := sdk.ContractCall(pool2Id, "swap", ...)
    
    // 5. Handle failure: return intermediate HBD
    if failed {
        return handlePartialSwap(swapResult1.AmountOut, instruction)
    }
    
    // 6. Update cache
    setPoolState(pool2Id, swapResult2.PoolState)
}
```

**Key Features**:
- ✅ Leverages VSC's intent validation (bag checking)
- ✅ Automatic rollback on failure (VSC handles this)
- ✅ Cache updated from actual transaction results
- ✅ Handles partial swap failures gracefully

### ⏳ Approach 2: Pool Registry (Not Implemented)

**Status**: Documented but not implemented

**Reason**: Approach 1 + 3 provides sufficient functionality without the complexity of a separate registry contract.

**Future Consideration**: Could be added if multiple routers need shared state.

## How It Works

### Intent Flow

1. **User creates transaction** with intents:
   ```json
   {
     "intents": [
       {"type": "transfer.allow", "args": {"limit": "1000", "token": "BTC"}},
       {"type": "transfer.allow", "args": {"limit": "1000000", "token": "HBD"}}
     ]
   }
   ```

2. **Router executes two-hop swap**:
   - First swap: BTC -> HBD (VSC validates BTC intent)
   - DEX returns: `{amount_out: 500, pool_state: {...}}`
   - Router caches pool state
   - Second swap: HBD -> HIVE (VSC validates HBD intent)
   - DEX returns: `{amount_out: 2000, pool_state: {...}}`
   - Router caches pool state

3. **If failure occurs**:
   - VSC automatically rolls back (if before execution)
   - Or router returns intermediate asset (if first swap succeeded)

### Cache Management

```go
// Cache updated after each successful swap
setPoolState(poolId, swapResult.PoolState)

// Used for pre-calculation (if available)
poolState := getPoolState(poolId)
if poolState.Reserve0 > 0 {
    // Use cached state for routing decisions
}
```

## Benefits

1. ✅ **No Pre-calculation Needed**: Can execute optimistically, rely on intents
2. ✅ **Automatic Validation**: VSC handles transfer limits
3. ✅ **Atomic Transactions**: Succeed or fail completely
4. ✅ **Accurate State**: Cache updated from actual results
5. ✅ **Efficient**: No separate queries needed

## Failure Handling

### Scenario: Second Hop Fails

```
1. First swap succeeds: BTC -> 500 HBD
2. Router now holds 500 HBD
3. Second swap fails: Insufficient liquidity
4. Router returns 500 HBD to user
5. Error: "second hop failed, returned intermediate HBD: 500"
```

**User Experience**:
- Receives intermediate HBD instead of final asset
- Can retry swap or use HBD separately
- No funds lost (VSC ensures atomicity)

## Next Steps

1. ✅ DEX returns pool state - **IMPLEMENTED**
2. ✅ Router caches pool state - **IMPLEMENTED**
3. ✅ Two-hop swap with intents - **IMPLEMENTED**
4. ⏳ Add comprehensive error logging
5. ⏳ Add retry logic for partial swaps
6. ⏳ Test with various failure scenarios
7. ⏳ Consider Approach 2 (pool registry) if needed

## Testing Recommendations

- Test successful two-hop swaps
- Test first swap failure (slippage exceeded)
- Test second swap failure (insufficient liquidity)
- Test intent violation (exceeding declared limits)
- Test concurrent swaps updating same pools
- Test cache invalidation scenarios
