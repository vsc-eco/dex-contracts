# Architecture

## Overview

The VSC DEX Mapping system enables Bitcoin UTXO mapping and DEX operations through external services that integrate with VSC via public APIs. This design ensures zero modifications to the core `go-vsc-node` while providing full cross-chain functionality.

## Components

### Contracts (`contracts/`)
TinyGo smart contracts deployed on VSC:

- **btc-mapping**: Handles Bitcoin header verification, SPV deposit proofs, and minting/burning of mapped BTC tokens

### Services (`services/`)
External services that connect via VSC's public APIs:

- **oracle**: Bitcoin node relay that submits block headers and verifies deposit proofs
- **router**: DEX route planner and transaction composer for optimal swap execution
- **indexer**: Event subscriber that builds read models for pools, tokens, and bridge operations

### SDK (`sdk/`)
Client libraries for integration:

- **go**: Go SDK for backend services and CLI tools
- **ts**: TypeScript SDK for frontend DEX applications

## Data Flow

### Deposit Flow (BTC → VSC)
1. User sends BTC to deposit address
2. Oracle monitors Bitcoin network and submits headers to `btc-mapping` contract
3. User generates SPV proof and calls `proveDeposit()` on contract
4. Contract verifies proof against accepted headers and mints mapped BTC tokens
5. Tokens appear in user's VSC balance, ready for DEX operations

### Swap Flow (BTC → HBD)
1. User requests BTC→HBD swap via frontend
2. Router service computes optimal route (direct pool or multi-hop)
3. Router composes contract call transaction
4. Transaction executes AMM swap on VSC DEX contracts
5. User receives HBD tokens

### Withdrawal Flow (VSC → BTC)
1. User requests BTC withdrawal, burning mapped tokens
2. Contract records withdrawal intent
3. Oracle monitors for burn events and facilitates BTC payout
4. User receives BTC on target address

## Security Considerations

- **SPV Verification**: All deposits require Bitcoin SPV proofs verified against rolling header window
- **Confirmation Requirements**: Minimum 6 BTC confirmations before deposits are accepted
- **Contract Ownership**: Mapped tokens controlled by mapping contract, preventing unauthorized minting
- **Oracle Independence**: Multiple oracles can operate for redundancy and verification

## Future Extensibility

The architecture supports additional chains through:
- New mapping contracts following the same SPV/verification pattern
- Additional oracle services for different blockchains
- Router service automatically discovers new mapped tokens
- Unified SDK interface across all supported chains



