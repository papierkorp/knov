// Wiki link autocomplete for [[...]] syntax.

(function (global) {
  // ── shared dropdown state ────────────────────────────────────────────────

  var dropdown = null;
  var currentResults = [];
  var activeIdx = 0;
  var fetchTimer = null;
  var onInsert = null; // set by each init function

  function ensureDropdown() {
    if (dropdown) return;
    dropdown = document.createElement("ul");
    dropdown.style.cssText = [
      "position:fixed",
      "z-index:99999",
      "background:var(--bg,#fff)",
      "border:1px solid var(--border,#ccc)",
      "border-radius:6px",
      "list-style:none",
      "margin:0",
      "padding:4px 0",
      "max-height:240px",
      "overflow-y:auto",
      "min-width:320px",
      "max-width:520px",
      "box-shadow:0 4px 16px rgba(0,0,0,0.18)",
      "display:none",
      "font-size:13px",
    ].join(";");
    document.body.appendChild(dropdown);
  }

  function hide() {
    if (dropdown) dropdown.style.display = "none";
    onInsert = null;
  }

  function isVisible() {
    return dropdown && dropdown.style.display !== "none";
  }

  function highlight(idx) {
    if (!dropdown) return;
    var activeEl = null;
    Array.from(dropdown.children).forEach(function (li, i) {
      var on = i === idx;
      li.style.background = on ? "var(--bg-secondary,#e5e7eb)" : "";
      li.style.color = on ? "var(--text,#1f2937)" : "inherit";
      if (on) activeEl = li;
    });
    activeIdx = idx;
    if (activeEl) activeEl.scrollIntoView({ block: "nearest" });
  }

  function getCaretRect(el) {
    var sel = window.getSelection();
    if (sel && sel.rangeCount) {
      var rect = sel.getRangeAt(0).getBoundingClientRect();
      if (rect.height > 0) return rect;
    }
    return (el || document.body).getBoundingClientRect();
  }

  function show(items, anchorEl) {
    ensureDropdown();
    currentResults = items;
    activeIdx = 0;
    dropdown.innerHTML = "";
    if (!items.length) {
      hide();
      return;
    }

    items.forEach(function (item, i) {
      var li = document.createElement("li");
      li.style.cssText =
        "padding:5px 14px;cursor:pointer;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;";
      var nameSpan = document.createElement("span");
      nameSpan.style.fontWeight = "600";
      nameSpan.textContent = item.filename;
      var pathSpan = document.createElement("span");
      pathSpan.style.cssText = "margin-left:8px;color:var(--text-secondary,#6b7280);font-size:11px;";
      pathSpan.textContent = item.path;
      li.appendChild(nameSpan);
      li.appendChild(pathSpan);
      li.addEventListener("mousedown", function (e) {
        e.preventDefault();
        doInsert(i);
      });
      dropdown.appendChild(li);
    });
    highlight(0);

    var rect = getCaretRect(anchorEl);
    dropdown.style.display = "block";
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
    if (idx < 0 || idx >= currentResults.length) return;
    if (onInsert) onInsert(currentResults[idx].path);
    hide();
  }

  function debouncedFetch(q, anchorEl) {
    clearTimeout(fetchTimer);
    fetchTimer = setTimeout(function () {
      fetch("/api/files/autocomplete?q=" + encodeURIComponent(q))
        .then(function (r) {
          return r.json();
        })
        .then(function (items) {
          show(items, anchorEl);
        })
        .catch(hide);
    }, 120);
  }

  function debouncedFetchHeaders(filepath, q, anchorEl) {
    clearTimeout(fetchTimer);
    fetchTimer = setTimeout(function () {
      fetch("/api/files/headers?filepath=" + encodeURIComponent(filepath))
        .then(function (r) {
          return r.json();
        })
        .then(function (headers) {
          var ql = q.toLowerCase();
          var items = headers
            .filter(function (h) {
              return (
                !q ||
                h.text.toLowerCase().includes(ql) ||
                h.id.toLowerCase().includes(ql)
              );
            })
            .map(function (h) {
              return {
                filename: "#".repeat(h.level) + " " + h.text,
                path: filepath + "#" + h.id,
              };
            });
          show(items, anchorEl);
        })
        .catch(hide);
    }, 120);
  }

  function cursorOffset(path, opts) {
    return opts.cursorEnd || path.indexOf("#") !== -1 ? 2 : 0;
  }

  function triggerAutocomplete(before, anchorEl, insertFn) {
    var m = before.match(/\[\[([^\]]*)$/);
    if (m) {
      onInsert = insertFn;
      dispatchFetch(m[1], anchorEl);
    } else {
      hide();
    }
  }

  function dispatchFetch(inner, anchorEl) {
    var hashIdx = inner.indexOf("#");
    if (hashIdx !== -1) {
      debouncedFetchHeaders(
        inner.substring(0, hashIdx),
        inner.substring(hashIdx + 1),
        anchorEl,
      );
    } else {
      debouncedFetch(inner, anchorEl);
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
          highlight(Math.min(activeIdx + 1, currentResults.length - 1));
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

  // ── ToastUI editor variant ───────────────────────────────────────────────

  global.initWikiAutocompleteToastUI = function (editor, opts) {
    opts = opts || {};
    var editorEl = document.getElementById("toastui-editor");
    if (!editorEl) return;

    attachSharedKeydown(editorEl);

    editorEl.addEventListener("keyup", function (e) {
      if (["ArrowUp", "ArrowDown", "Enter", "Tab", "Escape"].includes(e.key))
        return;
      var sel = editor.getSelection();
      if (!sel) {
        hide();
        return;
      }
      var lineText =
        editor.getMarkdown().split("\n")[(sel[0][0] || 1) - 1] || "";
      // ToastUI returns 1-based columns, so substring may include one char past
      // the cursor; strip a single trailing ] to keep the regex working when
      // the cursor is inside an existing [[file#]] link.
      var before = lineText.substring(0, sel[0][1]).replace(/\]$/, "");
      triggerAutocomplete(before, editorEl, function (path) {
        var s = editor.getSelection();
        var line = s[0][0],
          ch = s[0][1];
        var lt = editor.getMarkdown().split("\n")[line - 1] || "";
        var ws = lt.substring(0, ch).lastIndexOf("[[");
        if (ws === -1) return;
        // ch is 1-based: subtract 1 to check the two chars that actually follow the cursor
        var endSel = lt.substring(ch - 1, ch + 1) === "]]" ? ch + 2 : ch;
        // col 0 = paragraph boundary in ProseMirror → invalid TextSelection endpoint.
        // col 1 selects from the same position without triggering the error.
        editor.setSelection([line, Math.max(1, ws)], [line, endSel]);
        editor.insertText("[[" + path + "]]");
        var moveBack = 2 - cursorOffset(path, opts);
        if (moveBack > 0) {
          var after = editor.getSelection();
          if (after) {
            editor.setSelection(
              [after[0][0], after[0][1] - moveBack],
              [after[0][0], after[0][1] - moveBack],
            );
          }
        }
      });
    });
  };

  // ── CodeMirror 6 variant ────────────────────────────────────────────────────

  global.initWikiAutocompleteForCodeMirror = function (view, opts) {
    opts = opts || {};
    attachSharedKeydown(view.dom);

    view.dom.addEventListener("keyup", function (e) {
      if (["ArrowUp", "ArrowDown", "Enter", "Tab", "Escape"].includes(e.key))
        return;
      var pos = view.state.selection.main.head;
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
      triggerAutocomplete(before, anchor, function (path) {
        var cur = view.state.selection.main.head;
        var li = view.state.doc.lineAt(cur);
        var b = li.text.substring(0, cur - li.from);
        var ws = b.lastIndexOf("[[");
        if (ws === -1) return;
        var toPos = cur;
        if (li.text.substring(cur - li.from, cur - li.from + 2) === "]]")
          toPos += 2;
        var cursorPos =
          li.from + ws + 2 + path.length + cursorOffset(path, opts);
        view.dispatch({
          changes: {
            from: li.from + ws,
            to: toPos,
            insert: "[[" + path + "]]",
          },
          selection: { anchor: cursorPos },
        });
      });
    });
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
      triggerAutocomplete(before, anchor, function (path) {
        var pos = input.selectionStart;
        var val = input.value;
        var ws = val.substring(0, pos).lastIndexOf("[[");
        if (ws === -1) return;
        var endPos = val.substring(pos, pos + 2) === "]]" ? pos + 2 : pos;
        input.setRangeText("[[" + path + "]]", ws, endPos, "end");
        var cursorPos = ws + 2 + path.length + cursorOffset(path, opts);
        input.setSelectionRange(cursorPos, cursorPos);
        input.dispatchEvent(new Event("input", { bubbles: true }));
      });
    });
  };

  // ── path autocomplete ────────────────────────────────────────────────────────
  // Reuses the shared dropdown. Substring matching so partial folder/file names
  // anywhere in the path are found. Tab selects the highlighted item and stays
  // in the input instead of moving focus (native <datalist> can't do this).
  function initPathAutocomplete(inputEl, apiEndpoint) {
    if (!inputEl) return;

    var suggestions = [];

    fetch(apiEndpoint)
      .then(function (r) {
        return r.text();
      })
      .then(function (html) {
        var tmp = document.createElement("datalist");
        tmp.innerHTML = html;
        suggestions = Array.from(tmp.options)
          .map(function (o) {
            return o.value;
          })
          .filter(Boolean);
      })
      .catch(function () {});

    attachSharedKeydown(inputEl);

    function refresh() {
      var v = inputEl.value;
      if (!v) {
        hide();
        return;
      }
      var vl = v.toLowerCase();
      var items = suggestions
        .filter(function (s) {
          return s.toLowerCase().includes(vl);
        })
        .map(function (s) {
          var parts = s.replace(/\/$/, "").split("/");
          return { filename: parts[parts.length - 1], path: s };
        });
      onInsert = function (path) {
        inputEl.value = path;
      };
      show(items, inputEl);
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
})(window);
