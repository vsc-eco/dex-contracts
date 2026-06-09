package sdk

type Intent struct {
	Type string            `json:"type"`
	Args map[string]string `json:"args"`
}

type Sender struct {
	Address              Address   `json:"id"`
	RequiredAuths        []Address `json:"required_auths"`
	RequiredPostingAuths []Address `json:"required_posting_auths"`
}

//tinyjson:json
type ContractCallOptions struct {
	Intents []Intent `json:"intents,omitempty"`
	// Try opts this call into try/catch mode: a reverting callee no longer traps
	// the caller (its state/ledger writes are rolled back to a savepoint and the
	// caller gets a structured outcome). Requires consensus version >= 0.2.0;
	// ignored below it. See sdk.TryContractCall.
	Try bool `json:"try,omitempty"`
}

type AddressDomain string

const (
	AddressDomainUser     AddressDomain = "user"
	AddressDomainContract AddressDomain = "contract"
	AddressDomainSystem   AddressDomain = "system"
)

type AddressType string

const (
	AddressTypeEVM      AddressType = "evm"
	AddressTypeKey      AddressType = "key"
	AddressTypeHive     AddressType = "hive"
	AddressTypeSystem   AddressType = "system"
	AddressTypeBLS      AddressType = "bls"
	AddressTypeContract AddressType = "contract"
	AddressTypeUnknown  AddressType = "unknown"
)

type Address string

func (a Address) String() string {
	return string(a)
}
