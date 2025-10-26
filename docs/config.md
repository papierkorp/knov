# Configuration Guide

KNOV supports configuration through environment variables and configuration files.

## Environment Variables

### Server Configuration

- `KNOV_SERVER_PORT`: HTTP server port (default: 1324)
- `KNOV_LOG_LEVEL`: Logging level (debug, info, warning, error)

### Path Configuration

- `KNOV_DATA_PATH`: Directory containing your knowledge base files (default: "data")
- `KNOV_THEMES_PATH`: Directory containing theme .so files (default: "themes")
- `KNOV_CONFIG_PATH`: Directory for configuration and user settings (default: "config")

### Storage Configuration

- `KNOV_STORAGE_METHOD`: Storage backend (default: "json")

Storage files are created in `{KNOV_CONFIG_PATH}/.storage/`

### Git Configuration

- `KNOV_GIT_REPOSITORY`: Git repository URL for your knowledge base

## Configuration Files

User settings are stored in JSON format at:
`{KNOV_CONFIG_PATH}/users/{userid}/settings.json`

Example user settings:

```json
{
  "theme": "builtin",
  "language": "en",
  "fileView": "detailed",
  "darkMode": false,
  "colorScheme": "default",
  "customCSS": ""
}
```

## Deployment Examples

### Development

```bash
export KNOV_LOG_LEVEL=debug
go run .
```

### Production with Custom Paths

```bash
export KNOV_DATA_PATH=/var/lib/knov/data
export KNOV_THEMES_PATH=/usr/local/knov/themes
export KNOV_CONFIG_PATH=/etc/knov
export KNOV_SERVER_PORT=8080
./knov
```

### Docker Deployment

```dockerfile
ENV KNOV_DATA_PATH=/app/data
ENV KNOV_CONFIG_PATH=/app/config
ENV KNOV_THEMES_PATH=/app/themes
VOLUME ["/app/data", "/app/config"]
```

## Theme Configuration

Themes are automatically discovered from `{KNOV_THEMES_PATH}/*.so` files.

The builtin theme is always available and embedded in the binary.

Upload new themes via the admin interface at `/admin`.
