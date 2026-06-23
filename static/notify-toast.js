(function () {
    var container = document.getElementById('component-notify');
    var DURATION = parseInt(container.dataset.duration, 10) || 3500;

    function showToast(type, message) {
        var toast = document.createElement('div');
        toast.className = 'notify-toast notify-' + type;
        toast.textContent = message;
        toast.addEventListener('click', function () { dismiss(toast); });
        container.appendChild(toast);
        setTimeout(function () { dismiss(toast); }, DURATION);
    }

    function dismiss(toast) {
        toast.style.animation = 'notify-out 0.2s ease forwards';
        setTimeout(function () {
            if (toast.parentNode) { toast.parentNode.removeChild(toast); }
        }, 200);
    }

    document.body.addEventListener('notify', function (e) {
        var detail = e.detail;
        if (detail && detail.type && detail.message) {
            showToast(detail.type, detail.message);
        }
    });

    fetch('/api/notifications/flash')
        .then(function (r) { return r.ok ? r.json() : null; })
        .then(function (data) {
            if (data && data.level && data.message) {
                showToast(data.level, data.message);
            }
        })
        .catch(function () {});
})();
