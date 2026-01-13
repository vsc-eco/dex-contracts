# DEX Router E2E Test Suite

This document provides comprehensive end-to-end test scenarios for the VSC DEX Router contract, demonstrating all major functionality including pool creation, liquidity management, swaps, fee collection, and error handling.

## Test Setup

### Prerequisites
- VSC node running with DEX router contract deployed
- Test accounts with sufficient balances
- Router service running and connected to indexer

### Test Assets
- `HIVE`: Native Hive tokens
- `HBD`: Hive Backed Dollars
- `BTC`: Wrapped Bitcoin (mapped via btc-mapping contract)

## Test Scenario 1: Pool Creation and Liquidity Provision

### 1.1 Create HIVE-HBD Pool

**API Call:**
```bash
curl -X POST http://localhost:8080/api/v1/contract/dex-router/create_pool \
  -H "Content-Type: application/json" \
  -d '{
    "asset0": "HBD",
    "asset1": "HIVE",
    "fee_bps": 8
  }'
```

**Expected Response:**
```json
{
  "success": true,
  "pool_id": "1"
}
```

### 1.2 Create BTC-HBD Pool

**API Call:**
```bash
curl -X POST http://localhost:8080/api/v1/contract/dex-router/create_pool \
  -H "Content-Type: application/json" \
  -d '{
    "asset0": "HBD",
    "asset1": "BTC",
    "fee_bps": 8
  }'
```

**Expected Response:**
```json
{
  "success": true,
  "pool_id": "2"
}
```

### 1.3 Add Liquidity to HIVE-HBD Pool

**API Call:**
```bash
curl -X POST http://localhost:8080/api/v1/contract/dex-router/execute \
  -H "Content-Type: application/json" \
  -d '{
    "type": "deposit",
    "version": "1.0.0",
    "asset_in": "HBD",
    "asset_out": "HIVE",
    "recipient": "alice",
    "metadata": {
      "amount0": "1000000",
      "amount1": "500000"
    }
  }'
```

**Expected Response:**
```json
{
  "success": true,
  "tx_hash": "abc123..."
}
```

### 1.4 Add Liquidity to BTC-HBD Pool

**API Call:**
```bash
curl -X POST http://localhost:8080/api/v1/contract/dex-router/execute \
  -H "Content-Type: application/json" \
  -d '{
    "type": "deposit",
    "version": "1.0.0",
    "asset_in": "HBD",
    "asset_out": "BTC",
    "recipient": "alice",
    "metadata": {
      "amount0": "2000000",
      "amount1": "100000"
    }
  }'
```

**Expected Response:**
```json
{
  "success": true,
  "tx_hash": "def456..."
}
```

### 1.5 Query Pool Information

**API Call:**
```bash
curl http://localhost:8081/indexer/pools
```

**Expected Response:**
```json
[
  {
    "id": "1",
    "asset0": "HBD",
    "asset1": "HIVE",
    "reserve0": 1000000,
    "reserve1": 500000,
    "fee": 8,
    "total_supply": 707106
  },
  {
    "id": "2",
    "asset0": "HBD",
    "asset1": "BTC",
    "reserve0": 2000000,
    "reserve1": 100000,
    "fee": 8,
    "total_supply": 1414213
  }
]
```

## Test Scenario 2: Direct Swap Operations

### 2.1 HBD to HIVE Swap

**API Call:**
```bash
curl -X POST http://localhost:8080/api/v1/contract/dex-router/execute \
  -H "Content-Type: application/json" \
  -d '{
    "type": "swap",
    "version": "1.0.0",
    "asset_in": "HBD",
    "asset_out": "HIVE",
    "recipient": "bob",
    "min_amount_out": 245000,
    "slippage_bps": 50
  }'
```

**Expected Response:**
```json
{
  "success": true,
  "tx_hash": "ghi789..."
}
```

### 2.2 BTC to HBD Swap

**API Call:**
```bash
curl -X POST http://localhost:8080/api/v1/contract/dex-router/execute \
  -H "Content-Type: application/json" \
  -d '{
    "type": "swap",
    "version": "1.0.0",
    "asset_in": "BTC",
    "asset_out": "HBD",
    "recipient": "charlie",
    "min_amount_out": 19000000,
    "slippage_bps": 100
  }'
```

