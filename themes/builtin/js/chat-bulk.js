// theme: builtin
var chatSelected = new Set();

function chatBindMessageClick(root) {
    (root || document).querySelectorAll('.chat-message:not([data-bulk-bound])').forEach(function (msg) {
        msg.setAttribute('data-bulk-bound', '1');
        msg.addEventListener('click', function (e) {
            if (e.target.closest('button,a,textarea,input')) return;
            var id = msg.getAttribute('data-id');
            if (!id) return;
            if (chatSelected.has(id)) {
                chatSelected.delete(id);
                msg.classList.remove('chat-selected');
            } else {
                chatSelected.add(id);
                msg.classList.add('chat-selected');
            }
            chatSelectionChanged();
        });
    });
}

document.addEventListener('DOMContentLoaded', function () { chatBindMessageClick(document); });
document.addEventListener('htmx:afterSettle', function (e) { chatBindMessageClick(e.target); });

function chatSelectionChanged() {
    var bar = document.getElementById('chat-bulk-bar');
    if (!bar) return;
    bar.style.display = chatSelected.size > 0 ? 'flex' : 'none';
    var countEl = bar.querySelector('.chat-bulk-count');
    if (countEl) countEl.textContent = chatSelected.size + ' selected';
}

function chatBulkGetIDs() {
    return Array.from(chatSelected).join(',');
}

function chatBulkClear() {
    chatSelected.forEach(function (id) {
        var el = document.getElementById('chat-message-' + id);
        if (el) el.classList.remove('chat-selected');
    });
    chatSelected.clear();
    chatBulkCancelForm();
    chatSelectionChanged();
}

function chatBulkToNewFile() { chatBulkShowForm('new'); }
function chatBulkAppend() { chatBulkShowForm('append'); }

function chatBulkShowForm(mode) {
    var existing = document.getElementById('chat-bulk-move-form');
    if (existing) existing.remove();
    var bar = document.getElementById('chat-bulk-bar');
    if (!bar) return;
    htmx.ajax('GET', '/api/chat/bulk-form?mode=' + mode, { target: bar, swap: 'afterend' });
}

function chatBulkCancelForm() {
    var form = document.getElementById('chat-bulk-move-form');
    if (form) form.remove();
}

function chatBulkSubmit(mode) {
    var ids = chatBulkGetIDs();
    var target = (document.getElementById('chat-bulk-target') || {}).value || '';
    var editor = (document.getElementById('chat-bulk-editor') || {}).value || '';
    if (!target) return;
    fetch('/api/chat/messages/bulk/move', {
        method: 'POST',
        headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
        body: 'ids=' + encodeURIComponent(ids) + '&mode=' + encodeURIComponent(mode)
            + '&target=' + encodeURIComponent(target) + '&editor=' + encodeURIComponent(editor)
    }).then(function (res) { return res.text(); }).then(function (html) {
        chatSelected.forEach(function (id) {
            var el = document.getElementById('chat-message-' + id);
            if (el) el.remove();
        });
        chatBulkClear();
        var history = document.getElementById('component-chat-history');
        if (history && html) {
            var tmp = document.createElement('div');
            tmp.innerHTML = html;
            history.insertBefore(tmp.firstChild, history.firstChild);
        }
    });
}

function chatBulkDelete() {
    if (!confirm('Delete selected messages?')) return;
    fetch('/api/chat/messages/bulk', {
        method: 'DELETE',
        headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
        body: 'ids=' + encodeURIComponent(chatBulkGetIDs())
    }).then(function () {
        chatSelected.forEach(function (id) {
            var el = document.getElementById('chat-message-' + id);
            if (el) el.remove();
        });
        chatBulkClear();
    });
}

// ================================================================
// compact message kebab menu (to new file / append / delete)
// ================================================================
function closeAllChatShortMenus() {
    document.querySelectorAll('.chat-short-menu').forEach(function (m) {
        m.hidden = true;
    });
}

function positionChatShortMenu(btn, menu) {
    var rect = btn.getBoundingClientRect();
    menu.style.left = 'auto';
    menu.style.right = (window.innerWidth - rect.right) + 'px';
    menu.style.top = (rect.bottom + 2) + 'px';
    menu.style.bottom = 'auto';

    var menuRect = menu.getBoundingClientRect();
    if (menuRect.bottom > window.innerHeight) {
        // not enough room below — open upward instead
        menu.style.top = 'auto';
        menu.style.bottom = (window.innerHeight - rect.top + 2) + 'px';
    }
}

function toggleChatShortMenu(btn) {
    var menu = btn.parentElement.querySelector('.chat-short-menu');
    if (!menu) return;
    var wasHidden = menu.hidden;
    closeAllChatShortMenus();
    if (wasHidden) {
        menu.hidden = false;
        positionChatShortMenu(btn, menu);
    }
}

document.addEventListener('click', function (e) {
    if (!e.target.closest('.chat-short-menu-wrap')) {
        closeAllChatShortMenus();
    }
});

// close on scroll so the menu doesn't stay stuck at a stale position
document.addEventListener(
    'scroll',
    function () {
        closeAllChatShortMenus();
    },
    true,
);
