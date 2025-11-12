#!/bin/bash

# Translation String Extraction Script for Go HTML Templates & Go Source Files
# =============================================================================
# Extracts translatable strings from .gohtml files containing {{T "..."}} calls
# and .go files containing translation.Sprintf("...") calls
# and generates translation catalogs for internationalization.
#
# Usage: Run from internal/translation directory via go generate
# Output: Updates catalog.go with found translation strings
#
# Prerequisites:
# - gotext tool: go install golang.org/x/text/cmd/gotext@latest

echo "extracting translatable strings from .gohtml and .go files..."

# Configuration
TEMP_GO_FILE="./temp_extracted.go"
OUTPUT_CATALOG="catalog.go"
SUPPORTED_LANGUAGES="en,de"

# Extract translation strings from both template and Go files
{
    # Extract {{T "..."}} calls from .gohtml files
    echo "scanning .gohtml template files..."
    find ../../ -name "*.gohtml" \
        -exec grep -ho '{{[[:space:]]*T[[:space:]]\+"[^"]*"[[:space:]]*}}' {} \; | \
        sed 's/{{[[:space:]]*T[[:space:]]\+"//g; s/"[[:space:]]*}}//g'

    # Extract translation.Sprintf("...") calls from .go files
    echo "scanning .go source files..." >&2
    find ../../ -name "*.go" \
        -exec grep -ho 'translation\.Sprintf("[^"]*")' {} \; | \
        sed 's/translation\.Sprintf("//g; s/")//g'

    # Extract translation.SprintfForRequest(..., "...") calls from .go files
    echo "scanning .go source files for SprintfForRequest..." >&2
    find ../../ -name "*.go" \
        -exec grep -ho 'translation\.SprintfForRequest([^,]*, "[^"]*")' {} \; | \
        sed 's/translation\.SprintfForRequest([^,]*, "//g; s/")//g'
} | \
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

# Sync translation files - add missing entries and remove obsolete ones
sync_translation_files() {
    local lang_dir="$1"
    # Remove trailing slash to fix double slash issue
    lang_dir="${lang_dir%/}"
    local messages_file="$lang_dir/messages.gotext.json"
    local out_file="$lang_dir/out.gotext.json"

    if [ ! -f "$out_file" ]; then
        echo "warning: $out_file not found, skipping sync for $lang_dir"
        return
    fi

    if [ ! -f "$messages_file" ]; then
        echo "creating new $messages_file"
        echo '{"language": "'$(basename "$lang_dir")'", "messages": []}' > "$messages_file"
    fi

    echo "syncing translation file: $lang_dir"

    # Backup existing file
    cp "$messages_file" "$messages_file.backup"

    # Use a temporary file for the updated messages
    local temp_file="$messages_file.tmp"

    # Create a simpler jq script without shell quoting issues
    cat > /tmp/sync_translations.jq << 'EOF'
# Get existing translations as a lookup table
(.messages // []) as $existing |
($existing | map(select(.translation != null and .translation != "") | {(.id): .translation}) | add // {}) as $translations |

# Get current messages from out.gotext.json
$out_data[0].messages as $current |

# Build new messages array
{
    language: .language,
    messages: [
        $current[] | {
            id: .id,
            message: .message,
            translation: ($translations[.id] // "")
        }
    ]
}
EOF

    # Run jq with external script file
    jq --slurpfile out_data "$out_file" -f /tmp/sync_translations.jq "$messages_file" > "$temp_file"

    # Replace original file if jq succeeded
    if [ $? -eq 0 ]; then
        mv "$temp_file" "$messages_file"
        echo "successfully synced $messages_file"
        rm -f "$messages_file.backup"
    else
        echo "error: failed to sync $messages_file, restoring backup"
        mv "$messages_file.backup" "$messages_file"
        rm -f "$temp_file"
    fi

    # Clean up temp script
    rm -f /tmp/sync_translations.jq
}

# Sync all language directories
if [ $? -eq 0 ]; then
    echo "syncing translation files..."
    for lang_dir in locales/*/; do
        if [ -d "$lang_dir" ]; then
            sync_translation_files "$lang_dir"
        fi
    done
fi

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
