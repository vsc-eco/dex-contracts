# Test Coverage Summary

## Test Results

All tests passing ✅

### Indexer Service
- **Coverage**: 52.3% of statements
- **Tests**: 30+ test cases
- **New Tests Added**:
  - `TestDexReadModel_HandleEvent_RegisterPool_WithChains` - Tests pool registration with chain info
  - `TestDexReadModel_HandleEvent_RegisterPool_WithoutChains` - Tests pool registration without chain info
  - `TestDexReadModel_GetSchema` - Tests dynamic schema generation
  - `TestDexReadModel_GetSchema_Empty` - Tests schema with no registered pools
  - `TestDexReadModel_registerAsset` - Tests asset registration
  - `TestDexReadModel_getAssetChain` - Tests chain determination
  - `TestServer_handleGetSchema` - Tests schema API endpoint
  - `TestServer_handleGetSchema_NoReader` - Tests error handling

### Router Service
- **Coverage**: 41.4% of statements
- **Tests**: 15+ test cases
- All existing tests passing

### Schemas
- **Coverage**: 76.6% of statements
- **Tests**: 4 test suites with multiple sub-tests
- All tests passing

### Router-V2 Contract Tests
- **Tests**: 3 test suites
  - `TestPoolRegistration` - Pool registration and normalization
  - `TestFindPool` - Pool finding logic
  - `TestTwoHopRouting` - Two-hop routing logic
- All tests passing

## Coverage Gaps

### Areas Needing More Tests

1. **Two-Hop Swap Failure Handling**
   - Return address chain matching
   - Swap-back to original asset
   - Cross-chain return handling

2. **Asset Chain Registration**
   - Token registry integration
   - Mapping contract integration
   - Chain determination logic

3. **Schema Generation Edge Cases**
   - Empty chains list
   - Unknown assets
   - Chain conflicts

## Test Execution

Run all tests:
```bash
# Indexer tests
cd services/indexer && go test -v -cover ./...

# Router tests
cd services/router && go test -v -cover ./...

# Schema tests
cd schemas && go test -v -cover ./...

# Contract tests
cd contracts/dex-router-v2/test && go test -v ./...
```

## Future Test Additions

1. Integration tests for two-hop swap failures
2. E2E tests for dynamic schema updates
3. Tests for return address chain validation
4. Tests for asset chain registration flow
