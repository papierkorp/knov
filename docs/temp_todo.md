# daily

single source of truth is the metadata - we just display it differently

## sync function

1. change metadata (either status, targetDate/todoDate or parent)
2. hx-reload /kanban, /daily and /calendar

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

#

also help with this: if i set a parent -
