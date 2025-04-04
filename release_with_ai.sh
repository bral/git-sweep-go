#!/bin/bash
set -e

echo "🚀 Git-Sweep-Go Release Script 🚀"
echo "--------------------------------"

# Check if goreleaser is installed
if ! command -v goreleaser &> /dev/null; then
    echo "❌ goreleaser is not installed. Please install it first."
    echo "   See: https://goreleaser.com/install/"
    exit 1
fi

# Check if curl is installed (needed for API requests)
if ! command -v curl &> /dev/null; then
    echo "❌ curl is not installed. Please install it first."
    exit 1
fi

# Check if jq is installed (needed for JSON processing)
if ! command -v jq &> /dev/null; then
    echo "❌ jq is not installed. Please install it first."
    echo "   Install with: brew install jq (macOS) or apt install jq (Ubuntu)"
    exit 1
fi

# Check for OpenAI API key
if [ -z "$OPENAI_API_KEY" ]; then
    echo "⚠️ OPENAI_API_KEY environment variable not set."
    echo "   For AI-generated release notes, please set your OpenAI API key:"
    echo "   export OPENAI_API_KEY='your-api-key'"
    USE_AI=false
else
    USE_AI=true
    echo "✅ OpenAI API key found. Will use AI to generate release notes."
    
    # Verify we can connect to the OpenAI API
    echo "Testing OpenAI API connection..."
    TEST_RESPONSE=$(curl -s -X GET "https://api.openai.com/v1/models" \
        -H "Authorization: Bearer $OPENAI_API_KEY")
    
    if echo "$TEST_RESPONSE" | jq -e '.error' > /dev/null; then
        ERROR_MSG=$(echo "$TEST_RESPONSE" | jq -r '.error.message')
        echo "❌ OpenAI API connection test failed: $ERROR_MSG"
        echo "   Please check your API key and internet connection."
        USE_AI=false
    else
        echo "✅ OpenAI API connection successful."
    fi
fi

# Get current version
CURRENT_VERSION=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
echo "Current version: $CURRENT_VERSION"

# Extract version parts
if [[ $CURRENT_VERSION =~ v([0-9]+)\.([0-9]+)\.([0-9]+) ]]; then
    MAJOR="${BASH_REMATCH[1]}"
    MINOR="${BASH_REMATCH[2]}"
    PATCH="${BASH_REMATCH[3]}"
else
    echo "❌ Could not parse current version: $CURRENT_VERSION"
    exit 1
fi

# Ask for version bump type
echo "Select version bump type:"
echo "1) Major (v$((MAJOR+1)).0.0)"
echo "2) Minor (v$MAJOR.$((MINOR+1)).0)"
echo "3) Patch (v$MAJOR.$MINOR.$((PATCH+1)))"
echo "4) Enter custom version"

read -p "Choose option [1-4]: " BUMP_TYPE

