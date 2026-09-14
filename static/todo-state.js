// cycles a rendered todo checkbox's state (open -> done -> cancelled -> waiting -> open)
// entirely client-side for instant feedback, cascading the same state to any nested
// checkboxes already in the DOM (mirrors the server's line-indentation cascade), then
// persists the change in the background. reverts on failure.
// inline onclick attributes are stripped from rendered markdown content for XSS safety,
// so this is wired up via delegated click listener instead.
// only the actual file view's .file-content carries data-filepath (set from the
// already-known FilePath in the template data) — dashboard widgets and filter listings
// reuse the same .file-content markup without it, so clicks there are no-ops.
var TODO_STATES = [
    { liClass: 'todo-open', spanClass: 'todo-state-open', icon: 'fa-solid fa-circle' },
    { liClass: 'todo-done', spanClass: 'todo-state-done', icon: 'fa-solid fa-circle-check' },
    { liClass: 'todo-cancelled', spanClass: 'todo-state-cancelled', icon: 'fa-solid fa-circle-xmark' },
    { liClass: 'todo-waiting', spanClass: 'todo-state-waiting', icon: 'fa-solid fa-clock' }
];

function todoStateIndex(el) {
    for (var i = 0; i < TODO_STATES.length; i++) {
        if (el.classList.contains(TODO_STATES[i].spanClass)) return i;
    }
    return 0;
}

function applyTodoState(el, state) {
    var li = el.closest('li');
    TODO_STATES.forEach(function (s) {
        el.classList.remove(s.spanClass);
        if (li) li.classList.remove(s.liClass);
    });
    el.classList.add(state.spanClass);
    if (li) li.classList.add(state.liClass);
    var icon = el.querySelector('i');
    if (icon) icon.className = state.icon;
}

// sets li's date-stamp text, creating the span (and its clear button) on first stamp.
// the date always comes from the server response (see the todo-toggle request below),
// never computed client-side, so it's never off from what CycleTodoStateAtLine actually
// persisted (server timezone, not the visitor's browser timezone).
function setTodoDate(li, text) {
    if (!li) return;
    var dateSpan = li.querySelector(':scope > .todo-date');
    if (!dateSpan) {
        dateSpan = document.createElement('span');
        dateSpan.className = 'todo-date';
        var clearBtn = document.createElement('button');
        clearBtn.type = 'button';
        clearBtn.className = 'todo-date-clear';
        clearBtn.textContent = '×';
        var nestedList = li.querySelector(':scope > ul');
        var before = nestedList || null;
        li.insertBefore(dateSpan, before);
        li.insertBefore(clearBtn, before);
    }
    dateSpan.textContent = text;
}

document.addEventListener('click', function (e) {
    var el = e.target.closest('.todo-state[data-line]');
    if (!el) return;

    var container = el.closest('.file-content');
    var filepath = container && container.dataset.filepath;
    if (!filepath) return;

    var li = el.closest('li');
    var affected = li ? li.querySelectorAll('.todo-state[data-line]') : [el];
    var prevStates = [];
    affected.forEach(function (item) {
        prevStates.push(TODO_STATES[todoStateIndex(item)]);
    });

    var nextState = TODO_STATES[(todoStateIndex(el) + 1) % TODO_STATES.length];
    affected.forEach(function (item) {
        applyTodoState(item, nextState);
    });

    function onAfterRequest(e) {
        if (e.detail.ctx.sourceElement !== el) return;
        document.body.removeEventListener('htmx:after:request', onAfterRequest);
        if (!e.detail.ctx.response || e.detail.ctx.response.status >= 400) {
            affected.forEach(function (item, i) {
                applyTodoState(item, prevStates[i]);
            });
            return;
        }
        // response body is the " (YYYY-MM-DD)" stamp text; empty means date stamping is
        // off, in which case the server left any existing date untouched too, so the
        // display should be left alone rather than erasing a still-valid stored date
        if (e.detail.ctx.text) setTodoDate(li, e.detail.ctx.text);
    }
    document.body.addEventListener('htmx:after:request', onAfterRequest);

    htmx.ajax('POST', '/api/files/todo-toggle', {
        source: el,
        target: container,
        swap: 'none',
        values: {
            filepath: filepath,
            line: el.getAttribute('data-line')
        }
    });
});

document.addEventListener('click', function (e) {
    var clearBtn = e.target.closest('.todo-date-clear');
    if (!clearBtn) return;

    var li = clearBtn.closest('li');
    var container = clearBtn.closest('.file-content');
    var filepath = container && container.dataset.filepath;
    var stateEl = li && li.querySelector(':scope > .todo-state[data-line]');
    if (!filepath || !stateEl) return;

    var dateSpan = li.querySelector(':scope > .todo-date');
    var prevText = dateSpan ? dateSpan.textContent : '';
    setTodoDate(li, '');

    function onAfterRequest(e) {
        if (e.detail.ctx.sourceElement !== clearBtn) return;
        document.body.removeEventListener('htmx:after:request', onAfterRequest);
        if (!e.detail.ctx.response || e.detail.ctx.response.status >= 400) {
            setTodoDate(li, prevText);
        }
    }
    document.body.addEventListener('htmx:after:request', onAfterRequest);

    htmx.ajax('POST', '/api/files/todo-cleardate', {
        source: clearBtn,
        target: container,
        swap: 'none',
        values: {
            filepath: filepath,
            line: stateEl.getAttribute('data-line')
        }
    });
});
