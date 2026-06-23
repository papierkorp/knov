// theme: rail
var _lcTimer = null;
function updateLatestChangesSearch(query) {
    clearTimeout(_lcTimer);
    _lcTimer = setTimeout(function () {
        var results = document.getElementById('latestchanges-results');
        var collection = results ? (results.dataset.collection || '') : '';
        var q = query.trim();
        var base = '/api/git/latestchanges?count=50' + (collection ? '&collection=' + encodeURIComponent(collection) : '');
        var url = q ? '/api/git/latestchanges?count=50&q=' + encodeURIComponent(q) + (collection ? '&collection=' + encodeURIComponent(collection) : '') : base;
        htmx.ajax('GET', url, {
            target: '#latestchanges-results',
            swap: 'innerHTML',
            headers: { Accept: 'text/html' },
        });
    }, 300);
}
