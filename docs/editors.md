# Editors

# current usage
## codemirror6

https://github.com/blueberrycongee/codemirror-live-markdown

```bash
mkdir ~/codemirror-bundle && cd ~/codemirror-bundle
npm init -y
npm install @codemirror/state @codemirror/view @codemirror/commands @codemirror/search @codemirror/language @codemirror/lang-markdown @codemirror/autocomplete @codemirror/lint @replit/codemirror-vim @uiw/codemirror-extensions-line-numbers-relative
npm install --save-dev esbuild

touch editor.js
touch build.sh
touch webapp.sh
chmod +x build.sh webapp.sh
```

editor.js
```js
import { EditorState, EditorSelection, RangeSetBuilder } from "@codemirror/state";
import {
  EditorView,
  keymap,
  gutter,
  GutterMarker,
  drawSelection,
  highlightActiveLine,
  placeholder,
  lineNumbers,
  highlightSpecialChars,
  Decoration,
  ViewPlugin,
} from "@codemirror/view";
import { defaultKeymap, history, historyKeymap } from "@codemirror/commands";
import {
  search,
  searchKeymap,
  highlightSelectionMatches,
} from "@codemirror/search";
import { vim, Vim, getCM } from "@replit/codemirror-vim";
import {
  bracketMatching,
  foldGutter,
  syntaxHighlighting,
  HighlightStyle,
  foldKeymap,
  indentUnit,
  syntaxTree,
} from "@codemirror/language";
import { classHighlighter, tags } from "@lezer/highlight";
import { markdown } from "@codemirror/lang-markdown";
import { javascript } from "@codemirror/lang-javascript";
import { python } from "@codemirror/lang-python";
import { css } from "@codemirror/lang-css";
import { html } from "@codemirror/lang-html";
import { json } from "@codemirror/lang-json";
import { sql } from "@codemirror/lang-sql";
import { yaml } from "@codemirror/lang-yaml";
import { go } from "@codemirror/lang-go";
import { StreamLanguage } from "@codemirror/language";
import { shell } from "@codemirror/legacy-modes/mode/shell";
import {
  autocompletion,
  completionKeymap,
  closeBrackets,
  closeBracketsKeymap,
} from "@codemirror/autocomplete";

// ── Fenced code block languages ─────────────────────────────────────────────
//
// Gives ```js / ```python / … fenced blocks a real language parser, so their
// content gets live syntax highlighting (keywords, strings, …) instead of
// being treated as plain monospace text. classHighlighter already maps the
// resulting tags (tags.keyword, tags.string, tags.typeName, …) to tok-*
// classes, which index.html already styles.
const shellLanguage = StreamLanguage.define(shell);
const codeLanguageByAlias = {
  js: javascript(),
  jsx: javascript({ jsx: true }),
  javascript: javascript(),
  mjs: javascript(),
  cjs: javascript(),
  ts: javascript({ typescript: true }),
  tsx: javascript({ typescript: true, jsx: true }),
  typescript: javascript({ typescript: true }),
  py: python(),
  python: python(),
  css: css(),
  html: html(),
  htm: html(),
  json: json(),
  sql: sql(),
  yaml: yaml(),
  yml: yaml(),
  go: go(),
  golang: go(),
  sh: shellLanguage,
  bash: shellLanguage,
  shell: shellLanguage,
};

function codeLanguages(info) {
  const name = info.trim().split(/\s+/)[0].toLowerCase();
  const support = codeLanguageByAlias[name];
  // LanguageSupport instances (javascript(), python(), …) need unwrapping to
  // their .language; StreamLanguage instances (shellLanguage) already are one.
  return support ? support.language || support : null;
}

// ── Live-preview markdown highlighting ──────────────────────────────────────
//
// Goal: content renders styled (real heading sizes, real bold/italic, code
// pills, …) while the markdown syntax markers (#, **, _, `, >, -, [...]())
// stay visible at all times — no widget replacement / hide-on-blur tricks,
// just CSS driven by the syntax tree. `classHighlighter` supplies generic
// classes (tok-strong, tok-link, tok-comment, …); this style adds the
// markdown-specific ones classHighlighter doesn't cover (per-level headings,
// inline code, quotes, lists, hr, and the dimmed "marker" look).
const markdownLiveStyle = HighlightStyle.define([
  { tag: tags.heading1, class: "cm-h1" },
  { tag: tags.heading2, class: "cm-h2" },
  { tag: tags.heading3, class: "cm-h3" },
  { tag: tags.heading4, class: "cm-h4" },
  { tag: tags.heading5, class: "cm-h5" },
  { tag: tags.heading6, class: "cm-h6" },
  { tag: tags.strikethrough, class: "cm-strike" },
  { tag: tags.monospace, class: "cm-inline-code" },
  { tag: tags.quote, class: "cm-quote" },
  { tag: tags.list, class: "cm-list" },
  { tag: tags.contentSeparator, class: "cm-hr" },
  { tag: tags.processingInstruction, class: "cm-mark" },
]);

