// theme: rail

// ================================================================
// media list filter (client-side, no extra API call)
// ================================================================
function filterMediaList(query) {
    const items = document.querySelectorAll('#fp-media-content .media-compact-item');
    const q = query.toLowerCase();
    items.forEach(item => {
        const name = item.querySelector('.media-compact-name')?.textContent.toLowerCase() || '';
        item.style.display = name.includes(q) ? '' : 'none';
    });
}

// ================================================================
// close flyout (used by home, settings, admin links)
// ================================================================
function closePanel() {
    const flyout = document.getElementById('flyout');
    flyout.querySelectorAll('.flyout-panel').forEach(p => p.classList.remove('active'));
    document.querySelectorAll('#rail-site .rail-btn').forEach(b => b.classList.remove('active'));
    flyout.removeAttribute('data-active');
    localStorage.removeItem('rail-panel');
}

// ================================================================
// panel toggle — single shared flyout
// ================================================================
function togglePanel(panelId) {
    const flyout  = document.getElementById('flyout');
    const panels  = flyout.querySelectorAll('.flyout-panel');
    const target  = document.getElementById(panelId);
    const railBtn = document.getElementById('rb-' + panelId.replace('fp-', ''));
    const isOpen  = target.classList.contains('active');

    panels.forEach(p => p.classList.remove('active'));
    document.querySelectorAll('#rail-site .rail-btn').forEach(b => b.classList.remove('active'));

    if (isOpen) {
        flyout.removeAttribute('data-active');
        localStorage.removeItem('rail-panel');
    } else {
        target.classList.add('active');
        flyout.setAttribute('data-active', panelId);
        railBtn?.classList.add('active');
        lazyLoad(panelId);
        localStorage.setItem('rail-panel', panelId);
    }
}

// ================================================================
// lazy load panel content on first open
// ================================================================
function lazyLoad(panelId) {
    const el = document.getElementById(panelId + '-content');
    if (!el || el.dataset.loaded === 'true') return;
    const url = el.dataset.url;
    if (!url) return;
    el.dataset.loaded = 'true';
    htmx.ajax('GET', url, {target: el, swap: 'innerHTML', headers: {'Accept': 'text/html'}});
}

// ================================================================
// search title-only toggle
// ================================================================
function updateSearchMode(cb) {
    const input = document.getElementById('fp-search-input');
    if (!input) return;
    const base = '/api/search?format=list';
    input.setAttribute('hx-get', cb.checked ? base + '&titleonly=true' : base);
    htmx.process(input);
}

// ================================================================
// file sub-panel switching
// ================================================================
function switchFileSubPanel(view) {
    document.querySelectorAll('.fp-sub-panel').forEach(p => p.classList.remove('active'));
    const target = document.getElementById('fps-' + view);
    if (target) target.classList.add('active');
}

