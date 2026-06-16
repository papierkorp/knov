// Lazy-loaded kanban event log panel — fetched via dynamic import() only
// when a user actually opens the events view, so the cost is never paid
// by users who never click the events button.
// Theme-owned: rail. Deliberately not shared with other themes — see builtin/kanban-events.js.

function renderRows(list) {
    return list.map(function(e) {
        var ts    = new Date(e.timestamp).toLocaleString();
        var fname = e.filePath.split('/').pop();
        var from  = e.fromStatus || '—';
        return '<tr><td>' + ts + '</td><td title="' + e.filePath + '">' + fname + '</td><td>' + from + '</td><td>' + e.toStatus + '</td></tr>';
    }).join('');
}

function uniqueStatuses(events, field) {
    var seen = {};
    var out = [];
    events.forEach(function(e) {
        var v = e[field];
        if (v && !seen[v]) {
            seen[v] = true;
            out.push(v);
        }
    });
    out.sort();
    return out;
}

function renderStatusOptions(statuses, allLabel, fieldLabel) {
    var html = '<option value="">' + fieldLabel + ': ' + allLabel + '</option>';
    statuses.forEach(function(s) {
        html += '<option value="' + s + '">' + s + '</option>';
    });
    return html;
}

function renderFileOptions(filePaths, allLabel, fieldLabel, selected) {
    var html = '<option value="">' + fieldLabel + ': ' + allLabel + '</option>';
    filePaths.forEach(function(p) {
        var label = p.split('/').pop();
        var sel = p === selected ? ' selected' : '';
        html += '<option value="' + p + '" title="' + p + '"' + sel + '>' + label + '</option>';
    });
    return html;
}

function startOfDayISO(dateStr) {
    return new Date(dateStr + 'T00:00:00').toISOString();
}

function endOfDayISO(dateStr) {
    return new Date(dateStr + 'T23:59:59.999').toISOString();
}

function buildEventsURL(collection, dateFrom, dateTo, fileFilter) {
    // a specific file's full history shouldn't be truncated by the default page-sized limit
    var limit = fileFilter ? 0 : 200;
    var url = '/api/kanban/' + collection + '/events?limit=' + limit;
    if (dateFrom) url += '&from=' + encodeURIComponent(startOfDayISO(dateFrom));
    if (dateTo) url += '&to=' + encodeURIComponent(endOfDayISO(dateTo));
    if (fileFilter) url += '&file=' + encodeURIComponent(fileFilter);
    return url;
}

var SORT_KEYS = {
    time: function(e) { return e.timestamp; },
    file: function(e) { return e.filePath.toLowerCase(); },
    from: function(e) { return (e.fromStatus || '').toLowerCase(); },
    to: function(e) { return e.toStatus.toLowerCase(); }
};

function sortEvents(list, sortState) {
    var key = SORT_KEYS[sortState.column];
    var dir = sortState.dir === 'asc' ? 1 : -1;
    return list.slice().sort(function(a, b) {
        var av = key(a), bv = key(b);
        if (av < bv) return -1 * dir;
        if (av > bv) return 1 * dir;
        return 0;
    });
}

function renderHeader(t, sortState) {
    var columns = [
        { key: 'time', label: t.time },
        { key: 'file', label: t.file },
        { key: 'from', label: t.from },
        { key: 'to', label: t.to }
    ];
    return '<tr>' + columns.map(function(c) {
        var arrow = '';
        if (sortState.column === c.key) {
            arrow = ' <span class="kanban-events-sort-arrow">' + (sortState.dir === 'asc' ? '▲' : '▼') + '</span>';
        }
        return '<th class="kanban-events-sortable" data-column="' + c.key + '">' + c.label + arrow + '</th>';
    }).join('') + '</tr>';
}

// opts: { wrap: HTMLElement, collection: string, t: { loading, noEvents, failedToLoad, filterPlaceholder, time, file, from, to, all, dateFrom, dateTo } }
export function show(opts) {
    load(opts, '', '', null, '');
}