**Expected Response:**
```json
{
  "success": true,
  "tx_hash": "jkl012..."
}
```

## Test Scenario 3: Two-Hop Swap (BTC → HBD → HIVE)

### 3.1 Execute Two-Hop Swap

**API Call:**
```bash
curl -X POST http://localhost:8080/api/v1/contract/dex-router/execute \
  -H "Content-Type: application/json" \
  -d '{
    "type": "swap",
    "version": "1.0.0",
    "asset_in": "BTC",
    "asset_out": "HIVE",
    "recipient": "diana",
    "min_amount_out": 95000,
    "slippage_bps": 150
  }'
```

**Expected Response:**
```json
{
  "success": true,
  "tx_hash": "mno345..."
}
```

### 3.2 Verify Two-Hop Swap Results

**Check Pool Reserves After Swap:**
```bash
curl http://localhost:8081/indexer/pools
```

**Expected: Reserves should show the net effect of both swaps**

## Test Scenario 4: Failed Swap with Return Address

### 4.1 Attempt Swap with Insufficient Reserves

**API Call:**
```bash
curl -X POST http://localhost:8080/api/v1/contract/dex-router/execute \
  -H "Content-Type: application/json" \
  -d '{
    "type": "swap",
    "version": "1.0.0",
    "asset_in": "BTC",
    "asset_out": "HIVE",
    "recipient": "eve",
    "min_amount_out": 1000000000,
    "slippage_bps": 1,
    "return_address": {
      "chain": "BTC",
      "address": "bc1qxy2kgdygjrsqtzq2n0yrf2493p83kkfjhx0wlh"
    }
  }'
```

**Expected Response:**
```json
{
  "success": false,
  "error": "slippage tolerance exceeded",
  "refund_tx": "pqr678..."
}
```

### 4.2 Verify Return Address Processing

**Check Transaction Logs:**
```bash
curl http://localhost:4000/api/v1/graphql \
  -H "Content-Type: application/json" \
  -d '{
    "query": "query { transaction(hash: \"pqr678...\") { logs } }"
  }'
```

## Test Scenario 5: Referral Fees

### 5.1 Swap with Referral

**API Call:**
```bash
curl -X POST http://localhost:8080/api/v1/contract/dex-router/execute \
  -H "Content-Type: application/json" \
  -d '{
    "type": "swap",
    "version": "1.0.0",
    "asset_in": "HBD",
    "asset_out": "HIVE",
    "recipient": "frank",
    "beneficiary": "referrer",
    "ref_bps": 250,
    "min_amount_out": 240000
  }'
```

**Expected Response:**
```json
{
  "success": true,
  "tx_hash": "stu901..."
}
```

## Test Scenario 6: Fee Collection

### 6.1 Check Accumulated Fees

**API Call:**
```bash
curl http://localhost:8080/api/v1/contract/dex-router/get_pool \
  -H "Content-Type: application/json" \
  -d '"1"'
```

**Expected Response:**
```json
{
  "asset0": "HBD",
  "asset1": "HIVE",
  "reserve0": 1024000,
  "reserve1": 497000,
  "fee": 8,
  "total_lp": 707106,
  "fee0": 3200,
  "fee1": 0
}
```

### 6.2 Claim Fees (System Operation)

**API Call:**
```bash
curl -X POST http://localhost:8080/api/v1/contract/dex-router/claim_fees \
  -H "Content-Type: application/json" \
  -d '"1"'
```

**Expected Response:**
```json
{
  "success": true,
  "claimed_hbd": 3200
}
```

## Test Scenario 7: Liquidity Withdrawal

### 7.1 Withdraw Partial Liquidity

**API Call:**
```bash
curl -X POST http://localhost:8080/api/v1/contract/dex-router/execute \
  -H "Content-Type: application/json" \
  -d '{
    "type": "withdrawal",
    "version": "1.0.0",
    "asset_in": "HBD",
    "asset_out": "HIVE",
    "recipient": "alice",
    "metadata": {
      "lp_amount": 353553
    }
  }'
```

