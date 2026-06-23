// theme: builtin
function adjustFontSize(delta) {
    var content = document.querySelector('#view-fileview-reader .file-content, .view-reader .file-content');
    if (!content) return;
    var cur = parseInt(getComputedStyle(content).fontSize, 10);
    content.style.fontSize = Math.max(14, Math.min(28, cur + delta)) + 'px';
}
