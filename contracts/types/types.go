package types

import "github.com/vsc-eco/dex-contracts/sdk"

// DEX OPERATIONS

//tinyjson:json
type InitParams struct {
	Asset0                string `json:"asset0"`
	Asset1                string `json:"asset1"`
	FeeBps                uint64 `json:"fee_bps"`
	Asset0MappingContract string `json:"asset0_mapping_contract,omitempty"`
	Asset1MappingContract string `json:"asset1_mapping_contract,omitempty"`
	// RouterContract is the authorized router that can set PreDeposited flags.
	// If empty, PreDeposited is rejected for all callers.
	RouterContract string `json:"router_contract,omitempty"`
}

//tinyjson:json
type SwapParams struct {
	AssetIn      string  `json:"asset_in"`
	AmountIn     string  `json:"amount_in"`
	AssetOut     string  `json:"asset_out"`
	MinAmountOut *string `json:"min_amount_out,omitempty"`
	From         string  `json:"from,omitempty"`
	To           string  `json:"to"`
	Beneficiary  *string `json:"beneficiary,omitempty"`
	RefBps       *uint64 `json:"ref_bps,omitempty"`
	// PreDeposited indicates that mapped input tokens have already been
	// transferred into the pool by the router via ERC-20 allowances.
	// When true, the pool skips DrawAssetFrom for mapped assets.
	PreDeposited bool `json:"pre_deposited,omitempty"`
}

//tinyjson:json
type AddLiquidityParams struct {
	Amount0   string `json:"amount0"`
	Amount1   string `json:"amount1"`
	Recipient string `json:"recipient"`
	// MinLpOut is the minimum acceptable LP token amount to mint (slippage
	// protection). When empty/absent, no minimum is enforced (backward compat).
	MinLpOut string `json:"min_lp_out,omitempty"`
	// Per-asset pre-deposit flags. When true, the corresponding asset was
	// already transferred into the pool by the router via ERC-20 allowances.
	PreDeposited0 bool `json:"pre_deposited_0,omitempty"`
	PreDeposited1 bool `json:"pre_deposited_1,omitempty"`
}

//tinyjson:json
type RemoveLiquidityParams struct {
	LpAmount  string `json:"lp_amount"`
	Recipient string `json:"recipient"`
}

//tinyjson:json
type SwapResult struct {
	AmountOut string   `json:"amount_out"`
	PoolState PoolInfo `json:"pool_state"` // Current pool state after swap
}

// PendulumSwapFeeInput is the JSON payload for system.pendulum_apply_swap_fees.
// All numeric amounts are decimal strings; one side must be "hbd".
//
//tinyjson:json
type PendulumSwapFeeInput struct {
	AssetIn  string `json:"asset_in"`
	AssetOut string `json:"asset_out"`
	X        string `json:"x"`
	XReserve string `json:"x_reserve"`
	YReserve string `json:"y_reserve"`
}

// PendulumSwapFeeOutput is the JSON response from system.pendulum_apply_swap_fees.
// `user_output`, `new_x_reserve`, `new_y_reserve`, `node_bucket_credited_hbd`,
// `network_credit_output`, `lp_share_output` and `node_share_output` are
// decimal-string base-unit amounts; `multiplier_bps` and `s_after_bps` are
// basis-point (1e4-scaled) values.
//
// The output-side fee components reconcile as
// `lp_share_output + node_share_output + network_credit_output == grossOut - user_output`.
// `node_share_output` is the node cut in output-asset units (pre-conversion);
// `node_bucket_credited_hbd` is the HBD actually moved to pendulum:nodes after
// the secondary-hop conversion (equal to node_share_output only for HBD-out swaps).
//
//tinyjson:json
type PendulumSwapFeeOutput struct {
	UserOutput          string `json:"user_output"`
	NewXReserve         string `json:"new_x_reserve"`
	NewYReserve         string `json:"new_y_reserve"`
	MultiplierBps       string `json:"multiplier_bps"`
	SAfterBps           string `json:"s_after_bps"`
	NetworkCreditOutput string `json:"network_credit_output"`
	LpShareOutput       string `json:"lp_share_output"`
	NodeShareOutput     string `json:"node_share_output"`
	// HBD amount deducted for the node runners. Equal to node_share_output
	// when swap output is HBD, converted back through the pool via a second
	// swap (calculated internally within system.pendulum_apply_swap_fees)
	// when output asset is not HBD
	NodeBucketCreditedHbd string `json:"node_bucket_credited_hbd"`
}

// ROUTER OPERATIONS

//tinyjson:json
type RegisterPoolParams struct {
	Asset0        string `json:"asset0"`
	Asset1        string `json:"asset1"`
	DexContractId string `json:"dex_contract_id"`
}

//tinyjson:json
type PoolInfo struct {
	Asset0   string `json:"asset0"`
	Asset1   string `json:"asset1"`
	Reserve0 string `json:"reserve0"`
	Reserve1 string `json:"reserve1"`
	Fee      uint64 `json:"fee"`
	TotalLp  string `json:"total_lp"`
}

