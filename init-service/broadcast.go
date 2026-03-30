package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/vsc-eco/hivego"
)

type txVscCallContractJSON struct {
	NetId      string          `json:"net_id"`
	Caller     string          `json:"caller"`
	ContractId string          `json:"contract_id"`
	Action     string          `json:"action"`
	Payload    json.RawMessage `json:"payload"`
	RcLimit    uint            `json:"rc_limit"`
	Intents    []interface{}   `json:"intents"`
}

type Broadcaster struct {
	HiveURIs  []string
	ChainID   string
	NetID     string
	Username  string
	ActiveKey string
}

// CallContract broadcasts a vsc.call transaction and returns the transaction ID.
func (b *Broadcaster) CallContract(contractID string, action string, payload json.RawMessage) (string, error) {
	client := hivego.NewHiveRpc(b.HiveURIs)
	client.ChainID = b.ChainID

	wrapper := txVscCallContractJSON{
		NetId:      b.NetID,
		Caller:     fmt.Sprintf("hive:%s", b.Username),
		ContractId: contractID,
		Action:     action,
		Payload:    payload,
		RcLimit:    10000,
		Intents:    []any{},
	}

	txJson, err := json.Marshal(wrapper)
	if err != nil {
		return "", fmt.Errorf("marshalling tx json: %w", err)
	}

	log.Printf("broadcasting %s on %s: %s", action, contractID, string(txJson))

	txId, err := client.BroadcastJson(
		[]string{b.Username}, []string{}, "vsc.call", string(txJson), &b.ActiveKey,
	)
	if err != nil {
		return "", fmt.Errorf("broadcasting tx: %w", err)
	}

	log.Printf("broadcast ok, tx id: %s", txId)
	return txId, nil
}
