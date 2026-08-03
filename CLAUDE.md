# Project

- im currently working on the following golang, htmx app.

## Safety

- DO NOT EVER MAKE ANY CHANGES IN "/media/markus/SamsungT5/knov/data"

## Working style

- i want you to anwser with as little code as possible to only fix the problem i anwsered without any unecessary code, as simple and small as possible with as few changes as possible
- check current status of the project
- give me the full file not just the changes
- always search the project since you already have all the files
- no need for backwards compability since the app is not released yet - you can remove functions/routes
- i create the uploaded files using the Makefiles - so the filenames are not completly the same in the repo, there is a FILE_LIST.txt with the tree command for you to have a overview over all files in the repo

## Architecture

- business logic belongs in the package it's about (files, git, filter, etc.) - server (handlers) and job should stay thin wrappers: validate/resolve input, call one function in the owning package, translate the result into a response or JobRun
- i dont want any html generation in the handler - use the render subpackage for any html strings
- if anything related to paths prop up - use the pathutils package!
- if working with paths - we have to take care of both linux and windows os paths

## API

- make the api RESTful
- if you create an api call keep in mind to keep it theme friendly (lean more towards being generic) and also add comments for swagger to work, also stay with accept form data we dont need to accept json
- for every return in the api folder use: writeResponse
- dont forget to use translation.SprintfForRequest in the server package for handler and the render package for EVERY String

## Logging

- for logging message i only want use lowercase
- use the logging package with a appropriate key

## Frontend / theming

- think theme-agnostic
- for styles/css files use Global styles only in style.css and all specific files use ID selectors (#page-, #component-, #view-)
- dont use hardcoded colors use the available vars instead
- goal: less hand-rolled JS, more htmx
- if you add a new env also add it to .env.example and the html templates of both themes

## Go style

- Use fmt.Fprintf(…) instead of WriteString(fmt.Sprintf(…)) (QF1012 default)
- if possible use slices.Contains instead of loops