// ================================================================
// file page setup
// ================================================================
function setupFilePage() {
    const path = window.location.pathname;

    // dashboard modals
    const dashMatch = path.match(/^\/dashboard\/([^/]+)/);
    if (dashMatch && !path.includes('/edit/') && !path.includes('/new')) {
        const id = dashMatch[1];
        document.getElementById('rename-form')?.setAttribute('hx-post', '/api/dashboards/' + id + '/rename');
        document.getElementById('delete-form')?.setAttribute('hx-delete', '/api/dashboards/' + id);
        htmx.process(document.getElementById('rename-form'));
        htmx.process(document.getElementById('delete-form'));
    }

    // filter modals
    const filterMatch = path.match(/^\/filters\/(?!new$|edit\/)(.+)/);
    if (filterMatch) {
        document.getElementById('delete-form')?.setAttribute('hx-delete', '/api/filters/' + filterMatch[1]);
        htmx.process(document.getElementById('delete-form'));
    }

    // edit pages — show file panel with metadata
    const editMatch = path.match(/^\/files\/edit\/(.+)/);
    if (editMatch) {
        const filepath = editMatch[1].split('?')[0];
        const fp = encodeURIComponent(filepath);
        document.body.setAttribute('data-has-file', 'true');
        const noFile = document.getElementById('fp-no-file');
        if (noFile) noFile.style.display = 'none';
        const pathEl = document.getElementById('fp-meta-path');
        if (pathEl) pathEl.innerHTML = '<a href="/files/' + fp + '">' + filepath + '</a>';
        const refFp = document.getElementById('fp-reference-filepath');
        if (refFp) refFp.value = filepath;
        const editFields = {
            'fp-meta-created':    '/api/metadata/createdat?filepath='  + fp,
            'fp-meta-edited':     '/api/metadata/lastedited?filepath=' + fp,
            'fp-meta-tags':       '/api/metadata/tags?filepath='       + fp,
            'fp-meta-collection': '/api/metadata/collection?filepath=' + fp,
            'fp-meta-folders':    '/api/metadata/folders?filepath='    + fp,
        };
        for (const [id, url] of Object.entries(editFields)) {
            const el = document.getElementById(id);
            if (!el) continue;
            fetch(url, {headers: {'Accept': 'text/html'}})
                .then(r => r.text())
                .then(html => { el.innerHTML = html; })
                .catch(() => {});
        }
        htmx.ajax('GET', '/api/metadata/references?filepath=' + fp, {
            target: document.getElementById('component-references-list'),
            swap: 'outerHTML',
            headers: {'Accept': 'text/html'},
        });
        togglePanel('fp-file');
        return true;
    }

    // file pages
    const fileMatch = path.match(/^\/files\/(?!edit\/|new\/|history\/)(.+)/);
    if (!fileMatch) return false;

    const filepath = fileMatch[1];
    const fp = encodeURIComponent(filepath);

    // reveal file rail button
    document.body.setAttribute('data-has-file', 'true');

    // populate filename header
    const titleEl = document.getElementById('fp-file-title');
    if (titleEl) titleEl.textContent = filepath.split('/').pop();

    // wire action buttons
    const editLink = document.getElementById('fp-edit-link');
    if (editLink) editLink.href = '/files/edit/' + filepath;

    const rebuildBtn = document.getElementById('fp-rebuild-btn');
    if (rebuildBtn) {
        rebuildBtn.setAttribute('hx-post',   '/api/metadata/rebuild/' + filepath);
        rebuildBtn.setAttribute('hx-target', '#fp-rebuild-result');
        htmx.process(rebuildBtn);
    }

    const renameForm  = document.getElementById('rename-form');
    const renameInput = document.getElementById('rename-input');
    if (renameForm)  { renameForm.setAttribute('hx-post', '/api/files/rename/' + filepath); htmx.process(renameForm); }
    if (renameInput) renameInput.value = filepath;

    const deleteForm = document.getElementById('delete-form');
    if (deleteForm)  { deleteForm.setAttribute('hx-delete', '/api/files/delete/' + filepath); htmx.process(deleteForm); }

    const refFp = document.getElementById('fp-reference-filepath');
    if (refFp) refFp.value = filepath;
    htmx.ajax('GET', '/api/metadata/references?filepath=' + fp, {
        target: document.getElementById('component-references-list'),
        swap: 'outerHTML',
        headers: {'Accept': 'text/html'},
    });

    // hide no-file message and show metadata rows
    const noFile = document.getElementById('fp-no-file');
    if (noFile) noFile.style.display = 'none';
    const pathEl = document.getElementById('fp-meta-path');
    if (pathEl) pathEl.innerHTML = '<a href="/files/' + fp + '">' + filepath + '</a>';

    const htmxFields = {
        'fp-meta-created':    '/api/metadata/createdat?filepath='  + fp,
        'fp-meta-edited':     '/api/metadata/lastedited?filepath=' + fp,
        'fp-meta-tags':       '/api/metadata/tags?filepath='       + fp,
        'fp-meta-collection': '/api/metadata/collection?filepath=' + fp,
        'fp-meta-folders':    '/api/metadata/folders?filepath='    + fp,
        'fp-ancestors':       '/api/links/ancestors?filepath='     + fp,
        'fp-parents':         '/api/links/parents?filepath='       + fp,
        'fp-children':        '/api/links/kids?filepath='          + fp,
        'fp-links-to':        '/api/links/used?filepath='          + fp,
        'fp-links-from':      '/api/links/linkstohere?filepath='   + fp,
        'fp-related':         '/api/links/related?filepath='       + fp,
    };

    for (const [id, url] of Object.entries(htmxFields)) {
        const el = document.getElementById(id);
        if (!el) continue;
        fetch(url, {headers: {'Accept': 'text/html'}})
            .then(r => r.text())
            .then(html => { el.innerHTML = html; })
            .catch(() => {});
    }

    htmx.ajax('GET', '/api/files/versions/' + fp + '?output=sidebar', {
        target: document.getElementById('fp-versions'),
        swap: 'innerHTML',
        headers: {'Accept': 'text/html'},
    });

    // toc from hidden element rendered by fileview.gohtml
    const tocData = document.getElementById('fp-toc-data');
    const tocNav  = document.getElementById('fp-toc-nav');
    if (tocData && tocNav) tocNav.innerHTML = tocData.innerHTML;

    // auto-open file info panel
    togglePanel('fp-file');
    return true;
}

// ================================================================
// chat scroll — keep history pinned to bottom on load and new messages
// ================================================================
function scrollChatToBottom() {
    var h = document.getElementById('component-chat-history');
    if (h) h.scrollTop = h.scrollHeight;
}

document.addEventListener('htmx:afterSwap', function(e) {
    // scroll after initial load or new message swap into history
    var target = e.detail.target;
    if (!target) return;
    if (target.id === 'component-chat-history' ||
        target.id === 'fp-chat-content') {
        scrollChatToBottom();
    }
});

// ================================================================
// init
// ================================================================
document.addEventListener('DOMContentLoaded', () => {
    const isFilePage = setupFilePage();
    if (!isFilePage) {
        const saved = localStorage.getItem('rail-panel');
        if (saved && document.getElementById(saved)) togglePanel(saved);
    }
    // re-enable transitions after first state restore so user interactions animate
    requestAnimationFrame(() => {
        document.documentElement.classList.remove('no-transition');
    });
});
