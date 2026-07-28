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
        if (e.detail.elt !== el) return;
        document.body.removeEventListener('htmx:afterRequest', onAfterRequest);
        if (!e.detail.successful) {
            affected.forEach(function (item, i) {
                applyTodoState(item, prevStates[i]);
            });
        }
    }
    document.body.addEventListener('htmx:afterRequest', onAfterRequest);

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
