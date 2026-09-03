#!/usr/bin/env bash
# Point every registered repo at the sister repo that holds the shared modules.
#
# The modules used to be embedded in the binary, so a manifest could name no
# source at all. They live in their own repository now, which every managed
# repo reaches through a `sources:` block. This writes that block — one entry
# named agents-md, carrying the path or the git URL it was given and no ref —
# and drops any retired -brief module entry left over from the index migration.
# Nothing else in the file is touched: no reformatting, no reordering, no
# rewrite of the rest of the YAML, so a manifest keeps whatever shape its owner
# gave it.
#
# Usage:
#   migrate-sources.sh --path <dir>   # a local checkout, read directly
#   migrate-sources.sh --git <url>    # the pushed repository
#
# No `ref:` is written either way: `agents sync` fetches a git source the cache
# lacks and `agents update` is what pins it to a sha.
#
# The sister repo consumes itself with `path: .`, so a manifest whose agents-md
# entry says exactly that is left alone.
#
# Idempotent: a repo already carrying the exact entry prints nothing. Reads the
# registry the tool reads ($AGENTS_REGISTRY, else ~/.config/agents/repos), one
# absolute path per line. Set AGENTS_REGISTRY to a fake registry to test it.
set -euo pipefail

source_name="agents-md"

usage() {
	echo "usage: migrate-sources.sh --path <dir> | --git <url>"
}

key=""
value=""
while [[ $# -gt 0 ]]; do
	case "$1" in
	--path | --git)
		if [[ -n $key ]]; then
			echo "migrate-sources: --path and --git are exclusive" >&2
			exit 2
		fi
		key="${1#--}"
		value="${2:-}"
		shift 2 || true
		;;
	-h | --help)
		usage
		exit 0
		;;
	*)
		echo "migrate-sources: unknown argument $1" >&2
		usage >&2
		exit 2
		;;
	esac
done
if [[ -z $key || -z $value ]]; then
	usage >&2
	exit 2
fi

registry="${AGENTS_REGISTRY:-$HOME/.config/agents/repos}"
if [[ ! -f $registry ]]; then
	echo "migrate-sources: no registry at $registry" >&2
	exit 1
fi

# A manifest entry for a module retired by the situational index: "  - <name>-brief".
brief_entry='^[[:space:]]*-[[:space:]]+[a-z0-9-]+-brief[[:space:]]*$'

# rewrite reads a manifest on stdin and writes the migrated one on stdout,
# reporting through its exit status: 0 rewrote something, 1 nothing to do,
# 2 the repo is the source itself (path: .), 3 the manifest has no modules:
# block to anchor a new sources: block above.
read -r -d '' rewrite <<'AWK' || true
function is_entry(s) { return s ~ /^[[:space:]]*-[[:space:]]/ }

{ line[NR] = $0 }

END {
	name = ENVIRON["source_name"]
	want[1] = "  - name: " name
	want[2] = "    " ENVIRON["key"] ": " ENVIRON["value"]

	# The sources: block runs from its header to the last indented line under it.
	for (i = 1; i <= NR; i++) {
		if (line[i] ~ /^sources:[[:space:]]*$/) { block = i; break }
	}
	blockEnd = block
	for (i = block + 1; block && i <= NR; i++) {
		if (line[i] !~ /^[[:space:]]/ || line[i] ~ /^[[:space:]]*$/) break
		blockEnd = i
	}

	# The entry named agents-md inside it, from its "- " line to the line
	# before the next entry.
	for (i = block + 1; block && i <= blockEnd; i++) {
		if (is_entry(line[i])) at = i
		if (line[i] ~ ("name:[[:space:]]*" name "[[:space:]]*$")) { entry = at; break }
	}
	if (entry) {
		entryEnd = blockEnd
		for (i = entry + 1; i <= blockEnd; i++) {
			if (is_entry(line[i])) { entryEnd = i - 1; break }
		}
		# The source repo consumes itself; that entry is not ours to rewrite.
		for (i = entry; i <= entryEnd; i++) {
			if (line[i] ~ /^[[:space:]]*path:[[:space:]]*\.[[:space:]]*$/) exit 2
		}
	}

	for (i = 1; i <= NR; i++) {
		if (line[i] ~ /^modules:/) { anchor = i; break }
	}
	if (!block && !anchor) exit 3

	for (i = 1; i <= NR; i++) {
		if (!block && i == anchor) {
			out[++n] = "sources:"
			out[++n] = want[1]
			out[++n] = want[2]
		}
		if (entry && i >= entry && i <= entryEnd) {
			if (i == entry) { out[++n] = want[1]; out[++n] = want[2] }
			continue
		}
		if (line[i] ~ ENVIRON["brief_entry"]) continue
		out[++n] = line[i]
		# A block naming other sources but not this one gains it at the top:
		# the first source listed is the default, and a bare module name in
		# the manifest means the default source.
		if (block && !entry && i == block) { out[++n] = want[1]; out[++n] = want[2] }
	}

	changed = (n != NR)
	for (i = 1; !changed && i <= n; i++) {
		if (out[i] != line[i]) changed = 1
	}
	for (i = 1; i <= n; i++) print out[i]
	exit (changed ? 0 : 1)
}
AWK

status=0
while IFS= read -r repo; do
	if [[ -z $repo || $repo == \#* ]]; then
		continue
	fi
	manifest="$repo/.agents.yaml"
	if [[ ! -f $manifest ]]; then
		echo "migrate-sources: no manifest at $manifest" >&2
		status=1
		continue
	fi

	# grep exits 1 on no match, which pipefail would turn into a script exit.
	dropped=$({ grep -Eo "$brief_entry" "$manifest" || true; } |
		sed -E 's/^[[:space:]]*-[[:space:]]+//; s/[[:space:]]*$//' |
		tr '\n' ' ' | sed -E 's/ +$//')

	tmp="$manifest.migrate-sources.$$"
	rc=0
	source_name="$source_name" key="$key" value="$value" brief_entry="$brief_entry" \
		awk "$rewrite" "$manifest" >"$tmp" || rc=$?
	case "$rc" in
	0)
		mv "$tmp" "$manifest"
		echo "$repo: sources -> $source_name ($value)"
		if [[ -n $dropped ]]; then
			echo "$repo: dropped $dropped"
		fi
		;;
	1 | 2)
		# 1: already migrated. 2: the source repo itself, which consumes
		# itself with `path: .` and must keep it.
		rm -f "$tmp"
		;;
	3)
		rm -f "$tmp"
		echo "migrate-sources: $manifest has no modules: block" >&2
		status=1
		;;
	*)
		rm -f "$tmp"
		echo "migrate-sources: rewriting $manifest failed ($rc)" >&2
		status=1
		;;
	esac
done <"$registry"

exit "$status"
