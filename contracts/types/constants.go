package types

// Protocol-level intent constants for native asset draws (HIVE/HBD).
// Mapped assets (BTC, ETH, etc.) use ERC-20 allowances instead.
const (
	IntentTransferType = "transfer.allow"
	IntentAmountKey    = "limit"
	IntentTokenKey     = "token"
)

const (
	KeyAsset0       = "a1"
	KeyAsset1       = "a2"
	KeyReserve0     = "r0"
	KeyReserve1     = "r1"
	KeyFee          = "fee"
	KeyTotalLP      = "tlp"
	KeyLpPrefix     = "lp" + DirPathDelimiter // lp-{address} or lp/{address}
	KeySystemFee0   = "f0"
	KeySystemFee1   = "f1"
	KeyFeeLastClaim = "flc"
	// KeyRouterAssetPrefix            = "as-"
	// KeyMappingContractBalancePrefix = "bal-"
)

const DirPathDelimiter = "-"
