#!/bin/bash

# Translation String Extraction Script
# ===================================
# This script automates the process of extracting translatable strings from Go template files
# and generating translation catalogs for internationalization (i18n).
#
# What it does:
# 1. Searches all .templ files in the themes/ directory for translation.Sprintf() calls
# 2. Extracts the string literals from these calls
# 3. Generates a temporary Go file with all unique strings for gotext processing
# 4. Uses gotext to create/update translation catalogs
#
# Prerequisites:
# - gotext tool must be installed (go install golang.org/x/text/cmd/gotext@latest)
# - Project structure with themes/ directory containing .templ files

# Change to script directory to ensure relative paths work correctly
cd "$(dirname "$0")"

echo "Extracting translatable strings..."

# Define regex patterns with meaningful names
TRANSLATION_CALL_PATTERN='translation\.Sprintf("[^"]*")'  # Matches: translation.Sprintf("text")
EXTRACT_STRING_PATTERN='s/translation\.Sprintf("//g; s/")//g'  # Removes function wrapper, keeps content

# File paths and settings
THEME_FILES="../themes/"
TEMP_GO_FILE="../internal/translation/temp_extracted.go"
TRANSLATION_PATH="../internal/translation"
OUTPUT_CATALOG="catalog.go"
SUPPORTED_LANGUAGES="en,de"

# Step 1: Find all .templ files and extract translation calls
# Step 2: Clean up the extracted strings to get just the text content
# Step 3: Sort and remove duplicates
# Step 4: Generate a temporary Go file that imports all strings for gotext processing
find "$THEME_FILES" -name "*.templ" \
    -exec grep -ho "$TRANSLATION_CALL_PATTERN" {} \; | \
    sed "$EXTRACT_STRING_PATTERN" | \
    sort -u | \
    awk -v output_file="$TEMP_GO_FILE" '
    BEGIN {
        # Generate Go file header
        print "// Package translation is temporary created in the translation script and can be deleted"
        print "package translation"
        print ""
        print "import \"golang.org/x/text/message\""
        print ""
        print "func init() {"
        print "    p := message.NewPrinter(message.MatchLanguage(\"en\"))"
    }
    {
        # Process each unique string
        if (length($0) > 0) {
            # Add string to Go file so gotext can find it
            print "    _ = p.Sprintf(\"" $0 "\")"
            count++
        }
    }
    END {
        # Close the Go function
        print "}"

        # Report results to stderr (so it shows in terminal but not in file)
        print count " strings found" > "/dev/stderr"
}' > "$TEMP_GO_FILE"

echo "Generated temporary Go file: $TEMP_GO_FILE"

# Step 5: Use gotext to generate/update translation catalogs
echo "Generating translation catalogs..."
cd $TRANSLATION_PATH && \
    gotext -srclang=en update -out="$OUTPUT_CATALOG" -lang="$SUPPORTED_LANGUAGES" .

# Merge new strings from out.gotext.json into messages.gotext.json
for lang in $(echo "$SUPPORTED_LANGUAGES" | tr ',' ' '); do
    out_file="locales/$lang/out.gotext.json"
    messages_file="locales/$lang/messages.gotext.json"

    if [ -f "$out_file" ]; then
        if [ ! -f "$messages_file" ]; then
            # First time - just copy out to messages
            cp "$out_file" "$messages_file"
            echo "Created initial messages.gotext.json for language: $lang"
        else
            # Merge: add new entries from out.gotext.json to messages.gotext.json
            # This preserves existing translations and adds new untranslated entries
            python3 -c "
import json
import sys

# Read both files
with open('$out_file', 'r') as f:
    out_data = json.load(f)
with open('$messages_file', 'r') as f:
    messages_data = json.load(f)

# Create a dict of existing translations
existing = {msg['id']: msg for msg in messages_data['messages']}

# Add new messages that don't exist yet
for msg in out_data['messages']:
    if msg['id'] not in existing:
        messages_data['messages'].append(msg)

# Write back to messages file
with open('$messages_file', 'w') as f:
    json.dump(messages_data, f, indent=2)
            "
            echo "Merged new strings into messages.gotext.json for language: $lang"
        fi
    fi
done

if [ $? -eq 0 ]; then
    echo "Translation catalogs updated successfully!"
else
    echo "Error: Failed to generate translation catalogs" >&2
    exit 1
fi

echo "Translation extraction complete!"
