# daily

single source of truth is the metadata - we just display it differently

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

based on dailysync setting:

- if enabled: sync the links in the file
- if disabled: no sync

**on load**

- the file is saved in data/daily/<date>.md
  - look in the data/daily folder for the current date if there is a file
    - if there is no file and dailysync is activated:
      - search for todos for today and add them all to the markdown file (metadata TargetDate) accordingly to the parser
    - if there is no file and dailysync is deactivated:
      - create a new emtpy file
    - if there is a file
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
    - if moved to here => set status archived
    - status archived
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

**event trigger/on reload**

if dailysync is activated:

- status change in kanban => move the links
- targetDate change in calendar => remove/add the link and remove/add Parentlink
  else
- do nothing

**on save**

- check if the links were moved (parse) if yes => update metadata status of these files and reload /kanban
- update the parent in each link
- hx-trigger="change delay:1s"

**new settings**

- dailysync boolean enabled/disabled

## kanban

**routes**

- /kanban
- /kanban/<date> / /canban/from:<date>to:<date> (maybe with a PT parameter, so targetdate +x days)
- /api/kanban/board (with a parameter scope: for scope of today/week/month based on setting)
- /api/kanban/move
- /api/kanban/save

**views**

**parser**

**on load**

**display**

- 3 columns:
  - today
  - note
  - done
- - / add button: quick add of todo with targetDate and status
- how to do it with cards/what to display... (do i get /api/kanban/card/<id> route?..)

**event trigger/on reload**

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

- daily
  - show content of daily folder for this date
  - show filenames/paths links
- weekly/monthly
  - show filenames/paths links

**on load**

**parser**

**display**

- +/add button quick add of todo with targetDate and status
- without a time for now
- view based on htmx call

**event trigger/on reload**

**on save**

- allows drag and dropping the filenames into another date which changes the targetDate
- if targetDate changed => reload /daily and /kanban

**new settings**

## additional

additional neccessary changes

- when creating a fleeting note - add a targetdate of + 1 week
- change the status (which isnt used at the moment) to the 3 kanban status
- add a new metadata like PT or something like this (so i can have a range for targetDate e.g. targetdate+pt = date range)
- in server.go in /files/new/todo add parameter for quick adds for daily/calendar/kanban which automatically adds a targetDate of today and a status
- syncViews(fileID, changeType) function which is called in handleAPIxxxSave for all 3 new saves

#

also help with this: if i set a parent -