**Expected Response:**
```json
{
  "success": true,
  "withdrawn_hbd": 512000,
  "withdrawn_hive": 248500
}
```

### 7.2 Withdraw All Remaining Liquidity

**API Call:**
```bash
curl -X POST http://localhost:8080/api/v1/contract/dex-router/execute \
  -H "Content-Type: application/json" \
  -d '{
    "type": "withdrawal",
    "version": "1.0.0",
    "asset_in": "HBD",
    "asset_out": "HIVE",
    "recipient": "alice",
    "metadata": {
      "lp_amount": 353553
    }
  }'
```

**Expected Response:**
```json
{
  "success": true,
  "withdrawn_hbd": 512000,
  "withdrawn_hive": 248500
}
```

## Test Verification Scripts

### Automated Test Runner

Create `test-dex-e2e.sh`:

```bash
#!/bin/bash

# DEX E2E Test Runner
set -e

echo "🧪 Starting DEX E2E Tests..."

# Test 1: Pool Creation
echo "📦 Creating pools..."
create_pool "HBD" "HIVE" 8
create_pool "HBD" "BTC" 8

# Test 2: Liquidity Addition
echo "💰 Adding liquidity..."
add_liquidity "alice" "HBD" "HIVE" 1000000 500000
add_liquidity "alice" "HBD" "BTC" 2000000 100000

# Test 3: Direct Swaps
echo "🔄 Testing direct swaps..."
swap "bob" "HBD" "HIVE" 100000 245000
swap "charlie" "BTC" "HBD" 10000 19000000

# Test 4: Two-hop Swap
echo "🔀 Testing two-hop swap..."
swap "diana" "BTC" "HIVE" 5000 95000

# Test 5: Failed Swap
echo "❌ Testing failed swap..."
swap_with_return "eve" "BTC" "HIVE" 10000 1000000000 "bc1q..."

# Test 6: Referral Swap
echo "👥 Testing referral swap..."
swap_with_referral "frank" "HBD" "HIVE" 100000 240000 "referrer" 250

# Test 7: Fee Collection
echo "💸 Testing fee collection..."
claim_fees 1

# Test 8: Liquidity Withdrawal
echo "🏦 Testing liquidity withdrawal..."
withdraw_liquidity "alice" "HBD" "HIVE" 353553

echo "✅ All tests completed!"
```

### Helper Functions

```bash
#!/bin/bash

API_BASE="http://localhost:8080/api/v1/contract/dex-router"

create_pool() {
    local asset0=$1 asset1=$2 fee=$3
    curl -s -X POST $API_BASE/create_pool \
        -H "Content-Type: application/json" \
        -d "{\"asset0\":\"$asset0\",\"asset1\":\"$asset1\",\"fee_bps\":$fee}" \
        | jq .
}

add_liquidity() {
    local user=$1 asset0=$2 asset1=$3 amt0=$4 amt1=$5
    curl -s -X POST $API_BASE/execute \
        -H "Content-Type: application/json" \
        -d "{
            \"type\":\"deposit\",
            \"version\":\"1.0.0\",
            \"asset_in\":\"$asset0\",
            \"asset_out\":\"$asset1\",
            \"recipient\":\"$user\",
            \"metadata\":{\"amount0\":\"$amt0\",\"amount1\":\"$amt1\"}
        }" | jq .
}

swap() {
    local user=$1 asset_in=$2 asset_out=$3 amt_in=$4 min_out=$5
    curl -s -X POST $API_BASE/execute \
        -H "Content-Type: application/json" \
        -d "{
            \"type\":\"swap\",
            \"version\":\"1.0.0\",
            \"asset_in\":\"$asset_in\",
            \"asset_out\":\"$asset_out\",
            \"recipient\":\"$user\",
            \"min_amount_out\":$min_out
        }" | jq .
}

swap_with_return() {
    local user=$1 asset_in=$2 asset_out=$3 amt_in=$4 min_out=$5 ret_addr=$6
    curl -s -X POST $API_BASE/execute \
        -H "Content-Type: application/json" \
        -d "{
            \"type\":\"swap\",
            \"version\":\"1.0.0\",
            \"asset_in\":\"$asset_in\",
            \"asset_out\":\"$asset_out\",
            \"recipient\":\"$user\",
            \"min_amount_out\":$min_out,
            \"return_address\":{\"chain\":\"BTC\",\"address\":\"$ret_addr\"}
        }" | jq .
}

withdraw_liquidity() {
    local user=$1 asset0=$2 asset1=$3 lp_amt=$4
    curl -s -X POST $API_BASE/execute \
        -H "Content-Type: application/json" \
        -d "{
            \"type\":\"withdrawal\",
            \"version\":\"1.0.0\",
            \"asset_in\":\"$asset0\",
            \"asset_out\":\"$asset1\",
            \"recipient\":\"$user\",
            \"metadata\":{\"lp_amount\":\"$lp_amt\"}
        }" | jq .
}

claim_fees() {
    local pool_id=$1
    curl -s -X POST $API_BASE/claim_fees \
        -H "Content-Type: application/json" \
        -d "\"$pool_id\"" | jq .
}
```