// ── WYSIWYG marker hiding ───────────────────────────────────────────────────
//
// Conceals markdown syntax markers (#, **, `, >, -, [...]()) everywhere except
// on the line the selection currently touches — same idea as Obsidian/Typora
// "live preview". Implemented as replace decorations (zero-width, so the
// marker text is gone from layout) rather than a CSS hide, and registered as
// EditorView.atomicRanges so arrow-key/mouse navigation steps over a hidden
// marker as a single unit instead of getting stuck inside it.
const WYSIWYG_HIDDEN_TYPES = new Set([
  "HeaderMark",
  "QuoteMark",
  "ListMark",
  "LinkMark",
  "EmphasisMark",
  "CodeMark",
  "CodeInfo",
  "StrikethroughMark",
  "URL",
  "LinkTitle",
]);
// These mark types are followed by a literal space that isn't part of any
// node ("# heading", "> quote", "- item") — swallow it too, or hiding the
// mark alone leaves a stray leading space.
const WYSIWYG_TRIM_TRAILING_SPACE = new Set(["HeaderMark", "QuoteMark", "ListMark"]);

const hiddenMarkDecoration = Decoration.replace({});

function wysiwygMarkRange(doc, node) {
  let to = node.to;
  if (WYSIWYG_TRIM_TRAILING_SPACE.has(node.name) && doc.sliceString(to, to + 1) === " ") {
    to += 1;
  }
  return { from: node.from, to };
}

function wysiwygLineTouchesSelection(state, from, to) {
  const startLine = state.doc.lineAt(from).number;
  const endLine = state.doc.lineAt(Math.max(from, to - 1)).number;
  return state.selection.ranges.some((range) => {
    const anchorLine = state.doc.lineAt(range.anchor).number;
    const headLine = state.doc.lineAt(range.head).number;
    return (
      (anchorLine >= startLine && anchorLine <= endLine) ||
      (headLine >= startLine && headLine <= endLine)
    );
  });
}

function computeWysiwygDecorations(view) {
  const builder = new RangeSetBuilder();
  const { state } = view;
  for (const { from, to } of view.visibleRanges) {
    syntaxTree(state).iterate({
      from,
      to,
      enter: (node) => {
        if (!WYSIWYG_HIDDEN_TYPES.has(node.name)) return;
        const range = wysiwygMarkRange(state.doc, node);
        if (wysiwygLineTouchesSelection(state, range.from, range.to)) return;
        builder.add(range.from, range.to, hiddenMarkDecoration);
      },
    });
  }
  return builder.finish();
}

const wysiwygMarkerHiding = ViewPlugin.fromClass(
  class {
    constructor(view) {
      this.decorations = computeWysiwygDecorations(view);
    }
    update(update) {
      if (update.docChanged || update.selectionSet || update.viewportChanged) {
        this.decorations = computeWysiwygDecorations(update.view);
      }
    }
  },
  {
    decorations: (plugin) => plugin.decorations,
    provide: (pluginClass) =>
      EditorView.atomicRanges.of(
        (view) => view.plugin(pluginClass)?.decorations || Decoration.none
      ),
  }
);

// Relative line numbers gutter: "0" on the cursor line, absolute distance on others.
function makeRelativeLineNumbers() {
  return gutter({
    class: "cm-lineNumbers",
    lineMarkerChange: (update) => update.selectionSet || update.docChanged,
    lineMarker: (view, line) => {
      const curLine = view.state.doc.lineAt(
        view.state.selection.main.head
      ).number;
      const thisLine = view.state.doc.lineAt(line.from).number;
      const label =
        thisLine === curLine ? "0" : String(Math.abs(thisLine - curLine));
      return Object.assign(Object.create(GutterMarker.prototype), {
        toDOM() {
          const d = document.createElement("div");
          d.textContent = label;
          return d;
        },
        eq(other) {
          return other.toDOM && other.toDOM().textContent === label;
        },
      });
    },
  });
}

