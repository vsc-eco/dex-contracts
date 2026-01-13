#!/bin/bash

# DEX E2E Test Runner
# This script demonstrates the complete DEX router functionality

set -e

# Configuration
VSC_NODE="${VSC_NODE:-http://localhost:4000}"
ROUTER_SERVICE="${ROUTER_SERVICE:-http://localhost:8080}"
INDEXER_SERVICE="${INDEXER_SERVICE:-http://localhost:8081}"

echo "🧪 Starting DEX E2E Tests..."
echo "VSC Node: $VSC_NODE"
echo "Router: $ROUTER_SERVICE"
echo "Indexer: $INDEXER_SERVICE"
echo

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test counter
TESTS_RUN=0
TESTS_PASSED=0

# Helper function to run a test
run_test() {
    local test_name=$1
    local command=$2
    local expected_exit=${3:-0}

    echo -n "🧪 $test_name... "
    TESTS_RUN=$((TESTS_RUN + 1))

    if eval "$command" > /tmp/test_output 2>&1; then
        if [ $? -eq $expected_exit ]; then
            echo -e "${GREEN}✅ PASSED${NC}"
            TESTS_PASSED=$((TESTS_PASSED + 1))
        else
            echo -e "${RED}❌ FAILED${NC} (wrong exit code)"
            cat /tmp/test_output
        fi
    else
        if [ $expected_exit -ne 0 ]; then
            echo -e "${GREEN}✅ PASSED${NC} (expected failure)"
            TESTS_PASSED=$((TESTS_PASSED + 1))
        else
            echo -e "${RED}❌ FAILED${NC}"
            cat /tmp/test_output
        fi
    fi
}

# Helper function to make API calls
api_call() {
    local method=$1
    local endpoint=$2
    local data=$3

    if [ "$method" = "GET" ]; then
        curl -s -X GET "$ROUTER_SERVICE$endpoint" \
            -H "Content-Type: application/json"
    else
        curl -s -X $method "$ROUTER_SERVICE$endpoint" \
            -H "Content-Type: application/json" \
            -d "$data"
    fi
}

# Helper function to check service availability
check_service() {
    local url=$1
    local service_name=$2

    if curl -s --max-time 5 "$url" > /dev/null; then
        echo -e "${GREEN}✅ $service_name available${NC}"
        return 0
    else
        echo -e "${YELLOW}⚠️  $service_name not available${NC}"
        return 1
    fi
}

echo "🔍 Checking service availability..."
check_service "$VSC_NODE/api/v1/graphql" "VSC Node"
check_service "$ROUTER_SERVICE/api/v1/health" "Router Service"
check_service "$INDEXER_SERVICE/indexer/pools" "Indexer Service"
echo

echo "📦 Testing Pool Creation..."

# Test 1: Create HIVE-HBD pool
run_test "Create HIVE-HBD pool" "
api_call POST '/api/v1/contract/dex-router/create_pool' '{
    \"asset0\": \"HBD\",
    \"asset1\": \"HIVE\",
    \"fee_bps\": 8
}' | jq -e '.success == true' > /dev/null
"

# Test 2: Create BTC-HBD pool
run_test "Create BTC-HBD pool" "
api_call POST '/api/v1/contract/dex-router/create_pool' '{
    \"asset0\": \"HBD\",
    \"asset1\": \"BTC\",
    \"fee_bps\": 8
}' | jq -e '.success == true' > /dev/null
"

echo
echo "💰 Testing Liquidity Operations..."

# Test 3: Add liquidity to HIVE-HBD pool
run_test "Add liquidity HIVE-HBD" "
api_call POST '/api/v1/contract/dex-router/execute' '{
    \"type\": \"deposit\",
    \"version\": \"1.0.0\",
    \"asset_in\": \"HBD\",
    \"asset_out\": \"HIVE\",
    \"recipient\": \"alice\",
    \"metadata\": {
        \"amount0\": \"1000000\",
        \"amount1\": \"500000\"
    }
}' | jq -e '.success == true' > /dev/null
"

