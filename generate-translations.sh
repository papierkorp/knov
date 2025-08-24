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
THEME_FILES="themes/"
TEMP_GO_FILE="internal/translation/temp_extracted.go"
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
cd internal/translation && \
    gotext -srclang=en update -out="$OUTPUT_CATALOG" -lang="$SUPPORTED_LANGUAGES" .

if [ $? -eq 0 ]; then
    echo "Translation catalogs updated successfully!"
else
    echo "Error: Failed to generate translation catalogs" >&2
    exit 1
fi

echo "Translation extraction complete!"
