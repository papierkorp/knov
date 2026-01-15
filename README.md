# temp

# todo

- rework docs folder manually without ai
  - first set the files to be created (install.md, dev-guide.md, ai.md, features/concept.md,example_workflows.md)
  - combine with help.gohtml of builtin theme
  - use the docs folder as testdata and remove the internal/testdata/testfiles
- sqlite metadata db - collection is a path should be a folder
- PARA Metadata - should create a folder and if one is selected the other cant be selected
- if a file is moved (git..) look at linksto and in the target file  change the link
- Dashboard
  - make the positions work with a custom layout work
  - Add widget drag & drop reordering
  - Implement widget resizing
  - Add dashboard export/import
- metadata
  - save in a sqlite file
  - save in postgres
  - save in yaml header in markdown files
  - get metadata for sqlite
  - get metadata for postgres
  - get metadata for yaml header
  - change linkRegex config to names, e.g. obsidian, notion, dokuwiki... instead of a regex? or add one regex string + confignames
- search
  - optimise with sqlite.. (same as metadata)
- performance updates
  - add caching/indexing
  - use Query() instead of a loop through files.GetAllFiles()
  - use Query in filter.go
Then: Consider adding Query() back
Finally: Refactor filter.go to use it

# done
- display the content of multiple files (filter) in the dashboard (maybe use display content in filterview)
- edit just a header section instead of the whole file
- mkdir/copy on windows not working
- func getEditor(filepath string) -> switch metadata.FileType + handleFileEdit - switch editor
- add todo filetype
- [x] users/groups/permissions? - canceled
- move templ to go html templates
- deliver everything in builtin template with htmx (no forms... and so on in the html)
- add a editor (textbox) and the neccessary form e.g. parents, collection
- create folder structure for different file types
  - project (has board/boards) => own repository for each project
    - board (has everything else besides project) => displays filter and files
      - filter (which cards are displayed)
  - is displayed with filter/toc
    - todo
    - knowledge/note
    - journal
- [x] error handling in settings (especially git settings)
- [x] return filter form from api?
- [x] auto update
  - [x] startup metadata handling
    - [x] add MetaDataInitializeAll() to startup sequence
    - [x] add MetaDataLinksRebuild() to startup sequence
    - [x] add search.IndexAllFiles() to startup sequence
  - [x] periodic metadata scan
    - [x] create timer/goroutine for periodic checks (every 5-10 minutes)
    - [x] run same operations as startup: metadata init, links rebuild, search reindex
    - [x] add logging for periodic scan operations
- Dashboard
  - make it work for go html template
  - Core Structure
    - [x] Create dashboard data structure (widgets, layout, filters)
    - [x] Add dashboard CRUD API endpoints (create, read, update, delete)
    - [x] Create basic dashboard storage (JSON file initially)
    - [x] Add dashboard management to user settings
  - Widget System
    - [x] Design widget configuration structure (filter + display method + size)
    - [x] Create widget types (list, cards, content preview)
    - [x] Implement widget rendering system
    - [x] Add widget CRUD operations
  - UI
    - [x] Create dashboard template/view
    - [x] Add dashboard selection/switching UI
    - [x] Implement basic layouts (1-column, 2-column, 3-column)
    - [x] Add widget container rendering
    - [x] Create dashboard editor interface
    - [x] Add widget creation form (filter selection, display type)
    - [x] Add widget edit/delete functionality
- [x] git
  - [x] move plugins/git to git
  - [x] dont apply git settings on the fly only on startup
  - [x] test external repo and fix it
  - [x] if added without web interface but with a git commit, search commit message for --type ... => new to do startup + periodic scan
- make links visible in files view
- create api endpoint for fulltext search
- filter not working correctly
- create filter/toc
- add a markdown parser
- add basic git functions
  - use git do display all files in the data folder
  - add api endpoint to create a new file (git add)
  - add api endpoint to get a git history
  - add api endpoint to rename a filename
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

# prompt

```
- im currently working on the following golang, htmx, templ app.
- i want you to anwser with as little code as possible to only fix the problem i anwsered without any unecessary code, as simple and small as possible with as few changes as possible
- for logging message i only want use lowercase
- if you create an api call keep in mind to keep it theme friendly (lean more towards being generic) and also add comments for swagger to work, also stay with accept form data we dont need to accept json
- for styles/css files use Global styles only in style.css and all specific files use ID selectors (#page-, #component-, #view-)
- if you change something significant also make the neccessary changes in the docs folder
- think theme-agnostic
- always search the project since you already have all the files
- show me the code of what you actual changed, i dont want scroll through the whole artifact to search for your changes

filetree
```
