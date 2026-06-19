#!/usr/bin/env bash
# Audit M11 (MED 5.0 STRUCTURAL): refuse a release build that ships a
# local-path `replace` in go.mod. Without this gate, a release tag can
# bake in a sibling-checkout's working state for any of vsc-node,
# btc-mapping-contract, or dash-mapping-contract — the resulting wasm
# embeds unaudited code with no commit attribution.
#
# Operators iterating locally still use a gitignored go.work file to
# point modules at sibling checkouts — that's intentional + does not
# trip this gate (the regex only inspects go.mod).
#
# Wire into CI by invoking `./scripts/check-no-local-replace.sh` (or
# `make check-no-local-replace`) before any release-tag wasm build.

set -euo pipefail

if grep -nE '^[[:space:]]*replace[[:space:]].*=>[[:space:]]*(\.\.?/|/|~)' go.mod; then
    echo ""
    echo "ERROR: go.mod contains a LOCAL-PATH replace directive."
    echo "Release builds MUST pin every dependency to a remote @<commit>."
    echo "If you're iterating locally, use a gitignored go.work instead."
    exit 1
fi
echo "go.mod is clean — no local-path replaces."

# If the dependency pins resolve cleanly, the contract build should
# produce all wasm targets without manual intervention. This catches
# the case where a local replace was removed but the remote pin
# wasn't bumped (vendored state drift).
go mod verify
echo "go mod verify: clean."
