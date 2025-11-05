#!/bin/bash

# Translation String Extraction Script for Go HTML Templates
# ==========================================================
# Extracts translatable strings from .gohtml files containing {{T "..."}} calls
# and generates translation catalogs for internationalization.
#
# Usage: Run from internal/translation directory via go generate
# Output: Updates catalog.go with found translation strings
#
# Prerequisites:
# - gotext tool: go install golang.org/x/text/cmd/gotext@latest

echo "extracting translatable strings from .gohtml files..."

# Configuration
TEMP_GO_FILE="./temp_extracted.go"
OUTPUT_CATALOG="catalog.go"
SUPPORTED_LANGUAGES="en,de"

# Extract {{T "..."}} calls from .gohtml files in project root and generate temp Go file
find ../../ -name "*.gohtml" \
    -exec grep -ho '{{T "[^"]*"}}' {} \; | \
    sed 's/{{T "//g; s/"}}//g' | \
    sort -u | \
    awk -v temp_file="$TEMP_GO_FILE" '
    BEGIN {
        print "// temporary file for gotext extraction - auto-generated, safe to delete"
        print "package translation"
        print ""
        print "import \"golang.org/x/text/message\""
        print ""
        print "func init() {"
        print "    p := message.NewPrinter(message.MatchLanguage(\"en\"))"
    }
    {
        if (length($0) > 0) {
            print "    _ = p.Sprintf(\"" $0 "\")"
            count++
        }
    }
    END {
        # Always use the variable to avoid "declared and not used" error
        if (count == 0) {
            print "    _ = p // avoid unused variable error"
        }
        print "}"
        print count " translation strings found" > "/dev/stderr"
    }' > "$TEMP_GO_FILE"

echo "generated temporary extraction file: $TEMP_GO_FILE"

# Generate translation catalog
echo "updating translation catalog..."
gotext -srclang=en update -out="$OUTPUT_CATALOG" -lang="$SUPPORTED_LANGUAGES" .

if [ $? -eq 0 ]; then
    echo "translation catalog updated successfully!"
    rm "$TEMP_GO_FILE"
    echo "cleaned up temporary file"
else
    echo "error: failed to update translation catalog" >&2
    echo "temporary file preserved at: $TEMP_GO_FILE" >&2
    exit 1
fi

echo "translation extraction complete!"