case $BUMP_TYPE in
    1)
        NEW_VERSION="v$((MAJOR+1)).0.0"
        ;;
    2)
        NEW_VERSION="v$MAJOR.$((MINOR+1)).0"
        ;;
    3)
        NEW_VERSION="v$MAJOR.$MINOR.$((PATCH+1))"
        ;;
    4)
        read -p "Enter custom version (format vX.Y.Z): " NEW_VERSION
        if [[ ! "$NEW_VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
            echo "❌ Invalid version format. Please use semantic versioning (e.g., v0.1.4)"
            exit 1
        fi
        ;;
    *)
        echo "❌ Invalid option"
        exit 1
        ;;
esac

# Check if tag already exists
if git rev-parse "$NEW_VERSION" >/dev/null 2>&1; then
    echo "❌ Tag $NEW_VERSION already exists!"
    exit 1
fi

# Run tests
echo "Running tests..."
if ! go test ./...; then
    echo "❌ Tests failed! Aborting release."
    exit 1
fi

# Build to verify compilation
echo "Building project to verify it compiles..."
if ! go build -o git-sweep ./cmd/git-sweep/main.go; then
    echo "❌ Build failed! Aborting release."
    exit 1
fi
rm -f git-sweep

# Get commit logs since last tag
echo "Getting commit logs since $CURRENT_VERSION..."
COMMIT_LOGS=$(git log --pretty=format:"%h %s" $CURRENT_VERSION..HEAD)

if [ -z "$COMMIT_LOGS" ]; then
    echo "⚠️ No commits found since $CURRENT_VERSION"
    RELEASE_NOTES="Release $NEW_VERSION"
    USE_AI=false
else
    # Format the logs for the conventional output
    CONVENTIONAL_NOTES=$(git log --pretty=format:"- %s" $CURRENT_VERSION..HEAD)
    
    # Generate AI release notes if API key is available
    if [ "$USE_AI" = true ]; then
        echo "Generating AI release notes..."
        
        # Create a temporary file to build our JSON payload - this avoids issues with variable expansion
        TEMP_JSON=$(mktemp)
        
        # Start building the JSON payload
        cat > "$TEMP_JSON" << EOF
{
  "model": "gpt-4o",
  "messages": [
    {
      "role": "system",
      "content": "You are a technical release note writer. Generate concise, professional release notes from Git commits. Group related changes into sections (Features, Bug Fixes, Documentation, etc). Use bullet points and keep the tone professional."
    },
    {
      "role": "user",
      "content": "Create release notes for version $NEW_VERSION of git-sweep-go based on these commits:\\n\\n$(echo "$COMMIT_LOGS" | sed 's/"/\\"/g' | sed ':a;N;$!ba;s/\n/\\n/g')\\n\\nProject description:\\n$(head -n 5 README.md | sed 's/"/\\"/g' | sed ':a;N;$!ba;s/\n/\\n/g')"
    }
  ],
  "temperature": 0.7,
  "max_tokens": 1000
}
EOF

        # Check if the JSON is valid (using jq as a validator)
        if ! jq empty "$TEMP_JSON" > /dev/null 2>&1; then
            echo "❌ Generated JSON is invalid. This is likely due to special characters in commit messages."
            echo "Falling back to a simpler JSON structure, but still including commit details..."
            
            # Create a safer JSON using jq
            jq -n --arg version "$NEW_VERSION" --arg commits "$COMMIT_LOGS" '{
                "model": "gpt-4o",
                "messages": [
                    {
                        "role": "system", 
                        "content": "You are a technical release note writer. Generate concise, professional release notes from Git commits. Group related changes into sections (Features, Bug Fixes, Documentation, etc). Use bullet points and keep the tone professional."
                    },
                    {
                        "role": "user",
                        "content": "Create release notes for version " + $version + " of git-sweep-go based on these commits:\n\n" + $commits + "\n\nThe project is a Git branch cleanup tool. Focus specifically on what these exact commits changed."
                    }
                ],
                "temperature": 0.7,
                "max_tokens": 1000
            }' > "$TEMP_JSON"
        fi
        
        # Save a copy for error logging if needed
        ERROR_LOG_FILE=$(mktemp)
        cat "$TEMP_JSON" > "$ERROR_LOG_FILE"
        
        # Call the OpenAI API with the JSON from our temp file
        API_RESPONSE=$(curl -s -X POST "https://api.openai.com/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer $OPENAI_API_KEY" \
            --data-binary "@$TEMP_JSON")
            
        # Clean up the temp file
        rm -f "$TEMP_JSON"
        
        # Check for API errors
        if echo "$API_RESPONSE" | jq -e '.error' > /dev/null; then
            ERROR_MSG=$(echo "$API_RESPONSE" | jq -r '.error.message')
            ERROR_TYPE=$(echo "$API_RESPONSE" | jq -r '.error.type // "unknown"')
            echo "❌ OpenAI API error: $ERROR_MSG (Type: $ERROR_TYPE)"
            
            # Create error log for debugging
            ERROR_LOG="openai_error_$(date +%Y%m%d_%H%M%S).log"
            echo "=== API REQUEST ===" > "$ERROR_LOG"
            cat "$ERROR_LOG_FILE" >> "$ERROR_LOG" 2>/dev/null || echo "Request JSON not found" >> "$ERROR_LOG"
            echo -e "\n=== API RESPONSE ===" >> "$ERROR_LOG" 
            echo "$API_RESPONSE" | jq '.' >> "$ERROR_LOG" 2>/dev/null || echo "$API_RESPONSE" >> "$ERROR_LOG"
            echo "📝 Detailed error information saved to $ERROR_LOG"
            
            # Clean up the error log file
            rm -f "$ERROR_LOG_FILE"
            
            echo "Falling back to conventional release notes..."
            RELEASE_NOTES="$CONVENTIONAL_NOTES"
        else
            # Clean up the error log file
            rm -f "$ERROR_LOG_FILE"
            
            # Extract the generated release notes from the API response
            AI_NOTES=$(echo "$API_RESPONSE" | jq -r '.choices[0].message.content')
            
            # Show both options
            echo -e "\n------- AI-Generated Release Notes -------"
            echo "$AI_NOTES"
            echo -e "\n------- Conventional Release Notes -------"
            echo "$CONVENTIONAL_NOTES"
            echo -e "\n-----------------------------------------"
            
            # Ask the user which version to use
            read -p "Use AI-generated release notes? (y/n): " USE_AI_NOTES
            
            if [[ "$USE_AI_NOTES" == "y" || "$USE_AI_NOTES" == "Y" ]]; then
                RELEASE_NOTES="$AI_NOTES"
            else
                RELEASE_NOTES="$CONVENTIONAL_NOTES"
            fi
        fi
    else
        # Use conventional release notes if AI is not available
        RELEASE_NOTES="$CONVENTIONAL_NOTES"
    fi
