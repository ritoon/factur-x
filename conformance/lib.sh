# Functions shared by fetch.sh and parity.sh (sourced, not run).

log() { echo "== $*" >&2; }

die() {
	echo "${0##*/}: $*" >&2
	exit 1
}

sha256() {
	if command -v sha256sum >/dev/null; then
		sha256sum "$1" | cut -d' ' -f1
	else
		shasum -a 256 "$1" | cut -d' ' -f1
	fi
}

# fetch_git dir url commit: a shallow clone of one commit, checked by hash.
fetch_git() {
	local dir=$1 url=$2 commit=$3
	if [ "$(git -C "$dir" rev-parse HEAD 2>/dev/null)" = "$commit" ] && [ -z "$(git -C "$dir" status --porcelain)" ]; then
		return
	fi
	log "$url @ ${commit:0:7}"
	rm -rf "$dir"
	git init -q "$dir"
	git -C "$dir" fetch -q --depth 1 "$url" "$commit"
	git -C "$dir" -c advice.detachedHead=false checkout -q FETCH_HEAD
	[ "$(git -C "$dir" rev-parse HEAD)" = "$commit" ] || die "$url: HEAD is not $commit"
}

# fetch_git_sparse dir url commit path...: fetch_git for the given paths of
# the commit only, for a repository too large to fetch whole.
fetch_git_sparse() {
	local dir=$1 url=$2 commit=$3
	shift 3
	if [ "$(git -C "$dir" rev-parse HEAD 2>/dev/null)" = "$commit" ] && [ -z "$(git -C "$dir" status --porcelain)" ] &&
		[ "$(git -C "$dir" sparse-checkout list 2>/dev/null)" = "$(printf '%s\n' "$@")" ]; then
		return
	fi
	log "$url @ ${commit:0:7} ($*)"
	rm -rf "$dir"
	git init -q "$dir"
	git -C "$dir" sparse-checkout set --cone "$@"
	git -C "$dir" fetch -q --depth 1 --filter=blob:none "$url" "$commit"
	git -C "$dir" -c advice.detachedHead=false checkout -q FETCH_HEAD
	[ "$(git -C "$dir" rev-parse HEAD)" = "$commit" ] || die "$url: HEAD is not $commit"
}

# fetch_file path url sha256: a download, checked by SHA-256.
fetch_file() {
	local path=$1 url=$2 sum=$3 got
	if [ -f "$path" ] && [ "$(sha256 "$path")" = "$sum" ]; then
		return
	fi
	log "$url"
	mkdir -p "$(dirname "$path")"
	curl -fsSL --retry 3 -o "$path.part" "$url"
	got=$(sha256 "$path.part")
	[ "$got" = "$sum" ] || die "$url: SHA-256 $got, want $sum"
	mv "$path.part" "$path"
}

# export_vars name=value...: prints the export lines to eval, and appends the
# variables to $GITHUB_ENV under GitHub Actions.
export_vars() {
	local v
	for v in "$@"; do
		printf 'export %q\n' "$v"
		if [ -n "${GITHUB_ENV:-}" ]; then
			echo "$v" >>"$GITHUB_ENV"
		fi
	done
}
