function toggleConflictDiff(btn, id, url) {
	var c = document.getElementById(id);
	if (c.innerHTML !== '') { c.innerHTML = ''; btn.textContent = btn.dataset.show; return; }
	htmx.ajax('GET', url, {target: '#' + id, swap: 'innerHTML'});
	btn.textContent = btn.dataset.hide;
}
