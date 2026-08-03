// Wiki link autocomplete for [[...]] syntax.
//
// The suggestion list itself is server-rendered HTML (render.RenderAutocompleteList,
// served by the autocomplete endpoints through writeResponse). This file only
// positions the dropdown at the caret, handles keyboard navigation and inserts
// the selected data-value into the editor — things hx-attributes can't express
// for a caret-anchored dropdown inside CodeMirror.

(function (global) {
  var FILES_PREFIX = "/files/";

  // ── shared dropdown state ────────────────────────────────────────────────

  var dropdown = null;
  var items = [];
  var activeIdx = 0;
  var fetchTimer = null;
  var onInsert = null; // set by each init function

  function ensureDropdown() {
    if (dropdown) return;
    dropdown = document.createElement("div");
    dropdown.id = "component-autocomplete";
    // "manual" popover: promotes the dropdown to the browser's top layer so it
    // still renders above native <div popover> modals (e.g. the move/rename
    // modals) — a plain z-index can never win against top-layer content.
    dropdown.setAttribute("popover", "manual");
    dropdown.addEventListener("mousedown", function (e) {
      var li = e.target.closest(".autocomplete-item");
      if (!li) return;
      e.preventDefault();
      doInsert(Array.prototype.indexOf.call(items, li));
    });
    document.body.appendChild(dropdown);
  }

  function hide() {
    if (dropdown) {
      dropdown.style.display = "none";
      if (dropdown.hidePopover && dropdown.matches(":popover-open")) dropdown.hidePopover();
    }
    onInsert = null;
  }

  function isVisible() {
    return dropdown && dropdown.style.display === "block";
  }

  function highlight(idx) {
    if (!dropdown) return;
    Array.prototype.forEach.call(items, function (li, i) {
      li.classList.toggle("active", i === idx);
    });
    activeIdx = idx;
    if (items[idx]) items[idx].scrollIntoView({ block: "nearest" });
  }

  function getCaretRect(el) {
    var sel = window.getSelection();
    if (sel && sel.rangeCount) {
      var rect = sel.getRangeAt(0).getBoundingClientRect();
      if (rect.height > 0) return rect;
    }
    return (el || document.body).getBoundingClientRect();
  }

  function show(html, anchorEl) {
    ensureDropdown();
    dropdown.innerHTML = html;
    items = dropdown.querySelectorAll(".autocomplete-item");
    activeIdx = 0;
    if (!items.length) {
      hide();
      return;
    }
    highlight(0);

    var rect = getCaretRect(anchorEl);
    dropdown.style.display = "block";
    if (dropdown.showPopover && !dropdown.matches(":popover-open")) dropdown.showPopover();
    dropdown.style.top = rect.bottom + 6 + "px";
    dropdown.style.left = rect.left + "px";
    requestAnimationFrame(function () {
      if (!dropdown) return;
      var dr = dropdown.getBoundingClientRect();
      if (dr.bottom > window.innerHeight - 8)
        dropdown.style.top = rect.top - dr.height - 6 + "px";
      if (dr.right > window.innerWidth - 8)
        dropdown.style.left = window.innerWidth - dr.width - 8 + "px";
    });
  }

  function doInsert(idx) {
    if (idx < 0 || idx >= items.length) return;
    if (onInsert) onInsert(items[idx].getAttribute("data-value"));
    hide();
  }

  // Debounced fetch of a server-rendered suggestion list partial into the dropdown.
  function fetchList(url, anchorEl) {
    clearTimeout(fetchTimer);
    fetchTimer = setTimeout(function () {
      fetch(url, { headers: { Accept: "text/html" } })
        .then(function (r) {
          return r.text();
        })
        .then(function (html) {
          show(html, anchorEl);
        })
        .catch(hide);
    }, 120);
  }

  function fetchHeaders(filepath, q, anchorEl, bare) {
    fetchList(
      "/api/files/headers?filepath=" +
        encodeURIComponent(filepath) +
        "&q=" +
        encodeURIComponent(q) +
        (bare ? "&bare=1" : ""),
      anchorEl,
    );
  }

  function cursorOffset(path, opts, closeLen) {
    return opts.cursorEnd || path.indexOf("#") !== -1 ? closeLen : 0;
  }

  // Encodes each path segment (keeping "/" and "#" as separators intact) so
  // filenames with ")", spaces, etc. don't break the surrounding "](...)" syntax.
  function encodePathSegments(path) {
    return path
      .split("/")
      .map(function (seg) {
        return seg.split("#").map(encodeURIComponent).join("#");
      })
      .join("/");
  }

  // Builds the "](...)" target: a same-page anchor (e.g. "#translation") stays
  // relative instead of being routed through FILES_PREFIX.
  function buildTarget(path) {
    return (path.indexOf("#") === 0 ? "" : FILES_PREFIX) + encodePathSegments(path);
  }

  // Builds the "](...)" target for a media file, matching the "media/<path>"
  // form the media selector modal already inserts (no FILES_PREFIX).
  function buildMediaTarget(path) {
    return "media/" + encodePathSegments(path);
  }

  // Triggers on an unclosed "[[" (wikilink), an unclosed "![" + "](" (media/image
  // link target), or an unclosed "](" (markdown link target), whichever is open
  // at the caret. insertFn receives the chosen path plus which one matched, so
  // callers can build the right text.
  function triggerAutocomplete(before, anchorEl, insertFn, currentFile) {
    var wiki = before.match(/\[\[([^\]]*)$/);
    var md = !wiki && before.match(/\]\(([^)]*)$/);
    if (md && md[1].indexOf("://") !== -1) md = false;
    var isMedia = md && /!\[[^\[\]]*$/.test(before.slice(0, md.index));
    var m = wiki || md;
    if (m) {
      onInsert = function (path) {
        insertFn(path, wiki ? "wiki" : isMedia ? "media" : "md");
      };
      dispatchFetch(m[1], anchorEl, currentFile, isMedia);
    } else {
      hide();
    }
  }

  function dispatchFetch(inner, anchorEl, currentFile, isMedia) {
    if (isMedia) {
      fetchList("/api/media/autocomplete?q=" + encodeURIComponent(inner), anchorEl);
      return;
    }
    // Markdown links store "/files/<path>" (a real href), but the file and
    // header lookup APIs expect a plain repo-relative path, same as
    // wikilinks store internally. Strip the prefix before querying.
    if (inner.indexOf(FILES_PREFIX) === 0) inner = inner.substring(FILES_PREFIX.length);
    try {
      inner = decodeURIComponent(inner);
    } catch (e) {}
    var hashIdx = inner.indexOf("#");
    if (hashIdx !== -1) {
      // no filepath before the "#" (e.g. "](#") — link to a header in the
      // file currently being edited instead of querying an empty filepath.
      // Ask the server for bare "#id" values in that case, so the inserted
      // link stays a same-page anchor instead of the full file path.
      var typedFilepath = inner.substring(0, hashIdx);
      var filepath = typedFilepath || currentFile;
      if (!filepath) {
        hide();
        return;
      }
      fetchHeaders(filepath, inner.substring(hashIdx + 1), anchorEl, typedFilepath === "");
    } else {
      fetchList("/api/files/autocomplete?q=" + encodeURIComponent(inner), anchorEl);
    }
  }

  function attachSharedKeydown(el) {
    el.addEventListener(
      "keydown",
      function (e) {
        if (!isVisible()) return;
        if (e.key === "ArrowDown") {
          e.preventDefault();
          e.stopPropagation();
          highlight(Math.min(activeIdx + 1, items.length - 1));
        } else if (e.key === "ArrowUp") {
          e.preventDefault();
          e.stopPropagation();
          highlight(Math.max(activeIdx - 1, 0));
        } else if (e.key === "Enter" || e.key === "Tab") {
          e.preventDefault();
          e.stopPropagation();
          doInsert(activeIdx);
        } else if (e.key === "Escape") {
          e.preventDefault();
          hide();
        }
      },
      true,
    );
  }

  document.addEventListener("mousedown", function (e) {
    if (dropdown && !dropdown.contains(e.target)) hide();
  });

  // ── CodeMirror 6 variant ────────────────────────────────────────────────────

  global.initWikiAutocompleteForCodeMirror = function (view, opts) {
    opts = opts || {};
    attachSharedKeydown(view.dom);

    // CodeMirror positions are plain 0-based document offsets, so (unlike the
    // ToastUI variant above) it's safe to start the replacement right after
    // the "(" the user already typed instead of replacing it too.

    function insertWikiLink(path) {
      var cur = view.state.selection.main.head;
      var li = view.state.doc.lineAt(cur);
      var b = li.text.substring(0, cur - li.from);
      var ws = b.lastIndexOf("[[");
      if (ws === -1) return;
      var toPos = cur;
      if (li.text.substring(cur - li.from, cur - li.from + 2) === "]]")
        toPos += 2;
      var cursorPos = li.from + ws + 2 + path.length + cursorOffset(path, opts, 2);
      view.dispatch({
        changes: { from: li.from + ws, to: toPos, insert: "[[" + path + "]]" },
        selection: { anchor: cursorPos },
      });
    }

    function insertMdLink(path, isMedia) {
      var cur = view.state.selection.main.head;
      var li = view.state.doc.lineAt(cur);
      var b = li.text.substring(0, cur - li.from);
      var ws = b.lastIndexOf("](");
      if (ws === -1) return;
      var from = li.from + ws + 2;
      var toPos = cur;
      if (li.text.substring(cur - li.from, cur - li.from + 1) === ")")
        toPos += 1;
      var target = isMedia ? buildMediaTarget(path) : buildTarget(path);
      var cursorPos = from + target.length + cursorOffset(path, opts, 1);
      view.dispatch({
        changes: { from: from, to: toPos, insert: target + ")" },
        selection: { anchor: cursorPos },
      });
    }

    function runAutocompleteAt(pos) {
      var lineInfo = view.state.doc.lineAt(pos);
      var before = lineInfo.text.substring(0, pos - lineInfo.from);
      var coords = view.coordsAtPos(pos);
      var anchor = {
        getBoundingClientRect: function () {
          return {
            left: coords.left,
            right: coords.right || coords.left + 1,
            top: coords.top,
            bottom: coords.bottom,
            height: coords.bottom - coords.top,
            width: (coords.right || coords.left + 1) - coords.left,
          };
        },
      };
      triggerAutocomplete(before, anchor, function (path, mode) {
        if (mode === "wiki") insertWikiLink(path);
        else insertMdLink(path, mode === "media");
      }, opts.currentFile);
    }

    view.dom.addEventListener("keyup", function (e) {
      if (["ArrowUp", "ArrowDown", "Enter", "Tab", "Escape"].includes(e.key))
        return;
      runAutocompleteAt(view.state.selection.main.head);
    });

    // toolbar-triggered inserts: type the same opening syntax a user would
    // type by hand, then open the same autocomplete dropdown at the caret.
    function insertTrigger(snippet) {
      var pos = view.state.selection.main.to;
      var endPos = pos + snippet.length;
      view.dispatch({
        changes: { from: pos, to: pos, insert: snippet },
        selection: { anchor: endPos },
      });
      view.focus();
      runAutocompleteAt(endPos);
    }

    view.cmInsertMedia = function () {
      insertTrigger("![](");
    };
    view.cmInsertWikiLink = function () {
      insertTrigger("[[");
    };
  };

  // ── textarea caret position via mirror div ───────────────────────────────
  // window.getSelection() does not expose the cursor inside a <textarea>, so
  // we measure it by cloning the textarea's style into a hidden mirror div.

  function getTextareaCaretRect(textarea) {
    var computed = window.getComputedStyle(textarea);
    var taRect = textarea.getBoundingClientRect();
    var mirror = document.createElement("div");

    mirror.style.cssText = [
      "position:fixed",
      "visibility:hidden",
      "pointer-events:none",
      "white-space:pre-wrap",
      "word-wrap:break-word",
      "overflow:hidden",
      "width:" + taRect.width + "px",
      "height:" + taRect.height + "px",
      "top:" + taRect.top + "px",
      "left:" + taRect.left + "px",
    ].join(";");

    [
      "box-sizing", "font-family", "font-size", "font-weight", "font-style",
      "line-height", "letter-spacing", "word-spacing", "text-indent",
      "padding-top", "padding-right", "padding-bottom", "padding-left",
      "border-top-width", "border-right-width", "border-bottom-width", "border-left-width",
    ].forEach(function (p) {
      mirror.style[p] = computed[p];
    });

    mirror.appendChild(
      document.createTextNode(textarea.value.substring(0, textarea.selectionStart)),
    );
    var caret = document.createElement("span");
    caret.textContent = "​";
    mirror.appendChild(caret);

    document.body.appendChild(mirror);
    mirror.scrollTop = textarea.scrollTop;
    var rect = caret.getBoundingClientRect();
    document.body.removeChild(mirror);
    return rect;
  }

  // ── plain input/textarea variant (event-delegated) ───────────────────────

  global.initWikiAutocompleteForInputs = function (
    containerEl,
    optsOrSelector,
    selector,
  ) {
    if (!containerEl) return;
    var opts = {};
    if (optsOrSelector && typeof optsOrSelector === "object") {
      opts = optsOrSelector;
    } else if (typeof optsOrSelector === "string") {
      selector = optsOrSelector;
    }
    selector = selector || ".item-input";

    attachSharedKeydown(containerEl);

    // Plain string indices, so like the CodeMirror variant it's safe to
    // start the markdown-link replacement right after the "(".

    function insertWikiLink(input, path) {
      var pos = input.selectionStart;
      var val = input.value;
      var ws = val.substring(0, pos).lastIndexOf("[[");
      if (ws === -1) return;
      var endPos = val.substring(pos, pos + 2) === "]]" ? pos + 2 : pos;
      input.setRangeText("[[" + path + "]]", ws, endPos, "end");
      var cursorPos = ws + 2 + path.length + cursorOffset(path, opts, 2);
      input.setSelectionRange(cursorPos, cursorPos);
      input.dispatchEvent(new Event("input", { bubbles: true }));
    }

    function insertMdLink(input, path, isMedia) {
      var pos = input.selectionStart;
      var val = input.value;
      var ws = val.substring(0, pos).lastIndexOf("](");
      if (ws === -1) return;
      var start = ws + 2;
      var endPos = val.substring(pos, pos + 1) === ")" ? pos + 1 : pos;
      var target = isMedia ? buildMediaTarget(path) : buildTarget(path);
      input.setRangeText(target + ")", start, endPos, "end");
      var cursorPos = start + target.length + cursorOffset(path, opts, 1);
      input.setSelectionRange(cursorPos, cursorPos);
      input.dispatchEvent(new Event("input", { bubbles: true }));
    }

    containerEl.addEventListener("keyup", function (e) {
      if (["ArrowUp", "ArrowDown", "Enter", "Tab", "Escape"].includes(e.key))
        return;
      var input = e.target;
      if (!input.matches(selector)) {
        hide();
        return;
      }
      var before = input.value.substring(0, input.selectionStart);
      var anchor = input.tagName === "TEXTAREA"
        ? { getBoundingClientRect: function () { return getTextareaCaretRect(input); } }
        : input;
      triggerAutocomplete(before, anchor, function (path, mode) {
        if (mode === "wiki") insertWikiLink(input, path);
        else insertMdLink(input, path, mode === "media");
      }, opts.currentFile);
    });
  };

  // ── path autocomplete ────────────────────────────────────────────────────────
  // Reuses the shared dropdown; the endpoint does the substring matching and
  // returns the rendered list. Tab selects the highlighted item and stays in
  // the input instead of moving focus (native <datalist> can't do this).
  function initPathAutocomplete(inputEl, apiEndpoint) {
    if (!inputEl) return;

    attachSharedKeydown(inputEl);

    function refresh() {
      var v = inputEl.value;
      if (!v) {
        hide();
        return;
      }
      onInsert = function (path) {
        inputEl.value = path;
      };

      // typing "#" after a file path suggests that file's headings, same
      // data source as the [[wikilink]] anchor autocomplete, without
      // requiring the user to type the [[...]] bracket syntax
      var hashIdx = v.indexOf("#");
      if (hashIdx !== -1) {
        fetchHeaders(v.substring(0, hashIdx), v.substring(hashIdx + 1), inputEl);
        return;
      }

      fetchList(
        apiEndpoint +
          (apiEndpoint.indexOf("?") === -1 ? "?q=" : "&q=") +
          encodeURIComponent(v),
        inputEl,
      );
    }

    inputEl.addEventListener("input", refresh);
    inputEl.addEventListener("keydown", function (e) {
      if (e.key === "ArrowDown" && !isVisible()) {
        e.preventDefault();
        refresh();
      }
    });
    inputEl.addEventListener("blur", function () {
      setTimeout(hide, 150);
    });
  }

  global.initPathAutocomplete = initPathAutocomplete;

  // exposed so other insertion paths (e.g. the toolbar-triggered wiki-file
  // selector modal) build the same encoded "](...)" target instead of
  // duplicating (and potentially drifting from) the encoding logic here
  global.buildTarget = buildTarget;
})(window);
