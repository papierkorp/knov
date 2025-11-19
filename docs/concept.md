# KNOV - Knowledge Management System

KNOV is a flexible knowledge management system built with Go, HTMX, and Templ that supports multiple themes and deployment configurations.

## Core Features

- **File Management**: Support for Markdown, DokuWiki, and plain text files
- **Theme System**: Plugin-based themes with builtin theme embedded by default
- **Search**: Multiple search backends (memory, grep, SQLite)
- **Git Integration**: Version control for your knowledge base
- **Dashboard System**: Customizable dashboards with widgets
- **Multi-language Support**: English and German translations
- **Metadata**: each file can get a lot of metadata for searching/filtering (but its not forced)
  - **Tags**: fully customizable tags
  - **PARA**: implemented PARA Method - you can attach each File with its corresponding PARA with multiple PARAS possible
  - **ZK**: Different Filetypes accordingly to the ZK Method - can be used with or without PARA
  - **collection**: organizational field to group related files - defaults to the first folder in filepath or "default" - can be changed manually

## Architecture

- **Backend**: Go with Chi router
- **Frontend**: HTMX + Templ templates
- **Themes**: Go plugins with embedded assets
- **Storage**: JSON-based configuration storage
- **Search**: Pluggable search engines

## Theme System

KNOV uses a dual approach for themes:

### Builtin Theme

- Embedded directly in the binary
- No external dependencies
- Always available
- Self-contained with embedded CSS and templates

### Plugin Themes

- Loaded as .so files
- Can embed their own CSS and assets
- Uploadable via admin interface
- Hot-swappable without restart

## Deployment

KNOV can be deployed as a single binary with configurable paths:

- `KNOV_DATA_PATH`: Where your content files are stored
- `KNOV_THEMES_PATH`: Where theme .so files are located
- `KNOV_CONFIG_PATH`: Where configuration and user settings are stored
- `KNOV_SERVER_PORT`: HTTP server port

Static assets and builtin theme assets are embedded in the binary for portable deployment.
