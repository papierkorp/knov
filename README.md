# temp

# todo

- change thememanger to api
- change configmanager to api
- add a translator
- add GET api/themes to api (show all avaiable themes)
- add POST api/themes/{themeName} (switch to theme)
- add GET api/currentTheme
- add custom.css panel to settings
- make basic style for settings
- add api to docs and thememanager readme
- init new git repo in data folder
- add a markdown parser
- create folder structure for different file types
  - project (has board/boards)
    - board (has everything else besides project)
      - filter (which cards are displayed)
      - toc (like filter)
  - is displayed with filter/toc
    - todo
    - knowledge
    - journal
- use git do display all files in the data folder
- create api endpoints to save the metadata
  - in a sqlite file
  - in postgres
  - in a json file
  - as metadata in the markdown files
- when is metadata endpoint called? (time interval?)
- create api endpoints to retrieve the metadata
- create filter/toc
- add api endpoint to create a new file (git add)
- add api endpoint to get a git history
- add api endpoint to rename a filename
- add text editor
- add edit file
- create api endpoint for fulltext search
- users/groups/permissions?

# done

- add a api
- use htmx
- add settings route
- load defaulttheme/theme from settings.json instead of hardcoded in thememanager => configmanager
- make style.css and custom.css work
- add a flexible thememanager with 2 themes
