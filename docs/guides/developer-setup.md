# Developer Setup Guide

## Prerequisites

- Go 1.21 or later
- Git
- Make
- Swag CLI: `go install github.com/swaggo/swag/cmd/swag@latest`
- MinGW-w64 (for Windows cross-compilation): `sudo apt-get install mingw-w64` (Linux) or `brew install mingw-w64` (macOS)


## Quick Start

Clone and setup:

```bash
git clone https://github.com/papierkorp/knov.git
cd knov
go mod download
```

Install required tools:

```bash
go install github.com/swaggo/swag/cmd/swag@latest
go install golang.org/x/text/cmd/gotext@latest
```

Start development server:

```bash
make dev
```

## Development Workflow

1. Make changes to code
2. Run tests: `make test`
3. Check formatting: `make fmt`
4. Submit pull request

## Project Structure

- `cmd/` - Application entry point
- `internal/` - Core business logic
- `themes/` - UI themes and templates
- `static/` - Static assets
- `config/` - Configuration files

## Testing

    # Run all tests
    make test

    # Run specific test package
    go test ./internal/files

    # Run with coverage
    go test -cover ./...

## Database Setup

The system uses JSON files for metadata storage by default. For production:

    {
      "metadata": {
        "storagemethod": "sqlite"
      }
    }

## API Documentation

- Swagger UI: http://localhost:1324/swagger/
- API Playground: http://localhost:1324/playground

## Related Documentation

- [[../technical-documentation.md|Technical Documentation]]
- [[../project-overview.md|Project Overview]]
- [[../troubleshooting.md|Troubleshooting]]
