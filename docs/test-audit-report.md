# Test Coverage Audit Report

**Generated:** 2025-01-29  
**Last Audit:** 2025-01-29 (full audit for stubs, blinded tests, coverage)

## Executive Summary

| Package | Status | Coverage | Tests |
|---------|--------|----------|-------|
| tests | ✅ PASS | N/A | 1 |
| schemas | ✅ PASS | 88.3% | 15 |
| services/indexer | ✅ PASS | 52.3% | 36 |
| services/router | ✅ PASS | 41.4% | 20 |
| sdk/go | ✅ PASS | 39.0% | 9 |
| contracts/dex-router-v2/test | ✅ PASS | N/A | 8 |
| contracts/dex/test | ✅ PASS | N/A | 3 |
| contracts/dex-router | ✅ PASS | N/A | 15+ |

---

## Detailed Test Status

### 1. tests (root)

**Status:** ✅ PASS  
**Command:** `go test -v ./tests/...`

| Test | Status |
|------|--------|
| TestContractLoading | ✅ PASS |

**Coverage:** Loads embedded wasm artifacts (DexWasm, DexRouterWasm, DexRouterV2Wasm, BtcMappingWasm).

---

### 2. schemas

**Status:** ✅ PASS  
**Coverage:** 88.3%  
**Command:** `cd schemas && go test -v -cover ./...`

| Test | Status |
|------|--------|
| TestParseFromJSON | ✅ PASS |
| TestParseFromQueryParams | ✅ PASS |
| TestParseFromMemo | ✅ PASS |
| TestParseFromCustomJSON | ✅ PASS |
| TestValidateInstructionStruct | ✅ PASS |
| TestSwapInstruction_Validate | ✅ PASS |
| TestValidationError_Error | ✅ PASS |
| TestSwapInstruction_ToJSON | ✅ PASS |
| TestValidateInstruction | ✅ PASS |

**New tests added:** ParseFromCustomJSON, ValidateInstructionStruct, SwapInstruction.Validate, ValidationError.Error, SwapInstruction.ToJSON.

---

### 3. services/indexer

**Status:** ✅ PASS  
**Coverage:** 52.3%+  
**Command:** `cd services/indexer && go test -v -cover ./...`

| Test Category | Count | Status |
|---------------|-------|--------|
| Service | 10 | ✅ |
| DexReadModel | 12 | ✅ |
| Server | 13 | ✅ (incl. pool accounts, richlist, transactions) |
| WebSocket | 3 | ✅ |
| ParseVSCEvent | 1 | ✅ |

---

### 4. services/router

**Status:** ✅ PASS  
**Coverage:** 41.4%  
**Command:** `cd services/router && go test -v -cover ./...`

| Test Category | Count | Status |
|---------------|-------|--------|
| IndexerAdapter | 6 | ✅ |
| Service | 4 | ✅ |
| ExecuteSwap/Deposit/Withdrawal | 3 | ✅ |
| SwapValidation | 1 | ✅ |

---

### 5. sdk/go

**Status:** ✅ PASS  
**Coverage:** 39.0%  
**Command:** `cd sdk/go && go test -v -cover ./...`

| Test | Status |
|------|--------|
| TestNewClient | ✅ PASS |
| TestConfig_ContractAddresses | ✅ PASS |
| TestRouteResult_JSON | ✅ PASS |
| TestClient_ComputeDexRoute_Success | ✅ PASS |
| TestClient_ComputeDexRoute_ServerError | ✅ PASS |
| TestClient_ComputeDexRoute_InvalidJSON | ✅ PASS |
| TestClient_GetPools_Success | ✅ PASS |
| TestClient_GetPools_ServerError | ✅ PASS |
| TestIntent_Struct | ✅ PASS |

**New tests added:** All 9 tests (previously none). Uses httptest for HTTP mocking.

**Bug fixed:** `c.config.DexRouter` → `c.config.Contracts.DexRouter` in ExecuteDexOperationWithIntents.

---

### 6. contracts/dex-router-v2/test

**Status:** ✅ PASS  
**Command:** `cd contracts/dex-router-v2/test && go test -v ./...`

| Test | Status |
|------|--------|
| TestPoolRegistration | ✅ PASS |
| TestFindPool | ✅ PASS |
| TestTwoHopRouting | ✅ PASS |
| TestTokenRegistryEnforcement | ✅ PASS |
| TestSplitChains | ✅ PASS |
| TestSupportedChains | ✅ PASS |
| TestPoolKeyNormalization | ✅ PASS |

**New tests added:** TestSplitChains (splitChains logic replicated for standalone test package).

---

### 7. contracts/dex/test

**Status:** ✅ PASS  
**Command:** `cd contracts/dex/test && go test -v ./...`

