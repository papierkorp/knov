#!/usr/bin/env bash
# generate-changelog.sh — generates docs/changelogs/<year>.md from git log grouped by month

set -e

CHANGELOG_DIR="docs/changelogs"
mkdir -p "$CHANGELOG_DIR"

# Get all commits with their year-month and subject in one pass
# Format: YYYY-MM <subject>
ALL=$(git log --pretty=format:"%ad %s" --date=format:"%Y-%m")

YEARS=$(echo "$ALL" | awk '{print substr($1,1,4)}' | sort -ur)

for YEAR in $YEARS; do
    OUTPUT="$CHANGELOG_DIR/$YEAR.md"
    echo "# changelog $YEAR" > "$OUTPUT"
    echo "" >> "$OUTPUT"

    MONTHS=$(echo "$ALL" | awk -v y="$YEAR" '$1 ~ "^"y"-" {print $1}' | sort -ur)

    for MONTH in $MONTHS; do
        MONTH_NAME=$(date -d "$MONTH-01" +"%B" 2>/dev/null || date -j -f "%Y-%m-%d" "$MONTH-01" +"%B")
        SUBJECTS=$(echo "$ALL" | awk -v m="$MONTH" '$1 == m {$1=""; print substr($0,2)}')

        BREAKING=$(echo "$SUBJECTS" | grep -E "^.+(\(.+\))?!:|^BREAKING CHANGE:" | sed 's/^/- /' || true)
        FEATS=$(echo "$SUBJECTS"   | grep -iE "^feat(\(.+\))?!?:"                                             | sed 's/^[^:]*: */- /' || true)
        FIXES=$(echo "$SUBJECTS"   | grep -iE "^fix(\(.+\))?!?:"                                              | sed 's/^[^:]*: */- /' || true)
        OTHERS=$(echo "$SUBJECTS"  | grep -iE "^(build|chore|ci|docs|style|refactor|perf|test)(\(.+\))?!?:"  | sed 's/^[^:]*: */- /' || true)

        if [ -z "$BREAKING" ] && [ -z "$FEATS" ] && [ -z "$FIXES" ] && [ -z "$OTHERS" ]; then
            continue
        fi

        echo "## $MONTH_NAME" >> "$OUTPUT"
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
    done

    echo "changelog written to $OUTPUT"
done
