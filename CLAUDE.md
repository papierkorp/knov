# Project

- im currently working on the following golang, htmx app.

## Safety

- DO NOT EVER MAKE ANY CHANGES IN "/media/markus/SamsungT5/knov/data"
- dont touch or use the prod environment which is using the Port 1325

## Working style

- i want you to anwser with as little code as possible to only fix the problem i anwsered without any unecessary code, as simple and small as possible with as few changes as possible
- check current status of the project
- always search the project since you already have all the files
- no need for backwards compability since the app is not released yet - you can remove functions/routes
- no need to update the changelogs since they are auto generated per git commits

## Architecture

- business logic belongs in the package it's about (files, git, filter, etc.) - server (handlers) and job should stay thin wrappers: validate/resolve input, call one function in the owning package, translate the result into a response or JobRun
- same for the job, the job package should just include a wrapper
- i dont want any html generation in the handler - use the render subpackage for any html strings
- if anything related to paths prop up - use the pathutils package!
- if working with paths - we have to take care of both linux and windows os paths
- we updated to htmx 4.0 so the syntax is different to htmx 2.0 be careful of this

## API

- make the api RESTful
- always return an honest HTTP status code: real 4xx/5xx on failure (use writeAPIError), never a soft 200 with an error body. htmx 4 swaps 4xx/5xx response bodies by default, so the inline error still shows. a valid request that legitimately has no data is a 200 empty-state, not an error
- if you create an api call keep in mind to keep it theme friendly (lean more towards being generic) and also add comments for swagger to work, also stay with accept form data we dont need to accept json
- for every return in the api folder use: writeResponse (success) or writeAPIError (failure)
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
- keep the usage of alpine js in mind if useful
- DONT USE HARDCODE COLORS, ALWAYS use the color vars which are provided per defaults.css file

## Go style

- Use fmt.Fprintf(…) instead of WriteString(fmt.Sprintf(…)) (QF1012 default)
- if possible use slices.Contains instead of loops