## Test Results Verification

### Pool State Verification
```bash
#!/bin/bash

verify_pool_state() {
    local pool_id=$1 expected_reserve0=$2 expected_reserve1=$3

    local state=$(curl -s http://localhost:8080/api/v1/contract/dex-router/get_pool \
        -H "Content-Type: application/json" \
        -d "\"$pool_id\"" | jq -r ".reserve0, .reserve1")

    local actual_reserve0=$(echo $state | awk '{print $1}')
    local actual_reserve1=$(echo $state | awk '{print $2}')

    if [ "$actual_reserve0" = "$expected_reserve0" ] && [ "$actual_reserve1" = "$expected_reserve1" ]; then
        echo "✅ Pool $pool_id state correct"
    else
        echo "❌ Pool $pool_id state mismatch: expected $expected_reserve0/$expected_reserve1, got $actual_reserve0/$actual_reserve1"
        return 1
    fi
}
```

### Balance Verification
```bash
#!/bin/bash

verify_balance() {
    local account=$1 asset=$2 expected_balance=$3

    # Query VSC balance (would need to implement)
    local actual_balance=$(get_vsc_balance $account $asset)

    if [ "$actual_balance" = "$expected_balance" ]; then
        echo "✅ $account $asset balance correct: $actual_balance"
    else
        echo "❌ $account $asset balance mismatch: expected $expected_balance, got $actual_balance"
        return 1
    fi
}
```

## Error Scenarios to Test

### Invalid Pool Creation
```json
{
  "asset0": "HBD",
  "asset1": "HBD",
  "fee_bps": 8
}
```
**Expected:** `"error", "assets must be different"`

### Insufficient Liquidity
```json
{
  "type": "swap",
  "version": "1.0.0",
  "asset_in": "BTC",
  "asset_out": "SOL",
  "recipient": "user"
}
```
**Expected:** `"error", "no suitable pool found"`

### Slippage Exceeded
```json
{
  "type": "swap",
  "version": "1.0.0",
  "asset_in": "HBD",
  "asset_out": "HIVE",
  "recipient": "user",
  "min_amount_out": 1000000,
  "slippage_bps": 1
}
```
**Expected:** `"error", "slippage tolerance exceeded"`

## Performance Benchmarks

### Pool Creation: < 100ms
### Direct Swap: < 50ms
### Two-Hop Swap: < 150ms
### Liquidity Operations: < 200ms

## Monitoring and Debugging

### Transaction Logs
```bash
# Monitor contract events
curl http://localhost:4000/api/v1/graphql \
  -H "Content-Type: application/json" \
  -d '{
    "query": "subscription { contractEvents(contractId: \"dex-router\") { method args } }"
  }'
```

### Pool State Monitoring
```bash
# Continuous pool monitoring
while true; do
  curl -s http://localhost:8081/indexer/pools | jq .
  sleep 5
done
```







