#!/bin/bash
set -e

echo "🚀 Git-Sweep-Go Release Script (Enhanced) 🚀"
echo "-------------------------------------------"

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

# Get commit logs since last tag with enhanced details
echo "Getting detailed commit logs since $CURRENT_VERSION..."
COMMIT_LOGS=$(git log --pretty=format:"%h %s" $CURRENT_VERSION..HEAD)
DETAILED_COMMIT_LOGS=$(git log --pretty=format:"Commit: %h%nAuthor: %an <%ae>%nDate: %ad%nTitle: %s%n%nDescription:%n%b%n------------------" --date=format:"%Y-%m-%d %H:%M:%S" $CURRENT_VERSION..HEAD)

if [ -z "$COMMIT_LOGS" ]; then
    echo "⚠️ No commits found since $CURRENT_VERSION"
    RELEASE_NOTES="Release $NEW_VERSION"
    USE_AI=false
else
    # Format the logs for the conventional output
    CONVENTIONAL_NOTES=$(git log --pretty=format:"- %s" $CURRENT_VERSION..HEAD)
    
    # Extract previous release notes for context if available
    PREVIOUS_RELEASE_NOTES=""
    if [[ "$CURRENT_VERSION" != "v0.0.0" ]]; then
        echo "Fetching previous release notes for context..."
        # Try to get the release notes from the previous tag
        PREV_TAG_MESSAGE=$(git tag -l --format='%(contents)' "$CURRENT_VERSION" 2>/dev/null)
        
        # If the tag message exists and has content
        if [[ -n "$PREV_TAG_MESSAGE" && "$PREV_TAG_MESSAGE" != *"tag $CURRENT_VERSION"* ]]; then
            PREVIOUS_RELEASE_NOTES="$PREV_TAG_MESSAGE"
        else
            # Try to fetch from GitHub releases API as fallback
            REPO_URL=$(git config --get remote.origin.url)
            if [[ "$REPO_URL" =~ github\.com[:/]([^/]+)/([^/.]+)(.git)? ]]; then
                REPO_OWNER="${BASH_REMATCH[1]}"
                REPO_NAME="${BASH_REMATCH[2]}"
                
                echo "Attempting to fetch release notes from GitHub for $CURRENT_VERSION..."
                GITHUB_RESPONSE=$(curl -s "https://api.github.com/repos/$REPO_OWNER/$REPO_NAME/releases/tags/$CURRENT_VERSION")
                
                if [[ "$GITHUB_RESPONSE" != *"Not Found"* && "$GITHUB_RESPONSE" != *"message"* ]]; then
                    PREVIOUS_RELEASE_NOTES=$(echo "$GITHUB_RESPONSE" | jq -r '.body')
                    if [[ "$PREVIOUS_RELEASE_NOTES" == "null" ]]; then
                        PREVIOUS_RELEASE_NOTES=""
                    fi
                fi
            fi
        fi
        
        if [[ -n "$PREVIOUS_RELEASE_NOTES" ]]; then
            echo "✅ Previous release notes found. Will use as a style reference."
        else
            echo "⚠️ Could not find previous release notes. Will generate without historical context."
        fi
    fi
    
    # Extract project structure information
    echo "Extracting project structure information..."
    PROJECT_STRUCTURE=$(find . -type f -not -path "*/\.*" -not -path "*/dist/*" -not -path "*/vendor/*" | sort)
    
    # Get README content (more comprehensive than just first 5 lines)
    README_CONTENT=$(cat README.md | head -n 50)
    
    # Try to get main.go content for more context about the tool
    MAIN_FILE_CONTENT=""
    if [[ -f "./cmd/git-sweep/main.go" ]]; then
        MAIN_FILE_CONTENT=$(cat ./cmd/git-sweep/main.go)
    fi
    
    # Generate AI release notes if API key is available
    if [ "$USE_AI" = true ]; then
        echo "Generating enhanced AI release notes..."
        
        # Create a temporary file to build our JSON payload
        TEMP_JSON=$(mktemp)
        
        # For macOS compatibility with sed
        if [[ "$OSTYPE" == "darwin"* ]]; then
            # Use perl for macOS
            # Escaping all the content for JSON
            COMMITS_ESCAPED=$(echo "$DETAILED_COMMIT_LOGS" | perl -pe 's/"/\\"/g' | perl -0pe 's/\n/\\n/g')
            README_ESCAPED=$(echo "$README_CONTENT" | perl -pe 's/"/\\"/g' | perl -0pe 's/\n/\\n/g')
            PREV_NOTES_ESCAPED=""
            if [[ -n "$PREVIOUS_RELEASE_NOTES" ]]; then
                PREV_NOTES_ESCAPED=$(echo "$PREVIOUS_RELEASE_NOTES" | perl -pe 's/"/\\"/g' | perl -0pe 's/\n/\\n/g')
            fi
            MAIN_FILE_ESCAPED=""
            if [[ -n "$MAIN_FILE_CONTENT" ]]; then
                MAIN_FILE_ESCAPED=$(echo "$MAIN_FILE_CONTENT" | perl -pe 's/"/\\"/g' | perl -0pe 's/\n/\\n/g')
            fi
            STRUCTURE_ESCAPED=$(echo "$PROJECT_STRUCTURE" | perl -pe 's/"/\\"/g' | perl -0pe 's/\n/\\n/g')
        else
            # Use GNU sed for Linux
            COMMITS_ESCAPED=$(echo "$DETAILED_COMMIT_LOGS" | sed 's/"/\\"/g' | sed ':a;N;$!ba;s/\n/\\n/g')
            README_ESCAPED=$(echo "$README_CONTENT" | sed 's/"/\\"/g' | sed ':a;N;$!ba;s/\n/\\n/g')
            PREV_NOTES_ESCAPED=""
            if [[ -n "$PREVIOUS_RELEASE_NOTES" ]]; then
                PREV_NOTES_ESCAPED=$(echo "$PREVIOUS_RELEASE_NOTES" | sed 's/"/\\"/g' | sed ':a;N;$!ba;s/\n/\\n/g')
            fi
            MAIN_FILE_ESCAPED=""
            if [[ -n "$MAIN_FILE_CONTENT" ]]; then
                MAIN_FILE_ESCAPED=$(echo "$MAIN_FILE_CONTENT" | sed 's/"/\\"/g' | sed ':a;N;$!ba;s/\n/\\n/g')
            fi
            STRUCTURE_ESCAPED=$(echo "$PROJECT_STRUCTURE" | sed 's/"/\\"/g' | sed ':a;N;$!ba;s/\n/\\n/g')
        fi
        
        # Prepare enhanced system prompt
        SYSTEM_PROMPT="You are a technical release note writer specifically for the Git-Sweep-Go project.
