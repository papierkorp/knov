# Theme Development Guide

## Creating a New Theme

To create a new theme for the application:

1. **Create a new folder** in the `themes/` directory with your theme name (e.g., `themes/mytheme/`)

2. **Create a main.go file** in your theme folder with the following structure:
```go
package main

import (
    "github.com/a-h/templ"
    "knov/internal/thememanager"
)

type MyTheme struct{}

var Theme MyTheme

func (t *MyTheme) Home() (templ.Component, error) {
    return nil, nil
}

func main() {}
```

3. **Implement the ITheme interface** - currently requires:
   - `Home() (templ.Component, error)` method

4. **Create a templates/ folder** in your theme directory for your `.templ` files

5. **Add style.css** in your templates folder for theme-specific styling

6. **Use the build system** - run `make dev` to compile and load your theme

## Theme Structure
```
themes/
└── mytheme/
    ├── main.go
    └── templates/
        ├── home.templ
        └── style.css
```

The theme system uses Go plugins, so your theme will be compiled to `mytheme.so` and loaded dynamically at runtime.
