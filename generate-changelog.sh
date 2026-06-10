#!/usr/bin/env bash
# generate-changelog.sh — generates docs/changelogs/<year>.md from git log grouped by month

set -e

CHANGELOG_DIR="docs/changelogs"
mkdir -p "$CHANGELOG_DIR"

next_month() {
    local YEAR=$1 MONTH_NUM=$2
    date -d "$YEAR-$MONTH_NUM-01 +1 month" +%Y-%m-%d 2>/dev/null \
        || date -j -v+1m -f "%Y-%m-%d" "$YEAR-$MONTH_NUM-01" +%Y-%m-%d
}

month_name() {
    local YEAR=$1 MONTH_NUM=$2
    date -d "$YEAR-$MONTH_NUM-01" +"%B" 2>/dev/null \
        || date -j -f "%Y-%m-%d" "$YEAR-$MONTH_NUM-01" +"%B"
}

write_month() {
    local YEAR=$1 MONTH_NUM=$2 OUTPUT=$3
    local AFTER="$YEAR-$MONTH_NUM-01"
    local BEFORE
    BEFORE=$(next_month "$YEAR" "$MONTH_NUM")

    local ALL
    ALL=$(git log --pretty=format:"%s" --after="$AFTER" --before="$BEFORE")

    local BREAKING
    BREAKING=$(git log --pretty=format:"%s%n%b" --after="$AFTER" --before="$BEFORE" \
        | grep -E "^.+(\(.+\))?!:|^BREAKING CHANGE:" | sed 's/^/- /' || true)

    local FEATS FIXES OTHERS
    FEATS=$(echo "$ALL"  | grep -iE "^feat(\(.+\))?!?:"                                              | sed 's/^[^:]*: */- /' || true)
    FIXES=$(echo "$ALL"  | grep -iE "^fix(\(.+\))?!?:"                                               | sed 's/^[^:]*: */- /' || true)
    OTHERS=$(echo "$ALL" | grep -iE "^(build|chore|ci|docs|style|refactor|perf|test)(\(.+\))?!?:"   | sed 's/^[^:]*: */- /' || true)

    if [ -z "$BREAKING" ] && [ -z "$FEATS" ] && [ -z "$FIXES" ] && [ -z "$OTHERS" ]; then
        return
    fi

    echo "## $(month_name "$YEAR" "$MONTH_NUM")" >> "$OUTPUT"
    echo "" >> "$OUTPUT"

    if [ -n "$BREAKING" ]; then
        echo "### breaking changes" >> "$OUTPUT"
        echo "$BREAKING" >> "$OUTPUT"
        echo "" >> "$OUTPUT"
    fi
    if [ -n "$FEATS" ]; then
        echo "### features" >> "$OUTPUT"
        echo "$FEATS" >> "$OUTPUT"
        echo "" >> "$OUTPUT"
    fi
    if [ -n "$FIXES" ]; then
        echo "### fixes" >> "$OUTPUT"
        echo "$FIXES" >> "$OUTPUT"
        echo "" >> "$OUTPUT"
    fi
    if [ -n "$OTHERS" ]; then
        echo "### other" >> "$OUTPUT"
        echo "$OTHERS" >> "$OUTPUT"
        echo "" >> "$OUTPUT"
    fi
}

YEARS=$(git log --pretty=format:"%ad" --date=format:"%Y" | sort -ur)

for YEAR in $YEARS; do
    OUTPUT="$CHANGELOG_DIR/$YEAR.md"
    echo "# changelog $YEAR" > "$OUTPUT"
    echo "" >> "$OUTPUT"

    MONTHS=$(git log --pretty=format:"%ad" --date=format:"%Y-%m" | grep "^$YEAR-" | sort -ur)

    for MONTH in $MONTHS; do
        MONTH_NUM=$(echo "$MONTH" | cut -d'-' -f2)
        write_month "$YEAR" "$MONTH_NUM" "$OUTPUT"
    done

    echo "changelog written to $OUTPUT"
done
