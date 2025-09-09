# Configuration

Configuration is stored in `config/config.json`.

## Themes

- `themes.currentTheme` (string) - Active theme name (default: `"builtin"`)

## General

- `Language` (string) - UI language code (default: `"en"`)
- `LogLevel` (string) - Set the amount of Logs received gets less in order: `debug`, `info`, `warning`, `error`

## Git

If no repositoryURL is provided a new local git repository will be initiated.

- `git.repositoryUrl` (string) - Repository URL to clone. If empty, creates new repo (default: "local")

## Metadata

- `metadata.storagemethod` (string) - Storage method for file metadata (default: `"json"`)
  - Available options: `"json"`, `"sqlite"`, `"postgres"`, `"yaml"`

## Data Folder

The default Datafolder will be "data" in the Application Binary but can be set with the ENV Var: `KNOV_DATA_PATH`
