#!/bin/bash
set -e

echo "🧪 TEST MODE: Git-Sweep-Go Enhanced AI Release Script 🧪"
echo "--------------------------------"
echo "⚠️  No actual tags or releases will be created"
echo "--------------------------------"

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

# Check if tag already exists (will show warning but continue in test mode)
if git rev-parse "$NEW_VERSION" >/dev/null 2>&1; then
    echo "⚠️ Tag $NEW_VERSION already exists! In actual release, this would abort."
    echo "   Continuing for test purposes..."
fi

# Run tests
echo "Running tests..."
if ! go test ./...; then
    echo "❌ Tests failed! In actual release, this would abort."
    echo "   Continue anyway for testing? (y/n): "
    read CONTINUE
    if [[ "$CONTINUE" != "y" && "$CONTINUE" != "Y" ]]; then
        exit 1
    fi
fi

# Build to verify compilation
echo "Building project to verify it compiles..."
if ! go build -o git-sweep ./cmd/git-sweep/main.go; then
    echo "❌ Build failed! In actual release, this would abort."
    echo "   Continue anyway for testing? (y/n): "
    read CONTINUE
    if [[ "$CONTINUE" != "y" && "$CONTINUE" != "Y" ]]; then
        exit 1
    fi
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
        
        # Prepare enhanced system prompt for developer-focused GitHub releases
        SYSTEM_PROMPT="You are a technical release note writer for the Git-Sweep-Go project's GitHub releases.
Git-Sweep-Go is an interactive command-line tool written in Go that helps clean up old or merged Git branches in local repositories.

Generate developer-focused GitHub release notes from the provided Git commits. Your audience is primarily developers and technical users who want specifics about what changed and why.

## Organization Guidelines
1. Group changes into these developer-focused sections:
   - 🚨 **Breaking Changes** (if any - prominently at the top)
   - 🚀 **New Features** (new functionality that expands capabilities)
   - 🔧 **Improvements** (optimizations, enhancements, code quality)
   - 🐛 **Bug Fixes** (patches and corrections for issues)
   - 📦 **Dependencies** (dependency updates and management)
   - 🏗️ **Internal Changes** (refactoring, architecture, developer experience)
   - 📚 **Documentation** (README updates, inline docs, comments)

2. Present changes technically and precisely:
   - Mention specific technical details (command flags, config parameters)
   - For significant changes, include brief code examples or CLI usage examples
   - Highlight performance improvements with specifics where available
   - Clearly mark API changes or interface modifications

