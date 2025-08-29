# Developer Guide

## Prerequisites

```bash
go install github.com/a-h/templ/cmd/templ@latest
go install github.com/swaggo/swag/cmd/swag@latest
go install golang.org/x/text/cmd/gotext@latest
```

## Development

```bash
make dev     # Start development server
make prod    # Build production binary
```

Server runs on `http://localhost:1324`

## Usage

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

### Config Manager

See `internal/configmanager/configmanager.go` for methods like `GetLanguage()`, `SetLanguage()`.

### Theme Manager

See `internal/thememanager/thememanager.go` and `internal/thememanager/README.md` for theme development.

## File Structure

- `themes/` - Theme plugins (compiled to `.so` files)
- `config/` - Configuration and custom CSS
- `internal/` - Core application logic
- `static/` - Static assets