Git-Sweep-Go is an interactive command-line tool written in Go that helps clean up old or merged Git branches in a local repository.

Generate professional, detailed release notes from the provided Git commits. Follow these guidelines:
1. Group changes into appropriate sections:
   - Features (new functionality)
   - Enhancements (improvements to existing features)
   - Bug Fixes
   - Documentation
   - Internal (refactoring, dependencies, tooling)
   - UI/UX Improvements (related to the TUI interface)

2. Use Git-Sweep-Go's domain-specific terminology correctly:
   - Branch analysis refers to identifying branches by their merge status
   - TUI refers to the Terminal User Interface (using bubbletea)
   - Remote operations refer to actions on remote repositories
   - Protected branches are those that should never be deleted

3. Format each entry as a bullet point with a clear, concise description
   - Start with a verb in present tense (Adds, Fixes, Updates, etc.)
   - Reference issue numbers if they appear in commit messages (#XX format)
   - Don't include commit hashes or technical details not relevant to users

4. Keep the tone professional but approachable
5. Prioritize user-facing changes over internal changes
6. If the previous release notes are provided, follow a similar style and format"

        # Construct JSON payload with enhanced information
        jq -n \
            --arg version "$NEW_VERSION" \
            --arg commits "$COMMITS_ESCAPED" \
            --arg readme "$README_ESCAPED" \
            --arg prevnotes "$PREV_NOTES_ESCAPED" \
            --arg mainfile "$MAIN_FILE_ESCAPED" \
            --arg structure "$STRUCTURE_ESCAPED" \
            --arg sysprompt "$SYSTEM_PROMPT" \
            '{
                "model": "gpt-4o",
                "messages": [
                    {
                        "role": "system", 
                        "content": $sysprompt
                    },
                    {
                        "role": "user",
                        "content": ("Write comprehensive release notes for version " + $version + " of git-sweep-go based on these commits:\n\n" + $commits + "\n\nProject README Overview:\n" + $readme + 
                        (if $prevnotes != "" then "\n\nPrevious Release Notes Style Example:\n" + $prevnotes else "") +
                        (if $mainfile != "" then "\n\nMain Program Code Context:\n" + $mainfile else "") +
                        "\n\nProject Structure:\n" + $structure + 
                        "\n\nFocus on highlighting important changes, especially those that impact users. Group related changes into appropriate sections and follow the format guidelines.")
                    }
                ],
                "temperature": 0.7,
                "max_tokens": 1500
            }' > "$TEMP_JSON"
        
        # Save a copy for error logging if needed
        ERROR_LOG_FILE=$(mktemp)
        cat "$TEMP_JSON" > "$ERROR_LOG_FILE"
        
        # Call the OpenAI API with the enhanced JSON
        echo "Sending request to OpenAI API (this may take a moment)..."
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
            echo -e "\n------- Enhanced AI-Generated Release Notes -------"
            echo "$AI_NOTES"
            echo -e "\n------- Conventional Release Notes -------"
            echo "$CONVENTIONAL_NOTES"
            echo -e "\n-----------------------------------------"
            
            # Ask the user which version to use
            read -p "Use enhanced AI-generated release notes? (y/n): " USE_AI_NOTES
            
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
