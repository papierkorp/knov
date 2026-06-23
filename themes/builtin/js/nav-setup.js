// theme: builtin
const path = window.location.pathname;

const dashMatch = path.match(/\/dashboard\/([^\/]+)/);
if (dashMatch) {
    const id = dashMatch[1];
    document.getElementById('edit-link').href = `/dashboard/edit/${id}`;
    document.getElementById('rename-form').setAttribute('hx-post', `/api/dashboards/${id}/rename`);
    document.getElementById('delete-form').setAttribute('hx-delete', `/api/dashboards/${id}`);
}

const fileMatch = path.match(/\/files\/(.+)/);
if (fileMatch && !path.includes('/edit/')) {
    const filepath = fileMatch[1];
    document.getElementById('edit-link').href = `/files/edit/${filepath}`;
    document.getElementById('history-link').href = `/files/history/${filepath}`;
    document.getElementById('rename-form').setAttribute('hx-post', `/api/files/rename/${filepath}`);
    document.getElementById('delete-form').setAttribute('hx-delete', `/api/files/delete/${filepath}`);
    document.getElementById('rename-input').value = filepath;

    const rebuildBtn = document.getElementById('rebuild-metadata');
    rebuildBtn.setAttribute('hx-post', `/api/metadata/rebuild/${filepath}`);
    rebuildBtn.setAttribute('hx-target', '#rebuild-metadata-result');

    const refFilepath = document.getElementById('reference-filepath');
    if (refFilepath) refFilepath.value = filepath;

    const referencesModal = document.getElementById('references-modal');
    if (referencesModal) {
        referencesModal.addEventListener('beforetoggle', (e) => {
            if (e.newState === 'open') {
                document.body.dispatchEvent(new Event('referencesOpen'));
            }
        });
    }

    document.getElementById('export-markdown-link').href = `/api/files/export/markdown?filepath=${filepath}`;
}

const filterMatch = path.match(/^\/filters\/(?!new$|edit\/)(.+)/);
if (filterMatch) {
    const filterId = filterMatch[1];
    document.getElementById('edit-link').href = `/filters/edit/${filterId}`;
    document.getElementById('history-link').removeAttribute('href');
    document.getElementById('delete-form').setAttribute('hx-delete', `/api/filters/${filterId}`);
}

function positionPopover(popoverId) {
    const trigger = document.querySelector(`[popovertarget="${popoverId}"]`);
    const popover = document.getElementById(popoverId);
    if (!trigger || !popover) return;
    popover.addEventListener('beforetoggle', (event) => {
        if (event.newState === 'open') {
            requestAnimationFrame(() => {
                const triggerRect = trigger.getBoundingClientRect();
                const popoverRect = popover.getBoundingClientRect();
                const left = triggerRect.right - popoverRect.width;
                const top = triggerRect.bottom + 8;
                const maxLeft = window.innerWidth - popoverRect.width - 8;
                const maxTop = window.innerHeight - popoverRect.height - 8;
                popover.style.left = Math.max(8, Math.min(left, maxLeft)) + 'px';
                popover.style.top = Math.max(8, Math.min(top, maxTop)) + 'px';
            });
        }
    });
}

function openNotificationsPopover() {
    var pop = document.getElementById('notifications-popover');
    var content = document.getElementById('notifications-popover-content');
    pop.showPopover();
    htmx.ajax('GET', '/api/notifications', {
        target: content,
        swap: 'innerHTML',
        headers: { 'Accept': 'text/html' }
    });
}

document.addEventListener('DOMContentLoaded', () => {
    positionPopover('add-dropdown');
    positionPopover('edit-dropdown-content');
    positionPopover('menu-dropdown');
    positionPopover('references-modal');
});