function load(opts, dateFrom, dateTo, sortState, fileFilter) {
    var wrap = opts.wrap;
    var t = opts.t;
    sortState = sortState || { column: 'time', dir: 'desc' };

    wrap.innerHTML = '<p>' + t.loading + '</p>';

    Promise.all([
        fetch(buildEventsURL(opts.collection, dateFrom, dateTo, fileFilter), { headers: { Accept: 'application/json' } }).then(function(r) { return r.json(); }),
        fetch('/api/kanban/' + opts.collection + '/files', { headers: { Accept: 'application/json' } }).then(function(r) { return r.json(); })
    ])
        .then(function(results) {
            var events = results[0] || [];
            var filePaths = results[1] || [];

            var fromStatuses = uniqueStatuses(events, 'fromStatus');
            var toStatuses = uniqueStatuses(events, 'toStatus');

            wrap.innerHTML =
                '<div class="kanban-events-view">' +
                '<div class="kanban-events-controls">' +
                '<input type="search" id="kanban-events-search" class="kanban-events-search" placeholder="' + t.filterPlaceholder + '">' +
                '<select id="kanban-events-file-filter" class="kanban-events-status-filter" title="' + t.file + '">' + renderFileOptions(filePaths, t.all, t.file, fileFilter) + '</select>' +
                '<select id="kanban-events-from-filter" class="kanban-events-status-filter" title="' + t.from + '">' + renderStatusOptions(fromStatuses, t.all, t.from) + '</select>' +
                '<select id="kanban-events-to-filter" class="kanban-events-status-filter" title="' + t.to + '">' + renderStatusOptions(toStatuses, t.all, t.to) + '</select>' +
                '<input type="date" id="kanban-events-date-from" class="kanban-events-date" title="' + t.dateFrom + '" value="' + dateFrom + '">' +
                '<input type="date" id="kanban-events-date-to" class="kanban-events-date" title="' + t.dateTo + '" value="' + dateTo + '">' +
                '</div>' +
                (events.length === 0
                    ? '<p class="kanban-empty">' + t.noEvents + '</p>'
                    : '<table class="kanban-events-table">' +
                      '<thead>' + renderHeader(t, sortState) + '</thead>' +
                      '<tbody>' + renderRows(sortEvents(events, sortState)) + '</tbody>' +
                      '</table>') +
                '</div>';

            var searchInput = document.getElementById('kanban-events-search');
            var fileFilterSelect = document.getElementById('kanban-events-file-filter');
            var fromFilter = document.getElementById('kanban-events-from-filter');
            var toFilter = document.getElementById('kanban-events-to-filter');
            var dateFromInput = document.getElementById('kanban-events-date-from');
            var dateToInput = document.getElementById('kanban-events-date-to');
            var thead = wrap.querySelector('thead');
            var tbody = wrap.querySelector('tbody');

            var applyFilters = function() {
                if (!tbody) return;
                var q = searchInput.value.toLowerCase();
                var fromVal = fromFilter.value;
                var toVal = toFilter.value;
                var filtered = events.filter(function(e) {
                    if (fromVal && e.fromStatus !== fromVal) return false;
                    if (toVal && e.toStatus !== toVal) return false;
                    return e.filePath.toLowerCase().includes(q) ||
                        (e.fromStatus || '').toLowerCase().includes(q) ||
                        e.toStatus.toLowerCase().includes(q);
                });
                tbody.innerHTML = renderRows(sortEvents(filtered, sortState));
            };

            var attachSortHandlers = function() {
                wrap.querySelectorAll('.kanban-events-sortable').forEach(function(th) {
                    th.addEventListener('click', function() {
                        var column = th.dataset.column;
                        var nextDir = (sortState.column === column && sortState.dir === 'asc') ? 'desc' : 'asc';
                        sortState = { column: column, dir: nextDir };
                        thead.innerHTML = renderHeader(t, sortState);
                        attachSortHandlers();
                        applyFilters();
                    });
                });
            };

            var reload = function() {
                load(opts, dateFromInput.value, dateToInput.value, sortState, fileFilterSelect.value);
            };

            searchInput.addEventListener('input', applyFilters);
            fromFilter.addEventListener('change', applyFilters);
            toFilter.addEventListener('change', applyFilters);
            fileFilterSelect.addEventListener('change', reload);
            dateFromInput.addEventListener('change', reload);
            dateToInput.addEventListener('change', reload);

            if (thead) attachSortHandlers();
        })
        .catch(function() {
            wrap.innerHTML = '<p>' + t.failedToLoad + '</p>';
        });
}