## Formatting Guidelines
1. Use GitHub-flavored markdown effectively:
   - Format issue references as #XX to create auto-links
   - Format PR references as #XX or GH-XX
   - Use code blocks with language specification for examples: \`\`\`go
   - Use collapsible sections for verbose changes

2. For each bullet point:
   - Begin with a technical, specific description (not marketing language)
   - Include the PR or issue numbers in format #XX when available in commit messages
   - Credit relevant authors for significant contributions
   - Explain why changes were made when that context is available
   - Link to relevant documentation when applicable

3. Technical focus:
   - Emphasize implementation details over marketing descriptions
   - Include information about internal mechanics where relevant
   - Mention command-line interface changes explicitly
   - Note configuration changes and their impact

4. Follow the style of previous release notes where available
   - Maintain the project's established conventions
   - Use similar technical depth and specificity"

        # Build content string conditionally before passing to jq
        USER_CONTENT="Write comprehensive release notes for version $NEW_VERSION of git-sweep-go based on these commits:\n\n$COMMITS_ESCAPED\n\nProject README Overview:\n$README_ESCAPED"
        
        # Add previous release notes if available
        if [[ -n "$PREV_NOTES_ESCAPED" ]]; then
            USER_CONTENT="$USER_CONTENT\n\nPrevious Release Notes Style Example:\n$PREV_NOTES_ESCAPED"
        fi
        
        # Add main file content if available
        if [[ -n "$MAIN_FILE_ESCAPED" ]]; then
            USER_CONTENT="$USER_CONTENT\n\nMain Program Code Context:\n$MAIN_FILE_ESCAPED"
        fi
        
        # Add remaining content
        USER_CONTENT="$USER_CONTENT\n\nProject Structure:\n$STRUCTURE_ESCAPED\n\nFocus on highlighting important changes, especially those that impact users. Group related changes into appropriate sections and follow the format guidelines."
        
        # Construct JSON payload with enhanced information
        jq -n \
            --arg version "$NEW_VERSION" \
            --arg user_content "$USER_CONTENT" \
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
                        "content": $user_content
                    }
                ],
                "temperature": 0.7,
                "max_tokens": 1500
            }' > "$TEMP_JSON"
        
        # Save a copy for error logging if needed
        ERROR_LOG_FILE=$(mktemp)
        cat "$TEMP_JSON" > "$ERROR_LOG_FILE"
        
        # Call the OpenAI API with the JSON from our temp file
        echo "Calling OpenAI API with enhanced information..."
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
            
            # Generate basic notes for comparison (old style)
            echo "Generating basic AI release notes for comparison..."
            TEMP_JSON_BASIC=$(mktemp)
            
            # For macOS compatibility
            if [[ "$OSTYPE" == "darwin"* ]]; then
                BASIC_COMMITS_ESCAPED=$(echo "$COMMIT_LOGS" | perl -pe 's/"/\\"/g' | perl -0pe 's/\n/\\n/g')
                BASIC_README_ESCAPED=$(head -n 5 README.md | perl -pe 's/"/\\"/g' | perl -0pe 's/\n/\\n/g')
            else
                BASIC_COMMITS_ESCAPED=$(echo "$COMMIT_LOGS" | sed 's/"/\\"/g' | sed ':a;N;$!ba;s/\n/\\n/g')
                BASIC_README_ESCAPED=$(head -n 5 README.md | sed 's/"/\\"/g' | sed ':a;N;$!ba;s/\n/\\n/g')
            fi
            
            # Create basic payload (similar to original script)
            # First construct the user content string before passing to jq
            BASIC_USER_CONTENT="Create release notes for version $NEW_VERSION of git-sweep-go based on these commits:\n\n$BASIC_COMMITS_ESCAPED\n\nProject description:\n$BASIC_README_ESCAPED\n\nThe project is a Git branch cleanup tool. Focus specifically on what these exact commits changed."
            
            jq -n \
                --arg user_content "$BASIC_USER_CONTENT" \
                '{
                    "model": "gpt-3.5-turbo",
                    "messages": [
                        {
                            "role": "system", 
                            "content": "You are a technical release note writer. Generate concise, professional release notes from Git commits. Group related changes into sections (Features, Bug Fixes, Documentation, etc). Use bullet points and keep the tone professional."
                        },
                        {
                            "role": "user",
                            "content": $user_content
                        }
                    ],
                    "temperature": 0.7,
                    "max_tokens": 1000
                }' > "$TEMP_JSON_BASIC"
            
            # Call API with basic information (for comparison)
            BASIC_API_RESPONSE=$(curl -s -X POST "https://api.openai.com/v1/chat/completions" \
                -H "Content-Type: application/json" \
                -H "Authorization: Bearer $OPENAI_API_KEY" \
                --data-binary "@$TEMP_JSON_BASIC")
            
            rm -f "$TEMP_JSON_BASIC"
            
            # Check if basic API call succeeded
            if echo "$BASIC_API_RESPONSE" | jq -e '.error' > /dev/null; then
                echo "⚠️ Failed to generate basic release notes for comparison."
                BASIC_AI_NOTES="[Failed to generate basic version for comparison]"
            else
                BASIC_AI_NOTES=$(echo "$BASIC_API_RESPONSE" | jq -r '.choices[0].message.content')
            fi
            
            # Show all three options
            echo -e "\n------- Enhanced AI-Generated Release Notes -------"
            echo "$AI_NOTES"
            echo -e "\n------- Basic AI-Generated Release Notes -------"
            echo "$BASIC_AI_NOTES"
            echo -e "\n------- Conventional Release Notes -------"
            echo "$CONVENTIONAL_NOTES"
            echo -e "\n-----------------------------------------"
            
            # Ask the user which version to use
            echo "Choose release notes format:"
            echo "1) Enhanced AI-generated"
            echo "2) Basic AI-generated"
            echo "3) Conventional (git log)"
            read -p "Select option [1-3]: " NOTES_CHOICE
            
            case $NOTES_CHOICE in
                1)
                    RELEASE_NOTES="$AI_NOTES"
                    echo "✓ Using enhanced AI-generated notes"
                    ;;
                2)
                    RELEASE_NOTES="$BASIC_AI_NOTES"
                    echo "✓ Using basic AI-generated notes"
                    ;;
                3)
                    RELEASE_NOTES="$CONVENTIONAL_NOTES"
                    echo "✓ Using conventional notes"
                    ;;
                *)
                    echo "Invalid choice, defaulting to enhanced AI-generated notes"
                    RELEASE_NOTES="$AI_NOTES"
                    ;;
            esac
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
echo "🧪 TEST MODE: Final Release Output 🧪"
echo "Ready to release (no actual changes will be made):"
echo "  Current version: $CURRENT_VERSION"
echo "  New version:     $NEW_VERSION"
echo "  Release notes:"
echo "$RELEASE_NOTES"
echo ""

# In the test mode, we won't actually create tags or publish
echo "🧪 TEST COMPLETE: In a real release, the following would happen:"
echo "- Git tag '$NEW_VERSION' would be created"
echo "- Tag would be pushed to remote"
echo "- Goreleaser would create and publish the release"
echo ""
echo "✅ Test complete! The script reached the end without errors."
