# roadmap

1. basic tm which can load 2 different themes with this folder structure:
  - themes/builtin/base.gotmpl
  - themes/builtin/settings.gotmpl
  - themes/test/base.gotmpl
  - themes/test/settings.gotmpl
2. let me change the theme in settings.gotmpl
3. add metadata struct with a theme.json and it still works changing the themes
  - themes/builtin/theme.json
  - themes/test/theme.json
4. make base.gotmpl and settings.gotmpl required and add a errorhandler which prints if the required file is not there (failing doesnt have a settings.gotmpl) - validateTheme, validateTemplate
  - themes/failing/theme.json
  - themes/failing/base.gotmpl
5. pass a function (funcmap) to the theme and make translation work
6. add different views for base.gotmpl (e.g. "default", "advanced")
7. bundle builtin into the app and unpack it on startup and make it default
8. add optional Templates and all neccessary Templates (admin, browse, dashboard, fileedit, fileview, history, latestchanges, overview, playground, search, settings)
8. add a overwrite (e.g. if the themename is "overwrite" use every template in it, but i doesnt need to be a full theme)

**xx**
- theme (folder in /themes has to fullfill some requirements e.g. specific templates and theme.json)
- template (main part of the theme)
- view (each template can have different views)
- routes (given by the server manager, each route points to a specific template)

format/parts: init, manager, getter/setter, errorhandling, utils

# old

**vars**
- globalThemeManager *ThemeManager

**Functions**
- Init (tm.NewThemeManager, tm.Init)
- GetThemeManager
- NewThemeManager
- Interface
  - Initialize()
  - GetCurrentTheme() ITheme
  - GetCurrentThemeName() string
  - SetCurrentTheme(name string) error
  - GetAvailableThemes() []string
  - LoadTheme(themeName string) error
  - LoadAllThemes() error
  - GetAvailableViews(viewType string) []string
  - GetThemeMetadata(themeName string) *ThemeMetadata
  - CompileThemes() error
- registerBuiltinTheme

**Types**
- ThemeManager (themes, currentTheme, thememetadata)
- ITheme (Home, Settings, Admin..)
- ColorScheme
- ThemeMetadata (AvailableFileViews...,SupportsDarkMode, AvailableColorSchemes)
- IThemeManager


# example

**vars**
- globalThemeManager
- RequiredTemplates
- OptionalTemplates
- PredefinedCategories

**Functions**
- Init (NewThemeManager, initBuiltin, loadthemes, setcurrenttheme)
- GetThemeManager
- NewThemeManager
- LoadThemes
- loadTheme
- setDefaultViews
- loadThemeTemplates
- contains
- SetTheme
- GetThemeNames
- GetCurrentTheme
- GetCurrentThemeName
- hasTheme
- HasTemplate
- Render
- ApplyThemeOverwrites
- loadThemeTemplatesWithOverwrite
- GetAvailableViews
- GetAvailableThemes
- NewHomeContent
- NewFileViewContent
- ...
- GetThemeMetadata
- LoadTheme
- SetCurrentTheme

**Types**
- ThemeManager (themes, currentTheme, themesPath)
- Theme (Name, Path, Metadata, Template)
- ThemeMetadata (Name, Version, Author, Categories, Views, Features)
  - AvailableViews
  - ThemeFeatures
    - ColorScheme
- HomeContent
- FileViewContent
- OverviewContent
- ..

# current

**vars**
- RequiredTemplates

**Functions**
- Error
- NewThemeManager
- LoadThemes
- loadTheme
- validateThemeFiles
- validateParsedTemplates
- SetTheme
- GetThemeNames
- GetCurrentTheme
- HasTemplate
- Render

**types**
- ThemeManager
- Theme
- ThemeMetadata
- ThemeData
- ThemeValidationError
