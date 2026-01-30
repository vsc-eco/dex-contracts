# Documentation to Tests Mapping

This document maps published features and user journeys from the docs to their test coverage. Use it to verify that all documented functionality is tested.

## Instruction Schema (instruction-schema.md)

| Feature / User Journey | Doc Reference | Test | Status |
|------------------------|---------------|------|--------|
| Parse from JSON (basic swap) | Basic BTC to HBD Swap | `schemas/parser_test.go` TestParseFromJSON | ✅ |
| Parse from JSON (optional: slippage, min_amount_out, beneficiary, ref_bps, return_address, metadata) | Examples section | TestParseFromJSON instruction_with_optional_fields | ✅ |
| Parse from memo JSON format | Transfer Memo (JSON) | TestParseFromMemo JSON_format | ✅ |
| Parse from memo URL query format | Transfer Memo (URL Query) | TestParseFromMemo URL_query_format | ✅ |
| Parse from memo with return_address in URL | URL Query with Return Address | TestParseFromQueryParams query_with_optional_fields | ✅ |
| Parse from Custom JSON (vsc.dex_swap) | Custom JSON Operations | TestParseFromCustomJSON | ✅ |
| BTC to HBD_SAVINGS with slippage | BTC to HBD Savings example | TestParseFromJSON instruction_with_optional_fields | ✅ |
| Swap with referral (beneficiary, ref_bps) | Swap with Referral | TestParseFromJSON instruction_with_optional_fields | ✅ |
| Swap with return_address | Swap with Return Address | TestParseFromJSON instruction_with_optional_fields | ✅ |
| Validate required fields | Validation Rules | TestSwapInstruction_Validate, TestValidateInstruction | ✅ |
| Validate version semver | Validation Rules | TestValidateInstruction invalid_instruction | ✅ |
| Validate slippage_bps range (0-10000) | Validation Rules | TestValidateInstruction invalid_slippage_bps | ✅ |
| Validate invalid type | Validation Rules | TestValidateInstruction invalid_type | ✅ |
| ToJSON round-trip | - | TestSwapInstruction_ToJSON | ✅ |

## Asset and Chain Registration (asset-chain-registration.md)

| Feature / User Journey | Doc Reference | Test | Status |
|------------------------|---------------|------|--------|
| Register pool with chain info (asset0_chain, asset1_chain) | Pool Registration | TestDexReadModel_HandleEvent_RegisterPool_WithChains | ✅ |
| Register pool without chain info | Pool Registration | TestDexReadModel_HandleEvent_RegisterPool_WithoutChains | ✅ |
| Token registry enforcement (both assets must be registered) | Token Registration | contracts/dex-router-v2/test TestTokenRegistryEnforcement | ✅ |
| Pool key normalization (alphabetical) | - | TestPoolKeyNormalization, TestPoolRegistration | ✅ |
| Dynamic schema with return_address.chain enum | Schema API | TestDexReadModel_GetSchema, TestServer_handleGetSchema | ✅ |
| Supported chains list | Schema API | TestDexReadModel_GetSchema | ✅ |

## Indexer API (indexer-api.md)

| Endpoint | Doc Reference | Test | Status |
|----------|---------------|------|--------|
| GET /api/v1/pools | Get All Pools | TestServer_handleGetPools | ✅ |
| GET /api/v1/pools/{id} | Get Specific Pool | TestServer_handleGetPool_Existing, TestServer_handleGetPool_NotFound | ✅ |
| GET /api/v1/pools/{id}/accounts | Get Pool Liquidity Accounts | TestServer_handleGetPoolAccounts, TestServer_handleGetPoolAccounts_NotFound | ✅ |
| GET /api/v1/pools/{id}/richlist | Get Pool Rich List | TestServer_handleGetPoolRichList, TestServer_handleGetPoolRichList_NotFound | ✅ |
| GET /api/v1/transactions | Get Transaction History | TestServer_handleGetTransactions | ✅ |
| GET /api/v1/transactions/{id} | Get Specific Transaction | TestServer_handleGetTransaction, TestServer_handleGetTransaction_NotFound | ✅ |
| GET /api/v1/schema | Get Instruction Schema | TestServer_handleGetSchema, TestServer_handleGetSchema_NoReader | ✅ |
| GET /health | Service Health | TestServer_handleHealth | ✅ |

