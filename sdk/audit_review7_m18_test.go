package sdk

// review7 M18 — GetEnv must not panic on a malformed env. envAddrList is the
// comma-ok helper that replaced the bare type assertions; it must tolerate a
// missing key, a non-list value, and non-string elements (failing CLOSED to an
// empty auth set) instead of panicking the whole contract.

import "testing"

func TestAuditReview7_M18_EnvAddrListFailsClosed(t *testing.T) {
	// Well-formed list → parsed.
	got := envAddrList([]interface{}{"hive:alice", "hive:bob"})
	if len(got) != 2 || got[0] != "hive:alice" || got[1] != "hive:bob" {
		t.Fatalf("well-formed list mis-parsed: %v", got)
	}

	// Malformed inputs must yield an empty slice, NOT a panic.
	cases := []interface{}{
		nil,                                  // key absent
		"not-a-list",                         // wrong type
		123,                                  // wrong type
		[]interface{}{},                      // empty list
		[]interface{}{"ok", 123, nil, "ok2"}, // mixed: non-strings skipped
	}
	wantLen := []int{0, 0, 0, 0, 2}
	for i, c := range cases {
		out := envAddrList(c) // must not panic
		if len(out) != wantLen[i] {
			t.Errorf("case %d (%v): len=%d, want %d", i, c, len(out), wantLen[i])
		}
	}
}
