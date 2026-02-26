# DEX Mapper Instruction Schema

This document describes the JSON schema for DEX mapper instructions used by external bots to interact with the VSC DEX router service.

## Overview

The DEX mapper instruction schema defines a standardized format for specifying cross-chain swap operations. Instructions can be embedded in transfer memos or sent directly to the router API.

## Schema Definition

The instruction schema uses snake_case field names and follows JSON Schema Draft 07.

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "required": ["type", "version", "asset_in", "asset_out", "recipient"],
  "properties": {
    "type": { "type": "string", "enum": ["swap", "deposit", "withdrawal"] },
    "version": { "type": "string", "pattern": "^\\d+\\.\\d+\\.\\d+$" },
    "asset_in": { "type": "string" },
    "asset_out": { "type": "string" },
    "recipient": { "type": "string" },
    "slippage_bps": { "type": "integer", "minimum": 0, "maximum": 10000 },
    "min_amount_out": { "type": "integer", "minimum": 0 },
    "beneficiary": { "type": "string" },
    "ref_bps": { "type": "integer", "minimum": 0, "maximum": 10000 },
    "return_address": {
      "type": "object",
      "properties": {
        "chain": { "type": "string" },
        "address": { "type": "string" }
      },
      "required": ["chain", "address"]
    },
    "metadata": { "type": "object" }
  }
}
```

## Field Descriptions

### Required Fields

- **`type`** (string): Instruction type. Currently only `"swap"` is supported.
- **`version`** (string): Schema version in semver format (e.g., `"1.0.0"`).
- **`asset_in`** (string): Source asset symbol (e.g., `"BTC"`, `"ETH"`).
- **`asset_out`** (string): Destination VSC asset (e.g., `"HBD"`, `"HBD_SAVINGS"`, `"HIVE"`).
- **`recipient`** (string): VSC account to receive the output tokens.

### Optional Fields

- **`slippage_bps`** (integer): Maximum slippage in basis points (0-10000). Default: `50` (0.5%).
- **`min_amount_out`** (integer): Minimum output amount in smallest unit. Default: `0`.
- **`beneficiary`** (string): Referral beneficiary VSC account.
- **`ref_bps`** (integer): Referral fee in basis points (0-10000, 0.01%-10%).
- **`return_address`** (object): Return address for refunds in case of failure.
  - **`chain`** (string): Blockchain for the return address (e.g., "BTC", "ETH", "SOL")
  - **`address`** (string): Address on the specified chain
- **`metadata`** (object): Additional metadata for extensibility.

## Usage Methods

### 1. Transfer Memo (Recommended)

Embed instructions in transfer memos when depositing funds:

**JSON Format:**

```json
{
  "type": "swap",
  "version": "1.0.0",
  "asset_in": "BTC",
  "asset_out": "HBD",
  "recipient": "user123",
  "slippage_bps": 100
}
```

**URL Query Format:**

```
type=swap&version=1.0.0&asset_in=BTC&asset_out=HBD&recipient=user123&slippage_bps=100
```

**URL Query with Return Address:**

```
type=swap&version=1.0.0&asset_in=BTC&asset_out=HBD&recipient=user123&return_address.chain=ETH&return_address.address=0x123...
```

The system automatically detects the format and parses accordingly.

### 2. Custom JSON Operations

For on-chain instructions, use VSC custom_json operations:

```json
{
  "required_auths": ["user123"],
  "required_posting_auths": [],
  "id": "vsc.dex_swap",
  "json": "{\"type\":\"swap\",\"version\":\"1.0.0\",\"asset_in\":\"BTC\",\"asset_out\":\"HBD\",\"recipient\":\"user123\"}"
}
```

### 3. Direct Router API

Send instructions directly to the router service:

```bash
curl -X POST http://router-service:8080/api/v1/instruction \
  -H "Content-Type: application/json" \
  -d '{
    "instruction": {"type":"swap","version":"1.0.0","asset_in":"BTC","asset_out":"HBD","recipient":"user123"},
    "amountIn": 1000000
  }'