## Router Service (architecture-v2.md, examples)

| Feature / User Journey | Doc Reference | Test | Status |
|------------------------|---------------|------|--------|
| Execute swap (instruction payload) | E2E examples | router/router_test.go TestExecuteSwap | ✅ |
| Execute deposit | E2E examples | TestExecuteDeposit | ✅ |
| Execute withdrawal | E2E examples | TestExecuteWithdrawal | ✅ |
| Same-asset validation | Error scenarios | TestSwapValidation | ✅ |
| Instruction structure (type, version, asset_in, asset_out, recipient, min_amount_out, slippage_bps, beneficiary, ref_bps) | - | TestExecuteSwap (verifies JSON payload) | ✅ |

## SDK (sdk/go)

| Feature | Doc Reference | Test | Status |
|---------|---------------|------|--------|
| ComputeDexRoute (POST /router/route) | Direct Router API | client_test.go TestClient_ComputeDexRoute_Success | ✅ |
| GetPools (GET /indexer/pools) | - | TestClient_GetPools_Success | ✅ |
| Error handling (server error, invalid JSON) | - | TestClient_ComputeDexRoute_ServerError, TestClient_ComputeDexRoute_InvalidJSON | ✅ |

## Router Indexer Adapter (indexer-adapter)

| Feature | Doc Reference | Test | Status |
|---------|---------------|------|--------|
| GetPoolByID | - | TestGetPoolByID_Success, NotFound, ServerError, MalformedJSON, NetworkError | ✅ |
| GetPoolsByAsset | - | TestGetPoolsByAsset_Success, FiltersCorrectly, EmptyList, FeeConversion | ✅ |

## Contract Logic (dex-router-v2, dex)

| Feature | Doc Reference | Test | Status |
|---------|---------------|------|--------|
| Two-hop routing (BTC → HBD → HIVE) | architecture-v2.md | dex-router-v2/test TestTwoHopRouting | ✅ (spec) |
| Pool finding (direct + reversed assets) | - | TestFindPool | ✅ (spec) |
| AMM calculations (swap output, LP minting, liquidity removal) | - | dex/test, dex-router | ✅ |
| Slippage, fee, referral calculations | - | dex_router_test.go | ✅ |
| Instruction parsing (swap, deposit, unknown type) | - | TestInstructionParsing | ✅ |

## E2E / Integration (examples/dex-e2e-test.md)

| Scenario | Doc Reference | Test | Status |
|----------|---------------|------|--------|
| Pool creation + liquidity + swap | E2E Test Suite | contract_test.go (skipped: upstream bug) | ⏸️ Skipped |
| Two-hop swap | E2E Test Suite | dex-router-v2/test (spec test) | ✅ (spec) |
| Failed swap with return address | E2E Test Suite, V2-SYSTEM-SUMMARY | dex-router-v2/test TestTwoHopFailure_* | ✅ |
| Referral fees | E2E Test Suite | TestExecuteSwap (payload), TestReferralFeeCalculations | ✅ |
| Liquidity withdrawal | E2E Test Suite | TestExecuteWithdrawal | ✅ |

## Router Instruction API (instruction-schema.md)

| Endpoint | Doc Reference | Test | Status |
|----------|---------------|------|--------|
| POST /api/v1/instruction | Direct Router API | router/server_test.go TestServer_handleExecuteInstruction_* | ✅ |
| POST /api/v1/route | - | TestServer_handleComputeRoute | ✅ |
| GET /health | - | TestServer_handleHealth | ✅ |

## Coverage Gaps (Future Work)

1. **E2E contract tests** – TestCreatePool, TestAddLiquidity, TestDirectSwap skipped due to vsc-node upstream bug.
2. **Transaction filtering** – handleGetTransactions with pool_id and type filters – partially tested (type=swap).

## Running Tests

```bash
# All documented features
cd schemas && go test -v -cover ./...
cd sdk/go && go test -v -cover ./...
cd services/indexer && go test -v -cover ./...
cd services/router && go test -v -cover ./...
cd contracts/dex/test && go test -v ./...
cd contracts/dex-router-v2/test && go test -v ./...
cd contracts/dex-router && go test -v ./...
```