// ── Vim clipboard integration ─────────────────────────────────────────────────
//
// WRITE (y/d/c → system clipboard):
//   Patch RegisterController.prototype.pushText so every unnamed-register write
//   also calls navigator.clipboard.writeText().  Prototype patch survives
//   resetVimGlobalState_() calls that replace the controller instance.
//
// READ (system clipboard → p):
//   codemirror-vim already supports the "+" register: its paste action calls
//   navigator.clipboard.readText() when registerName === '+'.  We use
//   Vim.noremap to redirect p/P → "+p/"+P so pressing p always reads the
//   system clipboard via vim's own built-in path — no custom keydown intercept.
//
//   The first time p is pressed Firefox will show a one-time "Paste" permission
//   prompt.  After the user allows it once the permission is permanent.
//   To make the prompt appear on editor FOCUS (more natural than on p), we call
//   readText() in a focus handler so the permission is already granted by the
//   time the user reaches for p.

let _protoHooked = false;

function _hookVimPrototype() {
  if (_protoHooked || typeof navigator === "undefined" || !navigator.clipboard)
    return;
  try {
    const rc = Vim.getRegisterController();
    if (!rc) return;
    const proto = Object.getPrototypeOf(rc);
    if (proto._clipboardPatched) return;
    proto._clipboardPatched = true;
    _protoHooked = true;

    const origPushText = proto.pushText;
    proto.pushText = function (registerName, op, text, linewise, blockwise) {
      origPushText.call(this, registerName, op, text, linewise, blockwise);
      if (!registerName || registerName === '"') {
        const stored = this.unnamedRegister && this.unnamedRegister.toString();
        if (stored) navigator.clipboard.writeText(stored).catch(() => {});
      }
    };
  } catch (_) {}
}

// On editor focus: call readText() so the browser permission prompt (if any)
// appears when the user clicks into the editor, not when they press p.
// After the one-time grant this is a silent background sync.
function vimFocusClipboardSync() {
  let denied = false;
  return EditorView.domEventHandlers({
    focus() {
      if (!navigator.clipboard || denied) return;
      navigator.clipboard.readText().then(
        (text) => {
          if (!text) return;
          try {
            const rc = Vim.getRegisterController();
            if (rc) rc.unnamedRegister.setText(text, text.endsWith("\n"));
          } catch (_) {}
        },
        (e) => { if (e && e.name === "NotAllowedError") denied = true; }
      );
    },
  });
}

// ─────────────────────────────────────────────────────────────────────────────

window.createCodeMirror = function (element, content, options) {
  options = options || {};

  const relLineNums = makeRelativeLineNumbers();

  const extensions = [
    ...(options.vimMode ? [vim()] : []),

    history(),
    drawSelection(),
    EditorView.lineWrapping,

    markdown({ codeLanguages }),
    syntaxHighlighting(classHighlighter),
    syntaxHighlighting(markdownLiveStyle),
    ...(options.wysiwyg ? [wysiwygMarkerHiding] : []),

    ...(options.bracketMatching ? [bracketMatching()] : []),
    ...(options.autoBrackets ? [closeBrackets()] : []),
    autocompletion(),
    indentUnit.of("  "),

    highlightActiveLine(),
    ...(options.lineNumbers && !options.relativeLineNumbers
      ? [lineNumbers()]
      : []),
    ...(options.relativeLineNumbers ? [relLineNums] : []),
    ...(options.highlightSelection
      ? [
          highlightSelectionMatches(
            options.highlightSelectionWholeWord ? { wholeWords: true } : {}
          ),
        ]
      : []),
    highlightSpecialChars(),
    ...(options.foldGutter ? [foldGutter()] : []),

    search(),

    keymap.of([
      ...defaultKeymap,
      ...historyKeymap,
      ...searchKeymap,
      ...completionKeymap,
      ...(options.autoBrackets ? closeBracketsKeymap : []),
      ...(options.foldGutter ? foldKeymap : []),
    ]),

    EditorView.inputHandler.of(typeToWrap),

    EditorView.updateListener.of(function (update) {
      if (options.onChange) options.onChange(update);
    }),

    ...(options.vimMode ? [vimFocusClipboardSync()] : []),
  ];

  if (options.placeholder) {
    extensions.push(placeholder(options.placeholder));
  }

  const state = EditorState.create({ doc: content || "", extensions });
  const view = new EditorView({ state, parent: element });

  if (options.vimMode) {
    _hookVimPrototype();
    // Redirect p/P to the "+" register so vim's built-in readText() path is used.
    Vim.noremap("p", '"+p', "normal");
    Vim.noremap("P", '"+P', "normal");
  }

  return view;
};

