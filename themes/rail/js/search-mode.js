// theme: rail
function updatePageSearchMode() {
    var input = document.getElementById('search-page-input');
    if (!input) return;
    var titleOnly = document.getElementById('search-titleonly') && document.getElementById('search-titleonly').checked;
    var history = document.getElementById('search-history') && document.getElementById('search-history').checked;
    var format = input.dataset.format || 'cards';
    var url = '/api/search?format=' + encodeURIComponent(format);
    if (titleOnly) url += '&titleonly=true';
    if (history) url += '&history=true';
    input.setAttribute('hx-get', url);
    htmx.process(input);
}
