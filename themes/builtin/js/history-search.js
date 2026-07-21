// theme: builtin
var _lcTimer = null;
function updateLatestChangesSearch(query) {
    clearTimeout(_lcTimer);
    _lcTimer = setTimeout(function () {
        var results = document.getElementById('latestchanges-results');
        var collection = results ? (results.dataset.collection || '') : '';
        var folder = results ? (results.dataset.folder || '') : '';
        var suffix = (collection ? '&collection=' + encodeURIComponent(collection) : '') + (folder ? '&folder=' + encodeURIComponent(folder) : '');
        var q = query.trim();
        var base = '/api/git/latestchanges?count=50' + suffix;
        var url = q ? '/api/git/latestchanges?count=50&q=' + encodeURIComponent(q) + suffix : base;
        htmx.ajax('GET', url, {
            target: '#latestchanges-results',
            swap: 'innerHTML',
            headers: { Accept: 'text/html' },
        });
    }, 300);
}
