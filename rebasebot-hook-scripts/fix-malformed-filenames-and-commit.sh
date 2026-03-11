#!/bin/bash
set -euo pipefail

# Remove unicode control characters (e.g. U+200E left-to-right mark) from
# filenames. These break Go's module zip creation and block all downstream
# repos that depend on this module.
#
# Upstream fix reference: https://github.com/vmware-tanzu/velero/pull/9552
#
# Committed as UPSTREAM: <drop> so rebasebot will not carry it forward
# once upstream has fixed all occurrences.

if [[ -z "${REBASEBOT_GIT_USERNAME:-}" || -z "${REBASEBOT_GIT_EMAIL:-}" ]]; then
    author_flag=()
else
    author_flag=(--author="$REBASEBOT_GIT_USERNAME <$REBASEBOT_GIT_EMAIL>")
fi

found=0
while IFS= read -r -d '' file; do
    clean_name=$(echo "$file" | sed "s/[$(printf '\u200e\u200f\u200b')]//g")
    if [[ "$file" != "$clean_name" ]]; then
        echo "Renaming: $file -> $clean_name"
        git mv "$file" "$clean_name"
        found=1
    fi
done < <(find . -name '*['"$(printf '\u200e\u200f\u200b')"']*' -print0 2>/dev/null)

if [[ "$found" -eq 1 ]]; then
    git add -A
    git commit "${author_flag[@]}" -q -m "UPSTREAM: <drop>: Fix malformed unicode characters in filenames"
    echo "Committed filename fixes."
else
    echo "No files with malformed unicode characters found."
fi