```

## Examples

### Basic BTC to HBD Swap

```json
{
  "type": "swap",
  "version": "1.0.0",
  "asset_in": "BTC",
  "asset_out": "HBD",
  "recipient": "alice"
}
```

### BTC to HBD Savings with Slippage Protection

```json
{
  "type": "swap",
  "version": "1.0.0",
  "asset_in": "BTC",
  "asset_out": "HBD_SAVINGS",
  "recipient": "alice",
  "slippage_bps": 200,
  "min_amount_out": 50000
}
```

### Swap with Referral

```json
{
  "type": "swap",
  "version": "1.0.0",
  "asset_in": "BTC",
  "asset_out": "HIVE",
  "recipient": "alice",
  "beneficiary": "referrer",
  "ref_bps": 500
}
```

### Swap with Return Address

```json
{
  "type": "swap",
  "version": "1.0.0",
  "asset_in": "ETH",
  "asset_out": "HBD",
  "recipient": "alice",
  "return_address": {
    "chain": "ETH",
    "address": "0x1234567890abcdef..."
  },
  "metadata": {
    "notes": "Test transaction with refund protection"
  }
}
```

## Validation Rules

- All required fields must be present
- `version` must follow semver format (x.y.z)
- `slippage_bps` and `ref_bps` must be between 0 and 10000
- `min_amount_out` must be non-negative
- `recipient` and `beneficiary` should be valid VSC account names
- `return_address` should be a valid address for the source chain

## Error Handling

The system provides detailed error messages for validation failures:

- `"type is required"`: Missing required field
- `"version is required"`: Missing required field
- `"schema validation failed: ..."` : JSON schema validation errors
- `"Invalid slippage_bps value"`: Value outside allowed range

## Versioning

- Schema versions follow semantic versioning
- Breaking changes will increment the major version
- New optional fields may be added in minor versions
- External bots should validate the version field

## Integration Guide

### For Transfer Memo Integration

1. Construct instruction JSON or URL query string
2. Embed in transfer memo field
3. User deposits to the appropriate address
4. System automatically processes the swap

### For Custom JSON Integration

1. Construct instruction JSON
2. Wrap in custom_json operation with `id: "vsc.dex_swap"`
3. Broadcast transaction to VSC network
4. System processes the swap on-chain

### For Direct API Integration

1. Construct instruction JSON
2. POST to `/api/v1/instruction` endpoint with `amountIn`
3. Receive swap result
4. Handle success/failure responses

## Supported Assets and Chains

### Dynamic Schema Generation

The instruction schema is **dynamically generated** based on registered DEX pools. Chains are automatically added to the `return_address.chain` enum when pools are registered.

**Get Current Schema:**

```bash
curl http://localhost:8081/api/v1/schema
```

### Asset Registration

Assets and their chains are determined from:

1. **Mapping Contracts** (utxo-mapping pattern): Cross-chain assets (BTC, ETH, SOL, etc.) are registered via mapping contracts
2. **Token Registry**: VSC native tokens are registered with `ContractId = nil` → automatically recognized as chain "HIVE"
3. **Pool Registration**: When pools are registered, router queries token registry to determine chains
4. **Future-proof**: New VSC native tokens work automatically when registered in token registry with `ContractId = nil`

### Return Address Chains

The `return_address.chain` enum is **dynamically generated** from registered pools. When a new chain is added (e.g., via utxo-mapping contract), it automatically appears in the schema after pools using that chain are registered.

**Example**: When a SUI mapping contract is deployed and a SUI/HBD pool is registered, "SUI" automatically appears in the `return_address.chain` enum.

## Security Considerations

- Always validate instructions before processing
- Use reasonable slippage limits to prevent sandwich attacks
- Implement rate limiting for API endpoints
- Validate recipient addresses to prevent fund loss
- Consider implementing instruction expiration for security
