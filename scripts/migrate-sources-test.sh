#!/usr/bin/env bash
# Tests migrate-sources.sh against a fake registry of fixture repos in a temp
# directory. It never reads the real registry: AGENTS_REGISTRY is set on every
# call. Run it before pointing the script at the fleet.
#
#   scripts/migrate-sources-test.sh
set -uo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
script="$here/migrate-sources.sh"
# An explicit template, because macOS mktemp ignores $TMPDIR without one.
work="$(mktemp -d "${TMPDIR:-/tmp}/migrate-sources-test.XXXXXX")" || exit 1
trap 'rm -rf "$work"' EXIT

sister="/checkout/agents-md"
url="https://example.invalid/agents-md.git"
fails=0

fail() {
	echo "FAIL: $*"
	fails=$((fails + 1))
}

# want_file compares a manifest against the content it should now hold.
want_file() {
	local label="$1" path="$2" want="$3" got
	got="$(cat "$path")"
	if [[ $got != "$want" ]]; then
		fail "$label"
		printf -- '--- want ---\n%s\n--- got ---\n%s\n' "$want" "$got"
	fi
}

want_out() {
	local label="$1" got="$2" want="$3"
	if [[ $got != "$want" ]]; then
		fail "$label output"
		printf -- '--- want ---\n%s\n--- got ---\n%s\n' "$want" "$got"
	fi
}

mkrepo() {
	mkdir -p "$work/$1"
	printf '%s' "$2" >"$work/$1/.agents.yaml"
}

# --- fixtures ----------------------------------------------------------------

# No sources block at all: what every repo looked like before this migration.
mkrepo none 'modules:
  - core
  - principles
'

# An agents-md entry pointing somewhere else, and a second source that must
# survive untouched — the wave 6 path -> git switch runs over this shape.
mkrepo pathy 'sources:
  - name: agents-md
    path: /somewhere/else
  - name: other
    git: https://example.invalid/other.git
    ref: 1111111111111111111111111111111111111111
modules:
  - core
'

# The source repo itself, which consumes itself: never rewritten.
sister_manifest='sources:
  - name: agents-md
    path: .
modules:
  - core
  - principles
  - stage-build
'
mkrepo sister "$sister_manifest"

# A manifest still listing a module retired by the situational index.
mkrepo briefy 'modules:
  - core
  - jobs-brief
  - jobs
'

registry="$work/repos"
for name in none pathy sister briefy; do echo "$work/$name" >>"$registry"; done

# --- first pass: --path ------------------------------------------------------

out="$(AGENTS_REGISTRY=$registry "$script" --path "$sister" 2>&1)"
rc=$?
[[ $rc -eq 0 ]] || fail "first pass exited $rc"

want_out "first pass" "$out" "$work/none: sources -> agents-md ($sister)
$work/pathy: sources -> agents-md ($sister)
$work/briefy: sources -> agents-md ($sister)
$work/briefy: dropped jobs-brief"

want_file "no sources block" "$work/none/.agents.yaml" "sources:
  - name: agents-md
    path: $sister
modules:
  - core
  - principles"

want_file "existing entry replaced, other source kept" "$work/pathy/.agents.yaml" "sources:
  - name: agents-md
    path: $sister
  - name: other
    git: https://example.invalid/other.git
    ref: 1111111111111111111111111111111111111111
modules:
  - core"

want_file "the source repo itself" "$work/sister/.agents.yaml" "${sister_manifest%$'\n'}"

want_file "brief entry dropped" "$work/briefy/.agents.yaml" "sources:
  - name: agents-md
    path: $sister
modules:
  - core
  - jobs"

# --- second pass: idempotent -------------------------------------------------

before="$(cat "$work"/*/.agents.yaml)"
out="$(AGENTS_REGISTRY=$registry "$script" --path "$sister" 2>&1)"
rc=$?
[[ $rc -eq 0 ]] || fail "second pass exited $rc"
want_out "second pass (idempotent)" "$out" ""
if [[ "$(cat "$work"/*/.agents.yaml)" != "$before" ]]; then
	fail "second pass rewrote a manifest"
fi

# --- third pass: path -> git, the wave 6 switch ------------------------------

out="$(AGENTS_REGISTRY=$registry "$script" --git "$url" 2>&1)"
rc=$?
[[ $rc -eq 0 ]] || fail "git pass exited $rc"
want_out "git pass" "$out" "$work/none: sources -> agents-md ($url)
$work/pathy: sources -> agents-md ($url)
$work/briefy: sources -> agents-md ($url)"

want_file "path switched to git, with no ref" "$work/none/.agents.yaml" "sources:
  - name: agents-md
    git: $url
modules:
  - core
  - principles"

want_file "the source repo itself, again" "$work/sister/.agents.yaml" "${sister_manifest%$'\n'}"

# --- refusals ----------------------------------------------------------------

AGENTS_REGISTRY=$registry "$script" >/dev/null 2>&1 && fail "no arguments exited 0"
AGENTS_REGISTRY=$registry "$script" --path a --git b >/dev/null 2>&1 && fail "--path with --git exited 0"
AGENTS_REGISTRY=$work/nope "$script" --path "$sister" >/dev/null 2>&1 && fail "a missing registry exited 0"

if [[ $fails -eq 0 ]]; then
	echo "PASS"
else
	echo "$fails failure(s)"
fi
exit "$fails"
