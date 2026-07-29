# Editors

# traven

https://github.com/slpstream/traven

# codemirror

https://github.com/codemirror/codemirror5

# toastui

https://ui.toast.com/tui-editor

# quikdown

https://github.com/deftio/quikdown

```bash
wget -O static/quikdown-1.2.21.min.js https://cdn.jsdelivr.net/npm/quikdown
```

# knov

please anwser in english i want to create my own texteditor for my already existing pkms (which is written in golang with htmx) and i quickly wrote down my wishes:

- i want to create a markdown editor for this application
- i dont want to use node/npm
- if possible it shouldnt have any dependencies
- in the end i want either one of those:
  - directly use the editor via htmx?
  - get a knoveditor.min.js file which i can include
- redo/undo history
- i dont want a preview
- i want a live rendering - e.g. if i use **text** i want the text to be displayed bold while keeping the stars
- i want a toolbar with easily configurable buttons (so i easily can add and remove new buttons)
  - headings
  - bold
  - italic
  - strike
  - line
  - code
  - blockquote
  - lists
  - insert table
  - ...
- the buttons will need to support a selection - e.g. i selected a certain text => i need to detect if its already influenced (e.g. with two stars for bold)
- table editor
  - which allows to jump with tab
  - which automatically adjust the height if i use tab
  - just like this sublime text plugin: https://github.com/SublimeText/TableEditor
- easily customizable

i already have a markdown parser builtin my app which is also reachable via an api
i also have a existing codehighlighter
at the moment im using the toastui editor (https://ui.toast.com/tui-editor)

did i miss something obvious for editors?
is it possible to implement something like this?
would it be hard to make?
dont make any changes yet just talk about it if its feasible