fi

# Show generated release notes and allow editing
echo -e "\nFinal release notes:"
echo "$RELEASE_NOTES"
echo ""
read -p "Edit release notes? (y/n): " EDIT_NOTES

if [[ "$EDIT_NOTES" == "y" || "$EDIT_NOTES" == "Y" ]]; then
    # Create a temporary file with the release notes
    TEMP_FILE=$(mktemp)
    echo "$RELEASE_NOTES" > "$TEMP_FILE"

    # Open in default editor
    ${EDITOR:-vim} "$TEMP_FILE"

    # Read the edited release notes
    RELEASE_NOTES=$(cat "$TEMP_FILE")
    rm "$TEMP_FILE"
fi

# Confirm with user
echo ""
echo "Ready to release:"
echo "  Current version: $CURRENT_VERSION"
echo "  New version:     $NEW_VERSION"
echo "  Release notes:"
echo "$RELEASE_NOTES"
echo ""
read -p "Proceed with release? (y/n): " CONFIRM

if [[ "$CONFIRM" != "y" && "$CONFIRM" != "Y" ]]; then
    echo "Release canceled."
    exit 0
fi

# Create and push tag
git tag -a "$NEW_VERSION" -m "Release $NEW_VERSION"
echo "✅ Tag $NEW_VERSION created."

echo "Pushing tag to remote..."
git push origin "$NEW_VERSION"
echo "✅ Tag pushed to remote."

# Run goreleaser
echo "Running goreleaser..."
RELEASE_NOTES_FILE=$(mktemp)
echo "$RELEASE_NOTES" > "$RELEASE_NOTES_FILE"
GORELEASER_PREVIOUS_TAG="$CURRENT_VERSION" GORELEASER_CURRENT_TAG="$NEW_VERSION" goreleaser release --release-notes="$RELEASE_NOTES_FILE" --clean
rm "$RELEASE_NOTES_FILE"

echo "✅ Release $NEW_VERSION complete!"
