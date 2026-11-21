#!/usr/bin/env bash
# Fetches the artefacts of the optional tests of the Dutch rules
# (internal/rules/nl, validate) at pinned versions, checked by commit, and
# prints the variables to export, like fetch.sh, which runs it:
#
#   conformance/fetch-nl.sh [dir]
#   eval "$(conformance/fetch-nl.sh)" && go test ./internal/rules/nl ./validate
#
# It needs git only, and takes a few seconds: the CI workflow runs it alone.
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
. "$here/lib.sh"

DEST=${1:-$here/.artifacts}
mkdir -p "$DEST"
DEST=$(cd "$DEST" && pwd)

command -v git >/dev/null || die "git is required"

# Dutch Peppol Authority (NPa) validation artefacts, release 2026-05-21:
# SI-UBL 2.0.3.13, NLCIUS-CII 1.0.3.13 and G-account extension 1.0.2.13
# schematrons (FX_NL), which the catalogs of internal/rules/nl are compared
# with.
NPA_COMMIT=4e93ae8681f413fef627a338ad84ce465a153ee8
fetch_git "$DEST/npa-validation" https://github.com/peppolautoriteit-nl/validation.git "$NPA_COMMIT"

# phive-rules (Apache-2.0): the SimplerInvoicing test files of SI-UBL 2.0,
# NLCIUS-CII and the G-account extension (FX_NL_TESTS). The repository holds the test files of every
# rule set it packages (1.4 GB), so only that directory is fetched.
PHIVE_COMMIT=bab0629da499634482a23621c42398b222fccfec
PHIVE_TESTS=phive-rules-simplerinvoicing/src/test/resources/external/test-files/simplerinvoicing
fetch_git_sparse "$DEST/phive-rules" https://github.com/phax/phive-rules.git "$PHIVE_COMMIT" "$PHIVE_TESTS"

export_vars "FX_NL=$DEST/npa-validation" "FX_NL_TESTS=$DEST/phive-rules/$PHIVE_TESTS"
