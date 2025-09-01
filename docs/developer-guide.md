# Developer Guide

## Prerequisites

```bash
go install github.com/a-h/templ/cmd/templ@latest
go install github.com/swaggo/swag/cmd/swag@latest
go install golang.org/x/text/cmd/gotext@latest
```

## Development

```bash
make dev     # Start development server with test data
make prod    # Build production binary
```

Server runs on `http://localhost:1324`

## Test Data Setup

The development environment includes automatic test data setup which can be manually called with:

```bash
make setup-test-data    # Create test files and git operations
make clean-test-data    # Remove all test data
```

but is also include in the `make dev` command.

This creates sample markdown files in the `data/` directory and initializes a git repository with commit history for testing git operations.

## Usage

### File Management

The application provides both filesystem and git-based file operations:

**Core Functions:**

- `files.GetAllFiles()` - Scan filesystem for .md files
- `files.GetFileContent(path)` - Convert markdown to HTML
- `files.GetAllFilesWithMetadata()` - Files with metadata (TODO)
- `files.GetFileMetadata(path)` - Single file metadata (TODO)

**API Endpoints:**

```
GET  /api/files/list                 # List all files
GET  /api/files/content/{filepath}   # Get HTML content
GET  /api/files/metadata             # All files with metadata
GET  /api/files/metadata/{filepath}  # Single file metadata
```

### Git Operations

Git integration provides version control for markdown files:

**Core Functions:**

- `files.GetRecentlyChangedFiles(count)` - Recent git history
- `files.GetFileDiff(path)` - Git diff for file
- `files.AddFile(path)` - Add and commit single file
- `files.AddAllFiles()` - Add and commit all files
- `files.DeleteFile(path)` - Remove and commit file deletion

**API Endpoints:**

```
GET    /api/files/git/history?count=N  # Recent changes (default: 10)
GET    /api/files/git/diff/{filepath}  # File diff from last commit
POST   /api/files/git/add/{filepath}   # Add and commit file
POST   /api/files/git/addall           # Add and commit all files
DELETE /api/files/git/delete/{filepath} # Delete and commit removal
```

**Example Usage:**

```bash
# Get last 5 changed files
curl "http://localhost:1324/api/files/git/history?count=5"

# Get diff for specific file
curl "http://localhost:1324/api/files/git/diff/ai.md"

# Add new file to git
curl -X POST "http://localhost:1324/api/files/git/add/newfile.md"

# Add all files
curl -X POST "http://localhost:1324/api/files/git/addall"

# Delete file from git
curl -X DELETE "http://localhost:1324/api/files/git/delete/oldfile.md"
```

### Translation

Use `translation.Sprintf("text")` in `.templ` files. See `themes/builtin/templates/home.templ` for examples.

**Workflow:**

1. Add `translation.Sprintf("Your text")` to templates
2. Run `make translation` to extract strings and generate catalogs
3. Edit `internal/translation/locales/{lang}/messages.gotext.json` to add translations
4. Run `make translation` again to update catalogs
5. Existing translations in `messages.gotext.json` are preserved, new strings are added untranslated

### API

Check `http://localhost:1324/swagger/` for documentation or see `internal/server/api*.go` files.

**Interactive Testing:**
Visit `http://localhost:1324/playground` for an interactive API testing interface with examples for all file and git operations.

### Config Manager

See `internal/configmanager/configmanager.go` for methods like `GetLanguage()`, `SetLanguage()`.

### Theme Manager

See `internal/thememanager/thememanager.go` and `internal/thememanager/README.md` for theme development.

## File Structure

- `themes/` - Theme plugins (compiled to `.so` files)
- `config/` - Configuration and custom CSS
- `internal/` - Core application logic
  - `files/` - File and git operations
    - `files.go` - Filesystem operations
    - `git.go` - Git operations and API handlers
  - `configmanager/` - Configuration management
  - `server/` - HTTP server and API routes
  - `thememanager/` - Theme system
  - `translation/` - Internationalization
- `static/` - Static assets
- `data/` - Markdown files (git repository)

## Development Workflow

1. **Start development**: `make dev`
2. **Test APIs**: Visit `/playground` for interactive testing
3. **View files**: Visit `/` to see file browser
4. **Configure**: Visit `/settings` for configuration
5. **API docs**: Visit `/swagger/` for complete API documentation

## Git Integration

The application automatically initializes a git repository in the `data/` directory. All file operations through the git API include automatic commits with descriptive messages:

- Adding files: `"add file: filename.md"`
- Adding all: `"add all files"`
- Deleting: `"delete file: filename.md"`

This provides full version control for your markdown content with API-driven operations.