| Test | Status |
|------|--------|
| TestCalculateSwapOutput | ✅ PASS |
| TestLPTokenMinting | ✅ PASS |
| TestLiquidityRemoval | ✅ PASS |

**Coverage:** AMM math (constant product, LP minting, proportional removal).

---

### 8. contracts/dex-router

**Status:** ✅ PASS  
**Command:** `cd contracts/dex-router && go test -v ./...`

**TestDexRouterInit:** ✅ PASS (integration test with vsc-node)

**TestCreatePool, TestAddLiquidity, TestDirectSwap:** Skipped (vsc-node DataBin.Get nil pointer upstream bug). Use `runInTempDir` when fixed.

**Unit tests:** InstructionParsing, SlippageCalculation, FeeCalculations, JSONValidation, PoolReserveUpdates, MathFunctions, etc. — all passing.

---

## Audit Findings (2025-01-29)

### Issues Fixed

1. **TestErrorConditions (contracts/dex-router/dex_router_test.go)** – Fixed blinded assertions:
   - Previously used `t.Logf` for same-asset case (no assertion) and inverted logic for empty assets.
   - Now asserts required-fields validation correctly for empty asset_in/asset_out and same-asset scenarios.

2. **TestDexReadModel_HandleEvent_RegisterPool_WithoutChains (services/indexer/readmodels_test.go)** – Added assertions:
   - Previously had no assertions; only verified HandleEvent did not error.
   - Now asserts pool creation, asset mapping, and dexContractToPool mapping.

### Blinded / Specification Tests (By Design)

These packages test **replicated logic** rather than the actual contract code, because contracts compile to WASM and cannot be unit-tested directly in Go:

| Package | What's Tested | Caveat |
|---------|---------------|--------|
| **contracts/dex/test** | AMM math (swap output, LP minting, liquidity removal) | Uses local copies of sqrt128, min64, calculateSwapOutput. If contract's utils.go diverges, tests may pass while contract fails. |
| **contracts/dex-router-v2/test** | Pool registration, find pool, two-hop routing, token registry, splitChains | All logic replicated inline. Tests specification, not implementation. |
| **contracts/dex-router/test** | Same as dex-router-v2 (standalone module) | Replicates DexInstruction, bitsMul64, sqrt128, calculateSwapOutput. |

**Recommendation:** Keep these as specification tests but ensure replicated logic stays in sync with contract source. Consider adding a CI check that compares key algorithms (e.g. sqrt128, splitChains) between test and main.

### Tests Using Mocks (Appropriate)

- **services/router** – Uses mockDEXExecutor to verify instruction payload construction without calling VSC.
- **services/indexer** – Uses mockGraphQLServer, mockWebSocketServer, mockReadModel for HTTP/WS and event flow.
- **sdk/go** – Uses httptest for HTTP client behavior.

These mocks are appropriate for unit testing; they isolate the code under test.

### Skipped Tests (Documented)

- **TestCreatePool, TestAddLiquidity, TestDirectSwap** (contracts/dex-router/contract_test.go) – Skipped due to vsc-node DataBin.Get nil pointer upstream bug. Re-enable when fixed.

---

## Coverage Gaps (Future Work)

1. **contracts/dex-router** - Re-enable TestCreatePool, TestAddLiquidity, TestDirectSwap when vsc-node DataBin fix is released.
2. **services/indexer** - Increase from 52% (cmd package 0%).
3. **services/router** - Increase from 41% (cmd, types packages 0%).
4. **sdk/go** - broadcastTx, ExecuteDexSwap, ExecuteDexOperationWithIntents (require GraphQL mock).
5. **E2E** - Full contract flow (router + dex + btc-mapping) in ContractTest.

---

## Running All Tests

```bash
# Root tests
go test -v ./tests/...

# Schemas
cd schemas && go test -v -cover ./...

# Services
cd services/indexer && go test -v -cover ./...
cd services/router && go test -v -cover ./...

# SDK
cd sdk/go && go test -v -cover ./...

# Contract unit tests (no wasm runtime)
cd contracts/dex-router-v2/test && go test -v ./...
cd contracts/dex/test && go test -v ./...

# Contract integration tests (requires vsc-node, isolated env)
cd contracts/dex-router && go test -v ./...
```

---

## Summary

- **8 of 8** test packages pass.
- **Audit fixes applied:** TestErrorConditions (proper assertions), TestDexReadModel_HandleEvent_RegisterPool_WithoutChains (added assertions).
- **Blinded tests documented:** dex/test, dex-router-v2/test, dex-router/test use replicated logic; keep in sync with contract source.
- **Coverage:** schemas 88.3%, indexer 52.3%, router 41.4%, sdk 39.0%.
