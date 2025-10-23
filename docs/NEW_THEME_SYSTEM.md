# new go template theme system

the thememanager has been rewritten to use go's html/template instead of templ. this allows themes to be changed at runtime without recompilation.

## how it works

1. themes are stored in `themes/` directory as folders
2. each theme folder contains `.gotmpl` template files
3. themes are loaded automatically when the app starts
4. users can switch themes via the settings page

## theme structure

```
themes/mytheme/
├── base.gotmpl           # base layout template
├── home.gotmpl           # home page
├── settings.gotmpl       # settings page
├── admin.gotmpl          # admin page
├── playground.gotmpl     # playground page
├── history.gotmpl        # history page
├── overview.gotmpl       # overview page
├── search.gotmpl         # search page
├── fileview.gotmpl       # file viewer
├── fileedit.gotmpl       # file editor
├── dashboard.gotmpl      # dashboard
├── browsefiles.gotmpl    # browse files
├── latestchanges.gotmpl  # latest changes
├── theme.json            # theme metadata
└── style.css             # theme styles
```

## required templates

all themes must include these template files:
- base.gotmpl
- home.gotmpl
- settings.gotmpl
- admin.gotmpl
- playground.gotmpl
- history.gotmpl
- overview.gotmpl
- search.gotmpl
- fileview.gotmpl
- fileedit.gotmpl
- dashboard.gotmpl
- browsefiles.gotmpl
- latestchanges.gotmpl

## template data

templates receive data in this structure:
```go
{
    "Content": interface{},  // page-specific data
    "Theme": string,         // current theme name
}
```

## creating themes

1. copy the `themes/default/` folder
2. rename it to your theme name
3. edit the templates and css files
4. update `theme.json` with your theme info
5. restart the app or use the theme upload feature

## key changes from templ

- no more plugin compilation required
- themes can be switched at runtime
- simpler template syntax using go templates
- no more builtin theme - all themes are equal
- themes are just folders with template files

## migration

existing templ-based themes need to be converted to go templates. the example default theme shows the basic structure and template syntax.