// ── Type-to-wrap ─────────────────────────────────────────────────────────────
//
// Typing one of these characters over a non-empty selection wraps the
// selection instead of replacing it (default contenteditable behavior is to
// delete the selection and insert the typed character). The wrapped text
// stays selected, so pressing the same character again stacks another layer
// around it — e.g. selecting "word" and pressing * twice gives **word**.
//
// Deliberately not the toolbar's wrapSelection(): that toggles markers off
// when the selection is already surrounded by them, which would make a
// second keypress of the same character undo the first instead of stacking.
const TYPE_TO_WRAP_CHARS = new Set(["`", "*", "_", "~"]);

function insertWrap(view, char) {
  view.dispatch(
    view.state.changeByRange((range) => ({
      changes: [{ from: range.from, insert: char }, { from: range.to, insert: char }],
      range: EditorSelection.range(range.from + char.length, range.to + char.length),
    }))
  );
}

function typeToWrap(view, from, to, text) {
  if (text.length !== 1 || !TYPE_TO_WRAP_CHARS.has(text)) return false;
  if (view.state.selection.ranges.every((range) => range.empty)) return false;
  insertWrap(view, text);
  return true;
}

// ── Toolbar commands ──────────────────────────────────────────────────────
//
// Small, dependency-free markdown edit commands used by the toolbar buttons
// in index.html. Each takes the live EditorView and mutates it directly.

// Wrap/unwrap every selection range with `before`...`after` (e.g. "**" for
// bold). Toggles off when the selection (or its immediate surroundings) is
// already wrapped; on an empty selection it drops the cursor between the
// inserted markers so typing continues immediately.
function wrapSelection(view, before, after) {
  after = after == null ? before : after;
  const doc = view.state.doc;
  view.dispatch(
    view.state.changeByRange((range) => {
      const { from, to } = range;
      const selText = doc.sliceString(from, to);

      if (
        selText.length >= before.length + after.length &&
        selText.startsWith(before) &&
        selText.endsWith(after)
      ) {
        const inner = selText.slice(before.length, selText.length - after.length);
        return {
          changes: [{ from, to, insert: inner }],
          range: EditorSelection.range(from, from + inner.length),
        };
      }

      const outerBefore = doc.sliceString(Math.max(0, from - before.length), from);
      const outerAfter = doc.sliceString(to, Math.min(doc.length, to + after.length));
      if (outerBefore === before && outerAfter === after) {
        return {
          changes: [
            { from: from - before.length, to: from, insert: "" },
            { from: to, to: to + after.length, insert: "" },
          ],
          range: EditorSelection.range(from - before.length, to - before.length),
        };
      }

      return {
        changes: [{ from, insert: before }, { from: to, insert: after }],
        range: range.empty
          ? EditorSelection.cursor(from + before.length)
          : EditorSelection.range(from + before.length, to + before.length),
      };
    })
  );
  view.focus();
}

// Toggle a per-line prefix (list bullet, quote, …) across every line the
// current selection touches. Removes the prefix if every touched line
// already has it, otherwise adds it to whichever lines are missing it.
function toggleLinePrefix(view, regex, prefix) {
  const state = view.state;
  const range = state.selection.main;
  const startLine = state.doc.lineAt(range.from).number;
  const endLine = state.doc.lineAt(range.to).number;

  let allHave = true;
  for (let n = startLine; n <= endLine; n++) {
    if (!regex.test(state.doc.line(n).text)) {
      allHave = false;
      break;
    }
  }

  const changes = [];
  for (let n = startLine; n <= endLine; n++) {
    const line = state.doc.line(n);
    if (allHave) {
      const m = regex.exec(line.text);
      changes.push({ from: line.from, to: line.from + m[0].length, insert: "" });
    } else if (!regex.test(line.text)) {
      changes.push({ from: line.from, insert: prefix });
    }
  }
  if (!changes.length) return;

  const changeSet = state.changes(changes);
  view.dispatch({
    changes,
    selection: EditorSelection.range(
      changeSet.mapPos(range.anchor),
      changeSet.mapPos(range.head)
    ),
    scrollIntoView: true,
  });
  view.focus();
}

