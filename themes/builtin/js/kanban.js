// theme: builtin
(function () {
    var cfg = window.KANBAN_CONFIG || {};
    var board = cfg.board || '';
    var archiveStatus = cfg.archiveStatus || '';
    var eventsModulePath = cfg.eventsModulePath || '';
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

    var dragging = null;
    var showingEvents = false;
    var kanbanEventsModule = null;

    window.kanbanToggleEvents = function () {
        var wrap = document.getElementById('view-kanban-board-wrap');
        var btn = document.getElementById('kanban-events-btn');
        var filterPanel = document.getElementById('kanban-filter-panel');
        if (!wrap || !btn) return;

        showingEvents = !showingEvents;

        if (showingEvents) {
            btn.title = t.backToBoard || '';
            btn.querySelector('i').className = 'fa fa-table-columns';
            btn.classList.replace('btn-secondary', 'btn-primary');
            if (filterPanel) filterPanel.setAttribute('hidden', '');
            wrap.innerHTML = '<p>' + (t.loading || '') + '</p>';
            var eventsT = {
                loading: t.loading,
                noEvents: t.noEvents,
                failedToLoad: t.failedToLoad,
                filterPlaceholder: t.filterPlaceholder,
                time: t.time,
                file: t.file,
                from: t.from,
                to: t.to,
                all: t.all,
                dateFrom: t.dateFrom,
                dateTo: t.dateTo,
                dateFormat: cfg.dateFormat
            };
            (kanbanEventsModule ? Promise.resolve(kanbanEventsModule) : import(eventsModulePath))
                .then(function (mod) {
                    kanbanEventsModule = mod;
                    mod.show({ wrap: wrap, board: board, t: eventsT });
                })
                .catch(function () {
                    wrap.innerHTML = '<p>' + (t.failedToLoad || '') + '</p>';
                });
        } else {
            btn.title = t.showEvents || '';
            btn.querySelector('i').className = 'fa fa-timeline';
            btn.classList.replace('btn-primary', 'btn-secondary');
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
