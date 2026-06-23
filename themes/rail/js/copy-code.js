// theme: rail
(function () {
    function addCopyButtons(root) {
        root.querySelectorAll('pre:not([data-copy-ready]):not(.CodeMirror-line)').forEach(function (pre) {
            pre.setAttribute('data-copy-ready', '1');
            var btn = document.createElement('button');
            btn.className = 'code-copy-btn';
            btn.textContent = 'copy';
            btn.addEventListener('click', function () {
                var code = pre.querySelector('code');
                navigator.clipboard.writeText(code ? code.innerText : pre.innerText).then(function () {
                    btn.textContent = 'copied!';
                    setTimeout(function () { btn.textContent = 'copy'; }, 1500);
                });
            });
            pre.appendChild(btn);
        });
    }
    addCopyButtons(document);
    document.addEventListener('htmx:afterSettle', function (e) {
        addCopyButtons(e.target);
    });
})();
