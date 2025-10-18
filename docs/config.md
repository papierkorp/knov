# Configuration Guide

KNOV supports configuration through environment variables and configuration files.

## Environment Variables

### Server Configuration
- `KNOV_SERVER_PORT`: HTTP server port (default: 1324)
- `KNOV_LOG_LEVEL`: Logging level (debug, info, warning, error)

### Path Configuration
- `KNOV_DATA_PATH`: Directory containing your knowledge base files (default: "data")
- `KNOV_CONFIG_PATH`: Directory for configuration and user settings (default: "config")

**Note**: Themes are automatically stored in `{KNOV_CONFIG_PATH}/themes/` and cannot be configured separately.

### Storage Configuration
- `KNOV_STORAGE_METHOD`: Storage backend (default: "json")

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
export KNOV_CONFIG_PATH=/etc/knov
export KNOV_SERVER_PORT=8080
./knov
# Themes will be in /etc/knov/themes/
```

### Docker Deployment
```dockerfile
ENV KNOV_DATA_PATH=/app/data
ENV KNOV_CONFIG_PATH=/app/config
VOLUME ["/app/data", "/app/config"]
# Themes will be automatically stored in /app/config/themes/
```

## Theme Configuration

### Theme Storage Location

Themes are stored in: `{KNOV_CONFIG_PATH}/themes/`

```
config/
└── themes/
    ├── builtin/          # Extracted automatically from binary on first run
    ├── custom-theme/     # Your uploaded themes
    └── another-theme/
```

### Theme Management

1. **Builtin Theme**: Automatically extracted to `config/themes/builtin/` on first startup
2. **Upload Themes**: Use the admin interface at `/admin` to upload `.tar.gz` theme packages
3. **Manual Installation**: Extract theme archives directly to `config/themes/{theme-name}/`

### Theme Format

Themes are packaged as `.tar.gz` archives containing:
- `theme.json` - Theme metadata
- `templates/*.html` - HTML template files
- `static/` - CSS, JavaScript, fonts, etc. (optional)

See the [Theme Creator Guide](theme-creator-guide.md) for detailed information on creating themes.
