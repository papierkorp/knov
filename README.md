# temp

# todo

- files system
  - create internal/files folder
  - remove current file implementation: files dont return list
  - create file struct: name, path, metadata
  - new api route: files
  - api get all files (return list of file structs)
  - api get converted html for specific file (return html content)
  - metadata for inFile (markdown header), sqlite, postgresql
  - api get all files + metadata
  - api get metadata for specific file
  - api get files with filter (maybe add later)
- error handling in settings (especially git settings)
- create folder structure for different file types
  - project (has board/boards)
    - board (has everything else besides project)
      - filter (which cards are displayed)
      - toc (like filter)
  - is displayed with filter/toc
    - todo
    - knowledge
    - journal
- add basic git functions
  - use git do display all files in the data folder
  - add api endpoint to create a new file (git add)
  - add api endpoint to get a git history
  - add api endpoint to rename a filename
- create api endpoints to save the metadata
  - in a sqlite file
  - in postgres
  - in a json file
  - as metadata in the markdown files
- when is metadata endpoint called? (time interval?)
- create api endpoints to retrieve the metadata
- create filter/toc
- add a markdown parser
- add text editor
- add edit file
- create api endpoint for fulltext search
- users/groups/permissions?

# done

- create a setting to init a new git repo in data folder/set git url
- add api to docs and thememanager readme
- add custom.css panel to settings
- make basic style for settings
- add a translator
- add GET api/themes to api (show all avaiable themes)
- add POST api/themes/{themeName} (switch to theme)
- add GET api/currentTheme
- change configmanager to api
- change thememanger to api
- add a api
- use htmx
- add settings route
- load defaulttheme/theme from settings.json instead of hardcoded in thememanager => configmanager
- make style.css and custom.css work
- add a flexible thememanager with 2 themes

# dev

```bash
go install github.com/a-h/templ/cmd/templ@latest
go install github.com/swaggo/swag/cmd/swag@latest

```
