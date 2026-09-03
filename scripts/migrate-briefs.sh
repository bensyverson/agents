#!/usr/bin/env bash
# Drop the retired -brief modules from every registered repo's manifest.
#
# The five -brief modules existed only to point at a kind: file module; the
# generated `index` region does that now, so a manifest that still lists one
# renders an unknown-module error. This removes those lines and nothing else —
# no reformatting, no reordering, no rewrite of the rest of the YAML — so a
# repo's manifest keeps whatever shape its owner gave it.
#
# Idempotent: a second run prints nothing. Reads the registry the tool reads
# ($AGENTS_REGISTRY, else ~/.config/agents/repos), one absolute path per line.
# Set AGENTS_REGISTRY to a fake registry to test it.
set -euo pipefail

registry="${AGENTS_REGISTRY:-$HOME/.config/agents/repos}"
if [[ ! -f $registry ]]; then
	echo "migrate-briefs: no registry at $registry" >&2
	exit 1
fi

# A manifest entry for a retired module: "  - <name>-brief", alone on its line.
brief_entry='^[[:space:]]*-[[:space:]]+[a-z0-9-]+-brief[[:space:]]*$'

status=0
while IFS= read -r repo; do
	if [[ -z $repo || $repo == \#* ]]; then
		continue
	fi
	manifest="$repo/.agents.yaml"
	if [[ ! -f $manifest ]]; then
		echo "migrate-briefs: no manifest at $manifest" >&2
		status=1
		continue
	fi

	# grep exits 1 on no match, which pipefail would turn into a script exit.
	dropped=$({ grep -Eo "$brief_entry" "$manifest" || true; } |
		sed -E 's/^[[:space:]]*-[[:space:]]+//; s/[[:space:]]*$//' |
		tr '\n' ' ' | sed -E 's/ +$//')
	if [[ -z $dropped ]]; then
		continue
	fi

	tmp="$manifest.migrate-briefs.$$"
	grep -Ev "$brief_entry" "$manifest" >"$tmp" || true
	mv "$tmp" "$manifest"
	echo "$repo: dropped $dropped"
done <"$registry"

exit "$status"
