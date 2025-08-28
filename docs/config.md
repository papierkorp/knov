# Configuration

Configuration is stored in `config/config.json`.

## Themes

- **`themes.currentTheme`** (string) - Active theme name (default: `"builtin"`)

## General

- **`Language`** (string) - UI language code (default: `"en"`)

## Git

If no repositoryURL/local is provided an new local git repository will be initiated.

- **`git.repositoryUrl`** (string) - Repository URL to clone. If empty, creates new repo (default: "local")
- **`git.dataPath`** (string) - Path where git repository will be created/cloned (default: "data")
