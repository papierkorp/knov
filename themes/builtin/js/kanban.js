// theme: builtin
(function () {
    var cfg = window.KANBAN_CONFIG || {};
    var board = cfg.board || '';
    var archiveStatus = cfg.archiveStatus || '';
    var t = cfg.t || {};

    var ancestorSel = document.getElementById('kanban-ancestor-select');
    if (ancestorSel) {
        var url = ancestorSel.getAttribute('hx-get-ancestors');
        if (url) {
            fetch(url, { headers: { Accept: 'text/html' } })
                .then(function (r) { return r.text(); })
                .then(function (html) { ancestorSel.insertAdjacentHTML('beforeend', html); });
        }
    }

    var tagSel = document.getElementById('kanban-tag-select');
    if (tagSel) {
        var tagUrl = tagSel.getAttribute('hx-get-tags');
        if (tagUrl) {
            fetch(tagUrl, { headers: { Accept: 'text/html' } })
                .then(function (r) { return r.text(); })
                .then(function (html) { tagSel.insertAdjacentHTML('beforeend', html); });
        }
    }

    // toolbar overflow menu (board history / show events / show archive), same
    // pattern as the chat kebab menu (chat-bulk.js): toggle + viewport-aware
    // positioning + close on outside click or scroll.
    function closeAllKanbanMenus() {
        document.querySelectorAll('.kanban-menu').forEach(function (m) {
            m.hidden = true;
        });
    }

    function positionKanbanMenu(btn, menu) {
        var rect = btn.getBoundingClientRect();
        menu.style.left = 'auto';
        menu.style.right = (window.innerWidth - rect.right) + 'px';
        menu.style.top = (rect.bottom + 2) + 'px';
        menu.style.bottom = 'auto';

        var menuRect = menu.getBoundingClientRect();
        if (menuRect.bottom > window.innerHeight) {
            menu.style.top = 'auto';
            menu.style.bottom = (window.innerHeight - rect.top + 2) + 'px';
        }
    }

    window.toggleKanbanMenu = function (btn) {
        var menu = btn.parentElement.querySelector('.kanban-menu');
        if (!menu) return;
        var wasHidden = menu.hidden;
        closeAllKanbanMenus();
        if (wasHidden) {
            menu.hidden = false;
            positionKanbanMenu(btn, menu);
        }
    };

    document.addEventListener('click', function (e) {
        if (!e.target.closest('.kanban-menu-wrap')) {
            closeAllKanbanMenus();
        }
    });

    document.addEventListener(
        'scroll',
        function () {
            closeAllKanbanMenus();
        },
        true,
    );

    var dragging = null;
    var showingEvents = false;
    var showingArchive = false;
    var eventsSortState = { column: 'time', dir: 'desc' };
    var archiveSortState = { column: 'lastedited', dir: 'desc' };

    // generic click-to-sort: reorders the already-rendered <tr> nodes by a data-*
    // attribute, no re-fetch/re-render needed. Shared by events and archive tables.
    function sortTableRows(tbodyId, theadId, arrowClass, sortState, column) {
        var tbody = document.getElementById(tbodyId);
        var thead = document.getElementById(theadId);
        if (!tbody) return;

        sortState.dir = (sortState.column === column && sortState.dir === 'asc') ? 'desc' : 'asc';
        sortState.column = column;
        var dir = sortState.dir === 'asc' ? 1 : -1;

        var rows = Array.prototype.slice.call(tbody.querySelectorAll('tr'));
        rows.sort(function (a, b) {
            var av = a.dataset[column] || '';
            var bv = b.dataset[column] || '';
            if (av < bv) return -1 * dir;
            if (av > bv) return 1 * dir;
            return 0;
        });
        rows.forEach(function (row) { tbody.appendChild(row); });

        if (thead) {
            thead.querySelectorAll('th').forEach(function (th) {
                th.querySelectorAll('.' + arrowClass).forEach(function (a) { a.remove(); });
                if (th.dataset.column === column) {
                    var arrow = document.createElement('span');
                    arrow.className = arrowClass;
                    arrow.textContent = sortState.dir === 'asc' ? '▲' : '▼';
                    th.appendChild(document.createTextNode(' '));
                    th.appendChild(arrow);
                }
            });
        }
    }

    window.applyKanbanArchiveFilters = function () {
        var q = ((document.getElementById('kanban-archive-search') || {}).value || '').toLowerCase().trim();
        var tag = (document.getElementById('kanban-archive-tag-filter') || {}).value || '';
        var rows = document.querySelectorAll('#kanban-archive-rows tr');
        rows.forEach(function (row) {
            var matchQ = q === '' || (row.dataset.search || '').indexOf(q) !== -1;
            var matchTag = tag === '' || (row.dataset.tags || '').indexOf('|' + tag + '|') !== -1;
            row.style.display = matchQ && matchTag ? '' : 'none';
        });
    };

    // events view — search + from/to filter the already-rendered rows client-side;
    // file/date changes trigger a real reload since they change what the server pulled.
    window.applyKanbanEventsFilters = function () {
        var q = ((document.getElementById('kanban-events-search') || {}).value || '').toLowerCase().trim();
        var fromVal = (document.getElementById('kanban-events-from-filter') || {}).value || '';
        var toVal = (document.getElementById('kanban-events-to-filter') || {}).value || '';
        var rows = document.querySelectorAll('#kanban-events-rows tr');
        rows.forEach(function (row) {
            var matchQ = q === '' || (row.dataset.search || '').indexOf(q) !== -1;
            var matchFrom = fromVal === '' || row.dataset.from === fromVal;
            var matchTo = toVal === '' || row.dataset.to === toVal;
            row.style.display = matchQ && matchFrom && matchTo ? '' : 'none';
        });
    };

    window.reloadKanbanEvents = function (eventsBoard) {
        var fileSel = document.getElementById('kanban-events-file-filter');
        var fromInput = document.getElementById('kanban-events-date-from');
        var toInput = document.getElementById('kanban-events-date-to');
        var fileFilter = fileSel ? fileSel.value : '';
        var limit = fileFilter ? 0 : 200;
        var url = '/api/kanban/' + eventsBoard + '/events?limit=' + limit;
        if (fromInput && fromInput.value) url += '&from=' + encodeURIComponent(fromInput.value);
        if (toInput && toInput.value) url += '&to=' + encodeURIComponent(toInput.value);
        if (fileFilter) url += '&file=' + encodeURIComponent(fileFilter);
        htmx.ajax('GET', url, { target: '#view-kanban-board-wrap', swap: 'innerHTML' });
    };

    window.sortKanbanEvents = function (column) {
        sortTableRows('kanban-events-rows', 'kanban-events-thead', 'kanban-events-sort-arrow', eventsSortState, column);
    };

    window.sortKanbanArchive = function (column) {
        sortTableRows('kanban-archive-rows', 'kanban-archive-thead', 'kanban-archive-sort-arrow', archiveSortState, column);
    };

    function resetArchiveButton() {
        showingArchive = false;
        var archiveBtn = document.getElementById('kanban-archive-btn');
        if (!archiveBtn) return;
        archiveBtn.title = t.showArchive || '';
        archiveBtn.querySelector('i').className = 'fa fa-box-archive';
        archiveBtn.classList.remove('kanban-menu-item--active');
    }

    function resetEventsButton() {
        showingEvents = false;
        var eventsBtn = document.getElementById('kanban-events-btn');
        if (!eventsBtn) return;
        eventsBtn.title = t.showEvents || '';
        eventsBtn.querySelector('i').className = 'fa fa-timeline';
        eventsBtn.classList.remove('kanban-menu-item--active');
    }

    window.kanbanToggleArchive = function () {
        var wrap = document.getElementById('view-kanban-board-wrap');
        var btn = document.getElementById('kanban-archive-btn');
        var filterPanel = document.getElementById('kanban-filter-panel');
        if (!wrap || !btn) return;

        closeAllKanbanMenus();
        showingArchive = !showingArchive;

        if (showingArchive) {
            if (showingEvents) resetEventsButton();
            btn.title = t.backToBoard || '';
            btn.querySelector('i').className = 'fa fa-table-columns';
            btn.classList.add('kanban-menu-item--active');
            if (filterPanel) filterPanel.setAttribute('hidden', '');
            archiveSortState = { column: 'lastedited', dir: 'desc' };
            htmx.ajax('GET', '/api/kanban/' + board + '/archive', { target: '#view-kanban-board-wrap', swap: 'innerHTML' });
        } else {
            resetArchiveButton();
            htmx.ajax('GET', '/api/kanban/' + board, { target: '#view-kanban-board-wrap', swap: 'innerHTML' });
        }
    };

    window.kanbanToggleEvents = function () {
        var wrap = document.getElementById('view-kanban-board-wrap');
        var btn = document.getElementById('kanban-events-btn');
        var filterPanel = document.getElementById('kanban-filter-panel');
        if (!wrap || !btn) return;

        closeAllKanbanMenus();
        showingEvents = !showingEvents;

        if (showingEvents) {
            if (showingArchive) resetArchiveButton();
            btn.title = t.backToBoard || '';
            btn.querySelector('i').className = 'fa fa-table-columns';
            btn.classList.add('kanban-menu-item--active');
            if (filterPanel) filterPanel.setAttribute('hidden', '');
            eventsSortState = { column: 'time', dir: 'desc' };
            htmx.ajax('GET', '/api/kanban/' + board + '/events', { target: '#view-kanban-board-wrap', swap: 'innerHTML' });
        } else {
            resetEventsButton();
            htmx.ajax('GET', '/api/kanban/' + board, { target: '#view-kanban-board-wrap', swap: 'innerHTML' });
        }
    };

    window.kanbanSetTagFilter = function (tag) {
        var sel = document.getElementById('kanban-tag-select');
        if (!sel) return;
        sel.value = tag;
        sel.dispatchEvent(new Event('change'));
    };

    window.kanbanDragStart = function (e) {
        dragging = e.currentTarget;
        dragging.classList.add('dragging');
        e.dataTransfer.effectAllowed = 'move';
        e.dataTransfer.setData('text/plain', dragging.dataset.filepath);
        var zone = document.getElementById('kanban-archive-zone');
        if (zone && archiveStatus) zone.classList.add('kanban-archive-zone--active');
    };

    window.kanbanDragOver = function (e) {
        e.preventDefault();
        e.dataTransfer.dropEffect = 'move';
        var col = e.currentTarget;
        col.classList.add('drag-over');
        if (!dragging) return;
        var cardsDiv = col.querySelector('.kanban-cards');
        if (!cardsDiv) return;
        var after = getDragAfterElement(cardsDiv, e.clientY);
        if (after == null) {
            cardsDiv.appendChild(dragging);
        } else {
            cardsDiv.insertBefore(dragging, after);
        }
    };

    window.kanbanDragLeave = function (e) {
        if (!e.currentTarget.contains(e.relatedTarget)) {
            e.currentTarget.classList.remove('drag-over');
        }
    };

    window.kanbanDrop = function (e) {
        e.preventDefault();
        var col = e.currentTarget;
        col.classList.remove('drag-over');
        if (!dragging) return;
        var filepath = dragging.dataset.filepath;
        var oldStatus = dragging.dataset.status;
        var newStatus = col.dataset.status;
        dragging.dataset.status = newStatus;
        dragging.classList.remove('dragging');
        dragging = null;
        updateColumnCount(oldStatus);
        updateColumnCount(newStatus);
        if (oldStatus !== newStatus) {
            var body = new URLSearchParams();
            body.append('filepath', filepath);
            body.append('status', newStatus);
            body.append('board', board);
            fetch('/api/kanban/card/move', { method: 'POST', headers: { 'Content-Type': 'application/x-www-form-urlencoded' }, body: body.toString() })
                .catch(function (err) { console.error('kanban move error', err); });
            saveColumnOrder(oldStatus);
        }
        saveColumnOrder(newStatus);
    };

    window.kanbanArchiveDragOver = function (e) {
        e.preventDefault();
        e.dataTransfer.dropEffect = 'move';
        e.currentTarget.classList.add('drag-over');
    };

    window.kanbanArchiveDragLeave = function (e) {
        if (!e.currentTarget.contains(e.relatedTarget)) {
            e.currentTarget.classList.remove('drag-over');
        }
    };

    window.kanbanArchiveDrop = function (e) {
        e.preventDefault();
        var zone = e.currentTarget;
        zone.classList.remove('drag-over');
        zone.classList.remove('kanban-archive-zone--active');
        if (!dragging || !archiveStatus) return;
        var filepath = dragging.dataset.filepath;
        var oldStatus = dragging.dataset.status;
        dragging.remove();
        dragging = null;
        updateColumnCount(oldStatus);
        var body = new URLSearchParams();
        body.append('filepath', filepath);
        body.append('status', archiveStatus);
        body.append('board', board);
        fetch('/api/kanban/card/move', { method: 'POST', headers: { 'Content-Type': 'application/x-www-form-urlencoded' }, body: body.toString() })
            .catch(function (err) { console.error('kanban archive error', err); });
    };

    document.addEventListener('dragend', function () {
        if (dragging) { dragging.classList.remove('dragging'); dragging = null; }
        document.querySelectorAll('.kanban-column').forEach(function (c) { c.classList.remove('drag-over'); });
        var zone = document.getElementById('kanban-archive-zone');
        if (zone) { zone.classList.remove('kanban-archive-zone--active'); zone.classList.remove('drag-over'); }
    });

    function getDragAfterElement(container, y) {
        var cards = Array.prototype.slice.call(container.querySelectorAll('.kanban-card:not(.dragging)'));
        var result = cards.reduce(function (closest, child) {
            var box = child.getBoundingClientRect();
            var offset = y - box.top - box.height / 2;
            if (offset < 0 && offset > closest.offset) {
                return { offset: offset, element: child };
            }
            return closest;
        }, { offset: Number.NEGATIVE_INFINITY, element: null });
        return result.element;
    }

    function saveColumnOrder(status) {
        var col = document.getElementById('kanban-col-' + status);
        if (!col) return;
        var paths = Array.prototype.slice.call(col.querySelectorAll('.kanban-card')).map(function (c) { return c.dataset.filepath; });
        var body = new URLSearchParams();
        body.append('status', status);
        body.append('order', paths.join(','));
        fetch('/api/kanban/' + board + '/order', { method: 'POST', headers: { 'Content-Type': 'application/x-www-form-urlencoded' }, body: body.toString() })
            .catch(function (err) { console.error('kanban order error', err); });
    }

    function updateColumnCount(status) {
        var col = document.getElementById('kanban-col-' + status);
        if (!col) return;
        var badge = col.querySelector('.kanban-column-count');
        if (badge) badge.textContent = col.querySelectorAll('.kanban-card').length;
    }
})();
