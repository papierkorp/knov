// Wiki link autocomplete for [[...]] syntax.
//
// Two entry points:
//   initWikiAutocomplete(toastuiEditor)  — for the ToastUI markdown editor
//   initWikiAutocompleteForInputs(containerEl) — for containers with plain text inputs
//                                                (event-delegated, works with dynamic items)

(function(global) {
    // ── shared dropdown state ────────────────────────────────────────────────

    var dropdown = null;
    var currentResults = [];
    var activeIdx = 0;
    var fetchTimer = null;
    var onInsert = null; // set by each init function

    function ensureDropdown() {
        if (dropdown) return;
        dropdown = document.createElement('ul');
        dropdown.style.cssText = [
            'position:fixed',
            'z-index:99999',
            'background:var(--bg,#fff)',
            'border:1px solid var(--border,#ccc)',
            'border-radius:6px',
            'list-style:none',
            'margin:0',
            'padding:4px 0',
            'max-height:240px',
            'overflow-y:auto',
            'min-width:320px',
            'max-width:520px',
            'box-shadow:0 4px 16px rgba(0,0,0,0.18)',
            'display:none',
            'font-size:13px'
        ].join(';');
        document.body.appendChild(dropdown);
    }

    function hide() {
        if (dropdown) dropdown.style.display = 'none';
        onInsert = null;
    }

    function isVisible() {
        return dropdown && dropdown.style.display !== 'none';
    }

    function highlight(idx) {
        if (!dropdown) return;
        var activeEl = null;
        Array.from(dropdown.children).forEach(function(li, i) {
            var on = i === idx;
            li.style.background = on ? 'var(--accent,#0070f3)' : '';
            li.style.color = on ? '#fff' : 'inherit';
            if (on) activeEl = li;
        });
        activeIdx = idx;
        if (activeEl) activeEl.scrollIntoView({ block: 'nearest' });
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
        dropdown.innerHTML = '';
        if (!items.length) { hide(); return; }

        items.forEach(function(item, i) {
            var li = document.createElement('li');
            li.style.cssText = 'padding:5px 14px;cursor:pointer;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;';
            var nameSpan = document.createElement('span');
            nameSpan.style.fontWeight = '600';
            nameSpan.textContent = item.filename;
            var pathSpan = document.createElement('span');
            pathSpan.style.cssText = 'margin-left:8px;opacity:0.55;font-size:11px;';
            pathSpan.textContent = item.path;
            li.appendChild(nameSpan);
            li.appendChild(pathSpan);
            li.addEventListener('mousedown', function(e) {
                e.preventDefault();
                doInsert(i);
            });
            dropdown.appendChild(li);
        });
        highlight(0);

        var rect = getCaretRect(anchorEl);
        dropdown.style.display = 'block';
        dropdown.style.top = (rect.bottom + 6) + 'px';
        dropdown.style.left = rect.left + 'px';
        requestAnimationFrame(function() {
            if (!dropdown) return;
            var dr = dropdown.getBoundingClientRect();
            if (dr.bottom > window.innerHeight - 8)
                dropdown.style.top = (rect.top - dr.height - 6) + 'px';
            if (dr.right > window.innerWidth - 8)
                dropdown.style.left = (window.innerWidth - dr.width - 8) + 'px';
        });
    }

    function doInsert(idx) {
        if (idx < 0 || idx >= currentResults.length) return;
        if (onInsert) onInsert(currentResults[idx].path);
        hide();
    }

    function debouncedFetch(q, anchorEl) {
        clearTimeout(fetchTimer);
        fetchTimer = setTimeout(function() {
            fetch('/api/files/autocomplete?q=' + encodeURIComponent(q))
                .then(function(r) { return r.json(); })
                .then(function(items) { show(items, anchorEl); })
                .catch(hide);
        }, 120);
    }

    function attachSharedKeydown(el) {
        el.addEventListener('keydown', function(e) {
            if (!isVisible()) return;
            if (e.key === 'ArrowDown') {
                e.preventDefault(); e.stopPropagation();
                highlight(Math.min(activeIdx + 1, currentResults.length - 1));
            } else if (e.key === 'ArrowUp') {
                e.preventDefault(); e.stopPropagation();
                highlight(Math.max(activeIdx - 1, 0));
            } else if (e.key === 'Enter' || e.key === 'Tab') {
                e.preventDefault(); e.stopPropagation();
                doInsert(activeIdx);
            } else if (e.key === 'Escape') {
                e.preventDefault();
                hide();
            }
        }, true);
    }

    document.addEventListener('mousedown', function(e) {
        if (dropdown && !dropdown.contains(e.target)) hide();
    });

    // ── ToastUI editor variant ───────────────────────────────────────────────

    global.initWikiAutocomplete = function(editor) {
        var editorEl = document.getElementById('toastui-editor');
        if (!editorEl) return;

        attachSharedKeydown(editorEl);

        editorEl.addEventListener('keyup', function(e) {
            if (['ArrowUp','ArrowDown','Enter','Tab','Escape'].includes(e.key)) return;
            var sel = editor.getSelection();
            if (!sel) { hide(); return; }
            var lineText = (editor.getMarkdown().split('\n')[(sel[0][0] || 1) - 1]) || '';
            var before = lineText.substring(0, sel[0][1]);
            var m = before.match(/\[\[([^\]]*)$/);
            if (m) {
                onInsert = function(path) {
                    var s = editor.getSelection();
                    var line = s[0][0], ch = s[0][1];
                    var lt = (editor.getMarkdown().split('\n')[line - 1]) || '';
                    var ws = lt.substring(0, ch).lastIndexOf('[[');
                    if (ws === -1) return;
                    editor.setSelection([line, ws], [line, ch]);
                    editor.insertText('[[' + path + ']]');
                };
                debouncedFetch(m[1], editorEl);
            } else {
                hide();
            }
        });
    };

    // ── CodeMirror 6 variant ────────────────────────────────────────────────────

    global.initWikiAutocompleteForCodeMirror = function(view) {
        view.dom.addEventListener('keydown', function(e) {
            if (!isVisible()) return;
            if (e.key === 'ArrowDown') {
                e.preventDefault(); e.stopPropagation();
                highlight(Math.min(activeIdx + 1, currentResults.length - 1));
            } else if (e.key === 'ArrowUp') {
                e.preventDefault(); e.stopPropagation();
                highlight(Math.max(activeIdx - 1, 0));
            } else if (e.key === 'Enter' || e.key === 'Tab') {
                e.preventDefault(); e.stopPropagation();
                doInsert(activeIdx);
            } else if (e.key === 'Escape') {
                e.preventDefault();
                hide();
            }
        }, true);

        view.dom.addEventListener('keyup', function(e) {
            if (['ArrowUp','ArrowDown','Enter','Tab','Escape'].includes(e.key)) return;
            var pos = view.state.selection.main.head;
            var lineInfo = view.state.doc.lineAt(pos);
            var before = lineInfo.text.substring(0, pos - lineInfo.from);
            var m = before.match(/\[\[([^\]]*)$/);
            if (m) {
                onInsert = function(path) {
                    var cur = view.state.selection.main.head;
                    var li = view.state.doc.lineAt(cur);
                    var b = li.text.substring(0, cur - li.from);
                    var ws = b.lastIndexOf('[[');
                    if (ws === -1) return;
                    var toPos = cur;
                    if (li.text.substring(cur - li.from, cur - li.from + 2) === ']]') toPos += 2;
                    view.dispatch({ changes: { from: li.from + ws, to: toPos, insert: '[[' + path + ']]' } });
                };
                var coords = view.coordsAtPos(pos);
                var anchor = {
                    getBoundingClientRect: function() {
                        return { left: coords.left, right: coords.right || coords.left + 1,
                                 top: coords.top, bottom: coords.bottom,
                                 height: coords.bottom - coords.top,
                                 width: (coords.right || coords.left + 1) - coords.left };
                    }
                };
                debouncedFetch(m[1], anchor);
            } else {
                hide();
            }
        });
    };

    // ── plain input/textarea variant (event-delegated) ───────────────────────

    global.initWikiAutocompleteForInputs = function(containerEl, selector) {
        if (!containerEl) return;
        selector = selector || '.item-input';

        attachSharedKeydown(containerEl);

        containerEl.addEventListener('keyup', function(e) {
            if (['ArrowUp','ArrowDown','Enter','Tab','Escape'].includes(e.key)) return;
            var input = e.target;
            if (!input.matches(selector)) { hide(); return; }
            var before = input.value.substring(0, input.selectionStart);
            var m = before.match(/\[\[([^\]]*)$/);
            if (m) {
                onInsert = function(path) {
                    var pos = input.selectionStart;
                    var val = input.value;
                    var ws = val.substring(0, pos).lastIndexOf('[[');
                    if (ws === -1) return;
                    var inserted = '[[' + path + ']]';
                    input.setRangeText(inserted, ws, pos, 'end');
                    input.dispatchEvent(new Event('input', { bubbles: true }));
                };
                debouncedFetch(m[1], input);
            } else {
                hide();
            }
        });
    };

})(window);
