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

# Define characters using raw UTF-8 byte sequences (\xNN) for portability.
# Do NOT use printf '\uNNNN' — older bash/printf may not support \u escapes,
# causing the literal characters u,2,0,e,f,b to be matched and stripped.
LRM=$'\xe2\x80\x8e'    # U+200E LEFT-TO-RIGHT MARK
RLM=$'\xe2\x80\x8f'    # U+200F RIGHT-TO-LEFT MARK
ZWSP=$'\xe2\x80\x8b'   # U+200B ZERO WIDTH SPACE

if [[ -z "${REBASEBOT_GIT_USERNAME:-}" || -z "${REBASEBOT_GIT_EMAIL:-}" ]]; then
    author_flag=()
else
    author_flag=(--author="$REBASEBOT_GIT_USERNAME <$REBASEBOT_GIT_EMAIL>")
fi

found=0
# Use git ls-files (not find) to iterate only tracked files, avoiding
# "bad source" errors when find returns a directory that gets renamed
# before its child files are processed.
while IFS= read -r -d '' file; do
    # Use bash parameter substitution instead of sed character class —
    # sed [..] treats multi-byte UTF-8 sequences as individual bytes.
    clean_name="$file"
    clean_name="${clean_name//$LRM/}"
    clean_name="${clean_name//$RLM/}"
    clean_name="${clean_name//$ZWSP/}"
    if [[ "$file" != "$clean_name" ]]; then
        echo "Renaming: $file -> $clean_name"
        mkdir -p "$(dirname "$clean_name")"
        git mv "$file" "$clean_name"
        found=1
    fi
done < <(git ls-files -z)

if [[ "$found" -eq 1 ]]; then
    git add -A
    git commit "${author_flag[@]}" -q -m "UPSTREAM: <drop>: Fix malformed unicode characters in filenames"
    echo "Committed filename fixes."
else
    echo "No files with malformed unicode characters found."
fi
