# daily

single source of truth is the metadata - we just display it differently

## new functions

### sync function

is called for every save in /daily, /kanban and /calendar

1. change metadata (either status, targetDate/todoDate or parent) based on give parameter
2. hx-reload /kanban, /daily and /calendar

### daily function

is called for every load/reload of /daily

1. 

## daily

**routes**

- /daily
- /daily/<date> (show file of this day in readonly)
- /api/daily/<date>/content
- /api/daily/<date>/sidebar
- /api/daily/<date>/parse
- /daily/overview (shows a date range input and then display the content of all dailyfiles in this range in one long output)
- /api/daily/save

**views**


which parser to use (custom search for the links or plain markdown)

- todo
  - only available if sync is activated
  - custom daily parser
- note
  - markdown parser


**on load/reload**

the file is saved in data/daily/<date>.md, look in the data/daily folder for the current date if there is a file

- if there is no file and dailysync is activated:
  - search for todos for today and add them all to the markdown file (metadata TargetDate/todoDate) accordingly to the parser and with regex based on the status
- if there is no file and dailysync is deactivated:
  - create a new emtpy file
- if there is a file and dailysync is activated:
  - compare every link in the file with every todo of today and set the regex correctly
- if there is a file ad dailysync is deactivated:
  - load the existing file in edit mode

**parser**

if dailysync is activated use the new parser:

- /daily as new filetype with a new parser as a markdown file (extended markdown)
- 3 h1 header:
  - today:
    - existing todo files with metadata targetdate today
    - status??
  - notes:
    - existing files with metadata targetdate today
    - status??
    - type fleeting
  - done:
    - if moved to here => set status done
    - status done
- each h1 header will have a list of (internal) links
- these links will automatically switch if they are moved in the kanban board
- or move in the kanban board if they are changed and saved here
- remove the link if the date is changed in the calendar
- each link in this file will have the daily file as its parent
- and leave everything thats not a link in a list in one of these 3 headers alone so the user can comment or add info

else use default markdown parser

**display**

- sidebar
  - shows links to the last 10 files
  - link to calendar
  - link to canban
  - edit neccessary metadata
  - quick add of todo with targetDate and status
- old daily files
  - are read only
  - links will never be updated (even if the note file is deleted)

**event trigger/on reload - sync**

if dailysync is activated:

- status change in kanban = update links based on regex => just reload /daily since it should load based on metadata
- targetDate change in calendar => remove/add the link and remove/add Parentlink

else

- do nothing

**on save**

- check links:
  - for new links: add current daily file as parent to the link file
  - for existing links: do nothing
  - for removed links: remove the current daily file as parent, remove targetDate/todoDate from metadata from linkfile and reload /calendar and /kanban
  - for every link check regex (parser/regexsetting) => update metadata status of these files and reload /kanban

**new settings**

- dailysync boolean enabled/disabled - env
- dailyregexposition: before/after as env var
- regex for the parser as env vars (text/char before or after the link for the kanban board - can be changed by the user)
  - regextodo - base: todo, o
  - regexwaiting - base: waiting, ~
  - regexdoing - base: doing, +
  - regexdone - base: done, x

## kanban

**routes**

- /kanban
- /kanban/<date> / /canban/from:<date>to:<date> (maybe with a neededDays/workDays parameter, so targetdate +x days)
- /api/kanban/board (with a parameter scope: for scope of today/week/month based on setting)
- /api/kanban/move
- /api/kanban/save

**views**

(which data to display)

- dailytodo
  - only available if sync is activated
  - sync the links in the file with the board (single source of truth is metadata)
- filtertodo
  - reuse filter

**parser**

-

**on load**

**display**

- boards: (with sort + hide button)
  - todo
  - doing
  - waiting
  - done
- xxx columns based on view
- + / add button: quick add of todo with targetDate and status
- how to do it with cards/what to display... (do i get /api/kanban/card/<id> route?..)

**event trigger/on reload - sync**

- status change in kanban (link moved to another header) => move in kanban board..
- targetDate change in calendar => remove/add to board

**on save**

- when card moved => update metdata status of file and reload /daily

**new settings**

- show todos of today/this week/this month

## calendar

**routes**

- /calendar (save the view parameter in settings)
- /api/calendar/view/{type} (day/week/month)
- /api/calendar/month/<date>
- /api/calendar/week/<date>
- /api/calendar/day/<date>
- /api/calendar/save
- /api/calendar/move

**views**

which data to display

- dailytodo
  - show (parsed) content of daily folder for this date => links from daily file based on metadata status
  - show filenames/paths links
- normal
  - show based on target date + metadata status
  - show filenames/paths links

**on load**

**parser**

**display**

- +/add button quick add of todo with targetDate and status
- without a time for now
- view based on htmx call

**event trigger/on reload - sync**

**on save**

- allows drag and dropping the filenames into another date which changes the targetDate/todoDate
- if targetDate/todoDate changed => reload /daily and /kanban

**new settings**

## additional

additional neccessary changes

