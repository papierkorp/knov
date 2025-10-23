# roadmap

1. basic tm which can load 2 different themes with this folder structure:
  - themes/builtin/base.gotmpl
  - themes/anotherone/base.gotmpl
2. tm which can load 2 different themes with a metadata.json files:
  - themes/builtin/base.gotmpl
  - themes/builtin/theme.json
  - themes/anotherone/base.gotmpl
  - themes/anotherone/theme.json
3. make base.gotmpl required and add a errorhandler which prints if the required file is not there

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
