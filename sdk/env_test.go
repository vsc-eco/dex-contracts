package sdk

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Round-2 audit TC2-14: regression guard for the EffectiveCaller
// fallback semantics. The four dex-router-v2 sites that switched from
// env.Caller to env.EffectiveCallerOrCaller() rely on this exact
// fall-through behavior to preserve the direct-call path.
//
// These tests have no forwarder-mediated WASM harness path — they
// pin the env-level semantics so a refactor that breaks the
// fall-through (e.g. accidentally checking the empty-string case
// wrong) fails loudly.

func TestEnv_EffectiveCallerOrCaller_PrefersEffectiveWhenSet(t *testing.T) {
	env := &Env{
		Caller:          "hive:original-caller",
		EffectiveCaller: "did:pkh:bip122:abc:DashAddr",
	}
	assert.Equal(t, Address("did:pkh:bip122:abc:DashAddr"), env.EffectiveCallerOrCaller(),
		"forwarder-mediated path: EffectiveCaller wins")
}

func TestEnv_EffectiveCallerOrCaller_FallsBackWhenEffectiveEmpty(t *testing.T) {
	env := &Env{
		Caller:          "hive:original-caller",
		EffectiveCaller: "",
	}
	assert.Equal(t, Address("hive:original-caller"), env.EffectiveCallerOrCaller(),
		"direct-call path: empty EffectiveCaller falls back to Caller")
}

func TestEnv_EffectiveCallerOrCaller_BothEmptyReturnsEmpty(t *testing.T) {
	env := &Env{Caller: "", EffectiveCaller: ""}
	assert.Equal(t, Address(""), env.EffectiveCallerOrCaller(),
		"both-empty edge case: returns empty (caller would have rejected at framework level)")
}
