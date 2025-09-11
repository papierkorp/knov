# Technical Documentation

## Architecture Overview

Our system consists of several interconnected components that work together to provide a seamless experience.

### Core Components

1. **Backend API** - Go-based REST API
2. **Frontend** - HTML/HTMX interface
3. **Database** - Metadata storage
4. **Git Integration** - Version control

### API Endpoints

    GET /api/files/list
    POST /api/files/filter
    GET /api/metadata/{filepath}

## Database Schema

| Table   | Purpose        | Related Docs            |
| ------- | -------------- | ----------------------- |
| files   | File metadata  | [[project-overview.md]] |
| filters | Search filters | [[troubleshooting.md]]  |

### Configuration

The system can be configured via config.json:

    {
      "database": {
        "type": "sqlite",
        "path": "./data.db"
      },
      "features": {
        "search": true,
        "filters": true
      }
    }

## Development Setup

1. Install dependencies: `go mod download`
2. Run tests: `make test`
3. Start server: `make dev`

> **Warning**: Make sure to read [[troubleshooting.md]] before deploying to production.

## See Also

- [[getting-started.md|Getting Started Guide]]
- [[meeting-notes.md]] for recent decisions
- [[guides/developer-setup.md|Developer Setup Guide]]
