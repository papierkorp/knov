// cycles a rendered todo checkbox's state (open -> done -> cancelled -> waiting -> open)
// by saving the change to the file and swapping in the re-rendered content.
// inline onclick attributes are stripped from rendered markdown content for XSS safety,
// so this is wired up via delegated click listener instead.
// only the actual file view's .file-content carries data-filepath (set from the
// already-known FilePath in the template data) — dashboard widgets and filter listings
// reuse the same .file-content markup without it, so clicks there are no-ops.
document.addEventListener('click', function (e) {
    var el = e.target.closest('.todo-state[data-line]');
    if (!el) return;

    var container = el.closest('.file-content');
    var filepath = container && container.dataset.filepath;
    if (!filepath) return;

    htmx.ajax('POST', '/api/files/todo-toggle', {
        target: container,
        swap: 'innerHTML',
        values: {
            filepath: filepath,
            line: el.getAttribute('data-line')
        }
    });
});
