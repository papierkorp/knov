# Theme Development Guide

The theme system uses Go plugins, so your theme will be compiled to `mytheme.so` and loaded dynamically at runtime.

## Creating a New Theme

To create a new theme for the application:

1. Create a new folder in the `themes/` directory with your theme name (e.g., `themes/mytheme/`)
2. Create a main.go file in your theme folder which implements the ITheme interface. Use builtin as an example
3. Create a `templates/` folder in your theme directory for your `.templ` files
4. Add style.css in your templates folder for theme-specific styling, the path has to be correct
5. Use the build system - run `make dev` to compile and load your theme

## Theme Structure example

```
themes/
└── mytheme/
    ├── main.go
    └── templates/
        ├── home.templ
        └── style.css
```

## Available functions/api

Take a look at `internal/server/server.go` at the implemented routes and the functions used for these routes.
