# temp

# todo

- filter not working correctly
- metadata
  - save in a sqlite file
  - save in postgres
  - save in yaml header in markdown files
  - get metadata for sqlite
  - get metadata for postgres
  - get metadata for yaml header
  - reduce metadata debugging logs
  - make updateUsedLinks work
  - change linkRegex config to names, e.g. obsidian, notion, dokuwiki... instead of a regex? or add one regex string + confignames
- git
  - move plugins/git to git
  - dont apply git settings on the fly only on startup
  - test external repo and fix it
  - if added without web interface but with a git commit, search commit message for --type ...
- error handling in settings (especially git settings)
- create folder structure for different file types
  - project (has board/boards)
    - board (has everything else besides project)
      - filter (which cards are displayed)
  - is displayed with filter/toc
    - todo
    - knowledge/note
    - journal
- add basic git functions
  - use git do display all files in the data folder
  - add api endpoint to create a new file (git add)
  - add api endpoint to get a git history
  - add api endpoint to rename a filename
- when is metadata endpoint called? (time interval?)
- add text editor
- add edit file
- create api endpoint for fulltext search
- users/groups/permissions?

# done

- create filter/toc
- add a markdown parser
- files system
  - api get all files + metadata
  - api get metadata for specific file
  - api get files with filter (maybe add later)
  - does the dataPath config even make sense?
- git
  - file history (changes, differences)
- metadata
  - create api endpoints to retrieve the metadata
  - metadata for json, markdown header, sqlite, postgresql
  - metadata struct:
  - save in a json file
  - get metadata for json
- what about the filepath (e.g. http://localhost:1324/api/files/content/guides/developer-setup.md)
- add style for filepath (e.g. http://localhost:1324/api/files/content/guides/developer-setup.md)
- change the filter to return html instead of json
- update swagger comments
- is the data folder config neccessary?
- testgit is executed with every make dev command ..
- change handlers to set all handlers in api.go without creating a xxx.handler file..
- git
  - latest changes (latest changed files)
- files system
  - create internal/files folder
  - remove current file implementation: files dont return list
  - create file struct: name, path, metadata
  - new api route: files
  - api get all files (return list of file structs)
  - api get converted html for specific file (return html content)
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