# Test 4: Add liquidity to BTC-HBD pool
run_test "Add liquidity BTC-HBD" "
api_call POST '/api/v1/contract/dex-router/execute' '{
    \"type\": \"deposit\",
    \"version\": \"1.0.0\",
    \"asset_in\": \"HBD\",
    \"asset_out\": \"BTC\",
    \"recipient\": \"alice\",
    \"metadata\": {
        \"amount0\": \"2000000\",
        \"amount1\": \"100000\"
    }
}' | jq -e '.success == true' > /dev/null
"

echo
echo "🔄 Testing Swap Operations..."

# Test 5: Direct swap HBD -> HIVE
run_test "Direct swap HBD->HIVE" "
api_call POST '/api/v1/contract/dex-router/execute' '{
    \"type\": \"swap\",
    \"version\": \"1.0.0\",
    \"asset_in\": \"HBD\",
    \"asset_out\": \"HIVE\",
    \"recipient\": \"bob\",
    \"min_amount_out\": 240000,
    \"slippage_bps\": 50
}' | jq -e '.success == true' > /dev/null
"

# Test 6: Two-hop swap BTC -> HBD -> HIVE
run_test "Two-hop swap BTC->HIVE" "
api_call POST '/api/v1/contract/dex-router/execute' '{
    \"type\": \"swap\",
    \"version\": \"1.0.0\",
    \"asset_in\": \"BTC\",
    \"asset_out\": \"HIVE\",
    \"recipient\": \"charlie\",
    \"min_amount_out\": 95000,
    \"slippage_bps\": 150
}' | jq -e '.success == true' > /dev/null
"

echo
echo "❌ Testing Error Scenarios..."

# Test 7: Failed swap with return address
run_test "Failed swap with return address" "
api_call POST '/api/v1/contract/dex-router/execute' '{
    \"type\": \"swap\",
    \"version\": \"1.0.0\",
    \"asset_in\": \"BTC\",
    \"asset_out\": \"HIVE\",
    \"recipient\": \"diana\",
    \"min_amount_out\": 1000000000,
    \"slippage_bps\": 1,
    \"return_address\": {
        \"chain\": \"BTC\",
        \"address\": \"bc1qxy2kgdygjrsqtzq2n0yrf2493p83kkfjhx0wlh\"
    }
}' | jq -e '.success == false' > /dev/null
" 0

# Test 8: Invalid pool creation (same assets)
run_test "Invalid pool creation" "
api_call POST '/api/v1/contract/dex-router/create_pool' '{
    \"asset0\": \"HBD\",
    \"asset1\": \"HBD\",
    \"fee_bps\": 8
}' | jq -e '.success == false' > /dev/null
" 0

echo
echo "🏦 Testing Liquidity Withdrawal..."

# Test 9: Withdraw liquidity
run_test "Withdraw liquidity" "
api_call POST '/api/v1/contract/dex-router/execute' '{
    \"type\": \"withdrawal\",
    \"version\": \"1.0.0\",
    \"asset_in\": \"HBD\",
    \"asset_out\": \"HIVE\",
    \"recipient\": \"alice\",
    \"metadata\": {
        \"lp_amount\": \"353553\"
    }
}' | jq -e '.success == true' > /dev/null
"

echo
echo "📊 Testing Query Operations..."

# Test 10: Query pools
run_test "Query pools" "
curl -s '$INDEXER_SERVICE/indexer/pools' | jq -e 'length >= 2' > /dev/null
"

# Test 11: Query specific pool
run_test "Query specific pool" "
api_call POST '/api/v1/contract/dex-router/get_pool' '\"1\"' | jq -e '.asset0 == \"HBD\"' > /dev/null
"

echo
echo "📈 Test Results Summary"
echo "=========================="
echo "Tests Run: $TESTS_RUN"
echo "Tests Passed: $TESTS_PASSED"
echo "Tests Failed: $((TESTS_RUN - TESTS_PASSED))"

if [ $TESTS_PASSED -eq $TESTS_RUN ]; then
    echo -e "${GREEN}🎉 All tests passed!${NC}"
    exit 0
else
    echo -e "${RED}❌ Some tests failed${NC}"
    exit 1
fi