// Toggle an ATX heading marker ("#".repeat(level) + " ") on the current line.
// Re-clicking the same level removes it; clicking a different level swaps it.
function toggleHeading(view, level) {
  const marker = "#".repeat(level) + " ";
  view.dispatch(
    view.state.changeByRange((range) => {
      const line = view.state.doc.lineAt(range.from);
      const match = /^(#{1,6})\s+/.exec(line.text);
      let changes, delta;
      if (match && match[1].length === level) {
        changes = { from: line.from, to: line.from + match[0].length, insert: "" };
        delta = -match[0].length;
      } else if (match) {
        changes = { from: line.from, to: line.from + match[0].length, insert: marker };
        delta = marker.length - match[0].length;
      } else {
        changes = { from: line.from, insert: marker };
        delta = marker.length;
      }
      const pos = Math.max(line.from, range.head + delta);
      return { changes, range: EditorSelection.cursor(pos) };
    })
  );
  view.focus();
}

function insertCodeBlock(view) {
  const range = view.state.selection.main;
  const selText = view.state.sliceDoc(range.from, range.to);
  const before = "```\n";
  const after = "\n```";
  view.dispatch({
    changes: { from: range.from, to: range.to, insert: before + selText + after },
    selection: EditorSelection.range(
      range.from + before.length,
      range.from + before.length + selText.length
    ),
  });
  view.focus();
}

function insertLink(view) {
  const range = view.state.selection.main;
  const selText = view.state.sliceDoc(range.from, range.to);
  const text = selText || "link text";
  const url = "https://";
  const insert = `[${text}](/files/$%7Burl%7D.md)`;
  const urlStart = range.from + 1 + text.length + 2;
  view.dispatch({
    changes: { from: range.from, to: range.to, insert },
    selection: EditorSelection.range(urlStart, urlStart + url.length),
  });
  view.focus();
}

function insertHr(view) {
  const state = view.state;
  const pos = state.selection.main.to;
  const line = state.doc.lineAt(pos);
  const insert = (line.text.length ? "\n\n" : "\n") + "---\n\n";
  view.dispatch({
    changes: { from: pos, insert },
    selection: EditorSelection.cursor(pos + insert.length),
  });
  view.focus();
}

window.mdCommands = {
  bold: (view) => wrapSelection(view, "**"),
  italic: (view) => wrapSelection(view, "_"),
  strikethrough: (view) => wrapSelection(view, "~~"),
  inlineCode: (view) => wrapSelection(view, "`"),
  heading: (view, level) => toggleHeading(view, level),
  quote: (view) => toggleLinePrefix(view, /^>\s?/, "> "),
  bulletList: (view) => toggleLinePrefix(view, /^[-*]\s/, "- "),
  orderedList: (view) => toggleLinePrefix(view, /^\d+\.\s/, "1. "),
  codeBlock: (view) => insertCodeBlock(view),
  link: (view) => insertLink(view),
  hr: (view) => insertHr(view),
};
````

build.sh
```bash
#!/bin/bash

OUTPUTFILE=output/codemirror6-bundle.min.js
npx esbuild editor.js --bundle --minify --outfile=$OUTPUTFILE
ls -lh $OUTPUTFILE
```


# Tried

## overtype

https://github.com/panphora/overtype

```bash
wget https://cdn.jsdelivr.net/npm/overtype@latest
```

=> was working completly fine but delivered exactly the same as my custom codemirror6
=> is way more lightweight though and could maybe handle larger files as well

## toastui

```bash
wget https://cdn.jsdelivr.net/npm/@toast-ui/editor@3.2.2/dist/toastui-editor.min.js
wget https://cdn.jsdelivr.net/npm/@toast-ui/editor@3.2.2/dist/toastui-editor.min.css
```

https://ui.toast.com/tui-editor

=> annoying to customize
=> had small ui problems (e.g. with copy and paste, selection text..)


## quikdown

https://github.com/deftio/quikdown

```bash
wget -O static/quikdown-1.2.21.min.js https://cdn.jsdelivr.net/npm/quikdown
```

=> was ok
=> didnt test it for long

# options

## editor.md

https://github.com/pandao/editor.md

## traven

https://github.com/slpstream/traven

## codemirror5

https://github.com/codemirror/codemirror5


## codejar

https://medv.io/codejar/

# own editor

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
- auto continue lists
- keyboard shortcuts
- auto pairing markers
- paste handling
- image paste/drop
- code block highlighting
- wiki links
- folding headers/sections
- frontmatter

i already have a markdown parser builtin my app which is also reachable via an api
i also have a existing codehighlighter
at the moment im using the toastui editor (https://ui.toast.com/tui-editor)

did i miss something obvious for editors?
is it possible to implement something like this?
would it be hard to make?
dont make any changes yet just talk about it if its feasible
