#!/bin/sh
set -e

commit_file="$1"
commit_msg=$(cat "$commit_file")

# Skip merge commits
if echo "$commit_msg" | grep -qE "^Merge "; then
    exit 0
fi

pattern='^(feat|fix|docs|style|refactor|perf|test|chore|ci|build|revert)(\([a-z0-9_-]+\))?(!)?: .{1,}'

if ! echo "$commit_msg" | head -1 | grep -qE "$pattern"; then
    echo "ERROR: Invalid commit message format"
    echo ""
    echo "Expected: <type>(<scope>)?: <description>"
    echo ""
    echo "Types: feat, fix, docs, style, refactor, perf, test, chore, ci, build, revert"
    echo ""
    echo "Examples:"
    echo "  feat: add user authentication"
    echo "  fix(parser): handle empty input"
    echo "  docs: update readme"
    echo "  refactor!: rename config options"
    echo ""
    echo "Your message: $(head -1 "$commit_file")"
    exit 1
fi