- rename targetDate to todoDate
- when creating a fleeting note - add a targetdate of + 1 week
- change the status (which isnt used at the moment) to the kanban status
- add a new metadata like neededDays/workDays or something like this (so i can have a range for targetDate e.g. targetdate+neededDays/workDays = date range)
- in server.go in /files/new/todo add parameter for quick adds for daily/calendar/kanban which automatically adds a targetDate of today and a status
- syncViews(fileID, changeType) function which is called in handleAPIxxxSave for all 3 new saves

# Relationship Tracker

- create a Person
- last contact
- ...

# stream of consciousnes

1. new filetype soc
2. can be attached to other files => then filename = otherfilename_soc.md
3. input directly under <main> with 2 buttons on the right end side of the input:
  4. a expand button (per default only the input field is shown) if expanded show the last x entries depending on the styl
  5. a menu button which shows a pop up with 
    6. all available socs for this file with a ok icon to which file is currently selected
    7. a attach to this file add icon/button
    8. a info icon button which show the rules from below
4. 2 different styles (shown afte expand) with a new theme setting (no thememanger neede for this!)
  4. hover above everything (look like a terminal)
  5. display the output under the input so both the file and the soc is shown at the same time with the soc on top
6. Rules for the file (write all your questions and all your answers as a "stream of consciousness.")
  7. save each input in the markdown file with dd.mm.yyyy - hh:mm: <input>
  8. for each input save it as a new line
  9. after it was inputed - deleting anything is not allowed its read only
  10. after it was inputed - changing/correcting anything is not allowed its ready only
  11. no copy/pasting allowed (except urls)




# PARA

dont give me any code yet just your ideas on how to best implement PARA (p = short term efforts in your work or life that you are working on now, a = long term responisibilites you want to manage over time, r = topic or interessets that may be useful in the future, a = inactive items from the other 3 categories) into my app

this is what i wrote down:

- PARA Metadata - the current "implementation" does not work as is and was just a placeholder now i want to rework it 
  - if one para is selected the others cant be selected (only one can be selected)
  - for each para metadata create one folder in the data/docs folder: PARA_PROJECTS, PARA_ARCHIVE, PARA_RESOURCE, PARA_AREA 
  - if no para metadata is set dont use the PARA folders
  - no need for backwards compatibility or a database migration since the app is still in development and not released yet
  - Implementation Ideas
    - we need to account for:
        - para metadata added: move the file to its corresponding para folder
        - para metadata removed/set to none: move the file to its corresponding folder without para
        - which is basically the same as we do in the handleAPIRenameFile function - which i think makes sense to reuse
        - correctly set para in the testdata.go file to test everything out (dont add para to all files tough)
    - change metadata to: PARA_CATEGORY: <PROJECTS|ARCHIVE|RESOURCES|AREAS> with a new enum
        - remove the existing routes for all Paras (/metadata/para/projects, /metadata//para/areas, /metadata//para/resources, /metadata//para/archive)
        - create a new route (/metadata/para)
            - in this route check if para was added/removed and first call a check - if added we need to add PARA_xx to the new filepath, if removed we need to remove PARA_xx from the new filepath and then the handleAPIRenameFile with the new filepaths
    - in pathutils add a GetParaPath() in which a para PATH e.g. (projects/area/archive/resources) is passed and the correct path is returned => reuse existing functions in pathutils and should handle both directions
    - in render_metadata: display the para as radio buttons with a additional NONE (default) value if PARA_ENABLED
        - as a style: i want it as its own row in the metadata section but still in the form-group as a row (so it takes as little space as possible)
    - /browse/<para_xxx> wont work anymore => create independet routes e.g. /browse/projects, /browse/areas...
    - WidgetTypes: WidgetTypeParaProjects, WidgetTypeParaAreas, WidgetTypeParaResources, WidgetTypeParaArchive will break => update filter with folder matching
    - update sqlite schema and drop the 4 para columns and add a para_category (just like the metadata..)
    - update mapping.go for new para fields

**done**

    - add a new env: PARA_ENABLED
    - contentStorage: add a CreateParaDirectories() to the interface which adds PARA_PROJECTS, PARA_AREAS, PARA_RESOURCES and PARA_ARCHIVE if para_enabled is true
    - update collection logic to skip para folders when determining collection
      - e.g. PARA_PROJECTS/work/meeting-notes.md = collection work
      - e.g. PARA_PROJECTS/standalone.md = collection default

# small stuff

- make certain settings required in thememanager
- rework docs folder manually without ai
  - use the docs folder as testdata and remove the internal/testdata/testfiles
- new references filetype: Link Resources to certain files e.g. i have postgres file and i want to link a Page about Optimization
    - new editor with 3 inputs
      - reference (input for link)
      - referencedescription (textbox) (why did you add this reference)
      - referencedTo (datalist with GetAllFilePathsFromSystemData) - if one file is added add another input (so it can be referenced to multiple files)
    - new metadata: references
    - save references in database or as file?
- Dashboard
  - make the positions work with a custom layout work
  - Add widget drag & drop reordering
  - Implement widget resizing
  - Add dashboard export/import
- metadata
  - save in yaml header in markdown files
  - change linkRegex config to names, e.g. obsidian, notion, dokuwiki... instead of a regex? or add one regex string + confignames
- performance updates
  - use Query() instead of a loop through files.GetAllFiles()
  - use Query in filter.go
  - Refactor filter.go to use query
- move toc to contentHandler?
