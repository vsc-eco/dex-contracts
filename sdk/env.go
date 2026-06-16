package sdk

//tinyjson:json
type EnvMap map[string]any

//tinyjson:json
type Env struct {
	ContractId string `json:"contract.id"`

	//About the calling tx & operation
	TxId    string `json:"tx.id"`
	Index   uint64 `json:"tx.index"`
	OpIndex uint64 `json:"tx.op_index"`

	//Block section
	BlockId     string `json:"block.id"`
	BlockHeight uint64 `json:"block.height"`
	Timestamp   string `json:"block.timestamp"`

	//Original creator of the transaction triggering this operation; Must be a user address
	Sender Sender `json:"sender"`
	//The address that is calling the contract; It can be contract address or user address
	Caller Address `json:"msg.caller"`

	//Who pays for the RC fee. Can be used in other contexts.
	//Proper RC payer support is not implemented yet.
	Payer Address `json:"msg.payer"`

	// EffectiveCaller is the identity the callee should treat as the
	// authorising actor for permission checks. Falls back to Caller when
	// no trusted forwarder has overridden it. The host sets this when a
	// contract in system-config.TrustedForwarders invokes us via
	// contracts.call_as. See go-vsc-node modules/wasm/sdk/sdk.go.
	//
	// Read this (or its convenience wrapper EffectiveCallerOrCaller) for
	// "who is the user" decisions, not Caller — that way the contract is
	// forwarder-aware and supports Dash IS-login (and any future similar
	// flow on LTC/BCH/DOGE).
	EffectiveCaller Address `json:"msg.effective_caller"`

	CallerIntents []Intent `json:"intents.caller"`
	SenderIntents []Intent `json:"intents.sender"`
}

// EffectiveCallerOrCaller returns EffectiveCaller if non-empty, otherwise
// falls back to Caller. Recommended for any "which user am I acting for"
// decision in a contract that wants forwarder-pattern support.
func (e *Env) EffectiveCallerOrCaller() Address {
	if e.EffectiveCaller != "" {
		return e.EffectiveCaller
	}
	return e.Caller
}
