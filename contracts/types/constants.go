package types

// Protocol-level intent constants for native asset draws (HIVE/HBD).
// Mapped assets (BTC, ETH, etc.) use ERC-20 allowances instead.
const (
	IntentTransferType = "transfer.allow"
	IntentAmountKey    = "limit"
	IntentTokenKey     = "token"
)

// Keys for pool state storage
const (
	KeyAsset0Name    = "a0n" // asset0 lowercase name
	KeyAsset0Mapping = "a0m" // asset0 mapping contract (empty for native)
	KeyAsset1Name    = "a1n" // asset1 lowercase name
	KeyAsset1Mapping = "a1m" // asset1 mapping contract (empty for native)
	KeyReserve0      = "r0"
	KeyReserve1      = "r1"
	KeyFee           = "fee"
	KeyTotalLP       = "tlp"
	KeyLpPrefix      = "lp" + DirPathDelimiter // lp-{address}
	KeySystemFee0    = "f0"
	KeySystemFee1    = "f1"
	KeyFeeLastClaim  = "flc"
	KeyRouter        = "rtr" // authorized router contract ID
	KeyMigrateVer    = "mv"  // migration version tracker
)

const DirPathDelimiter = "-"

// Log formatting
const (
	LogDelimiter    = "|"
	LogKeyDelimiter = "="
)

const (
	DefaultBaseFeeBps = 8 // 0.08%
)