//tinyjson:json
type TokenInfo struct {
	MappingContract string `json:"mapping_contract,omitempty"`
	Chain           string `json:"chain"` // HIVE, MAGI, BTC, ETH, etc.
	Description     string `json:"description,omitempty"`
	Decimals        int    `json:"decimals"`
}

// MappingContractInfoReturn is the response from calling getInfo on a mapping contract.
//
//tinyjson:json
type MappingContractInfoReturn struct {
	Name     string `json:"name"`
	Symbol   string `json:"symbol"`
	Decimals string `json:"decimals"`
}

//tinyjson:json
type RegisterTokenParams struct {
	Name string `json:"name"`
	TokenInfo
}

//tinyjson:json
type GetPoolParams struct {
	Asset0 string `json:"asset0"`
	Asset1 string `json:"asset1"`
}

//tinyjson:json
type ReturnAddress struct {
	Chain   string `json:"chain"`
	Address string `json:"address"`
}

// ROUTER TYPES
//
// DexInstruction is a union type dispatched on the Type field:
//
//   type "swap":       uses AssetIn, AssetOut, AmountIn (directional)
//   type "deposit":    uses Asset0, Asset1, Amount0, Amount1 (alphabetical pool order)
//   type "withdrawal": uses Asset0, Asset1, LpAmount (alphabetical pool order)
//
// Asset0/Asset1 always follow alphabetical ordering, matching the pool's
// canonical storage order. The router normalizes user input automatically.

//tinyjson:json
type DexInstruction struct {
	Type    string `json:"type"`
	Version string `json:"version"`

	// --- Swap fields (type "swap") ---
	AssetIn      string  `json:"asset_in,omitempty"`
	AssetOut     string  `json:"asset_out,omitempty"`
	AmountIn     string  `json:"amount_in,omitempty"`
	MinAmountOut *string `json:"min_amount_out,omitempty"`
	Beneficiary  *string `json:"beneficiary,omitempty"`
	RefBps       *uint64 `json:"ref_bps,omitempty"`

	// --- Deposit/Withdrawal pool identifiers ---
	Asset0 string `json:"asset0,omitempty"`
	Asset1 string `json:"asset1,omitempty"`

	// --- Deposit fields (type "deposit") ---
	// Asset0/Asset1 are in alphabetical order, matching pool state.
	Amount0 string `json:"amount0,omitempty"`
	Amount1 string `json:"amount1,omitempty"`
	// MinLpOut is the minimum acceptable LP token amount to mint (slippage
	// protection). Forwarded into AddLiquidityParams.MinLpOut. When
	// empty/absent, no minimum is enforced (backward compat).
	MinLpOut string `json:"min_lp_out,omitempty"`

	// --- Withdrawal fields (type "withdrawal") ---
	LpAmount string `json:"lp_amount,omitempty"`

	// --- Common fields ---
	Recipient     string            `json:"recipient"`
	ReturnAddress *ReturnAddress    `json:"return_address,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	// DestinationChain specifies an external chain for settlement (e.g. "HIVE", "BTC").
	// When set to a non-MAGI chain, the router bridges swap output to the Recipient
	// address on that chain instead of settling on Magi.
	DestinationChain string `json:"destination_chain,omitempty"`
	// MaxFee caps the unmap fee charged when settling a *mapped* asset (BTC,
	// ETH, etc.) to its native chain: it is threaded into UnmapParams.MaxFee
	// so the mapping contract rejects the unmap if its fee exceeds this cap.
	// It does NOT apply to HIVE/HBD settlement, which uses sdk.HiveWithdraw
	// and charges no such fee — MaxFee is ignored on that path.
	// nil = unchanged behavior (no cap).
	MaxFee *int64 `json:"max_fee,omitempty"`
}

//tinyjson:json
type SchemaReturn struct {
	SupportedChains     []string              `json:"supported_chains"`
	ReturnAddressChains []string              `json:"return_address_chains"`
	Tokens              []RegisterTokenParams `json:"tokens"`
	Note                string                `json:"note"`
}

//tinyjson:json
type KeyList struct {
	Keys []string `json:"keys"`
}

//tinyjson:json
type KeyMap map[string]string

// type for the ENV that only queries the env if actually needed, saving gas if not
type MaybeEnv struct {
	env *sdk.Env
}

//tinyjson:json
type MappingContractInput struct {
	Amount string `json:"amount"`
	To     string `json:"to"`
	From   string `json:"from,omitempty"`
}

// UnmapParams is the input for the "unmap" action on mapping contracts,
// which sends mapped assets out to their native chain (e.g. BTC to Bitcoin mainnet).
//
//tinyjson:json
type UnmapParams struct {
	Amount    string `json:"amount"`
	To        string `json:"to"`
	From      string `json:"from,omitempty"`
	DeductFee bool   `json:"deduct_fee,omitempty"`
	MaxFee    *int64 `json:"max_fee,omitempty"`
}
