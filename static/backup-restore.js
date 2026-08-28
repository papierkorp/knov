// theme: builtin — backup restore confirmation, same shared-popover-modal mechanic as
// rename/move/delete (see panel-file.js): a click fills in the shared form's hx-post target and
// the modal's message text, then the button's own popovertarget opens the modal natively - no
// JS involved in showing/hiding it.
document.addEventListener("click", function (e) {
	var btn = e.target.closest(".backup-restore-btn");
	if (!btn) return;

	var form = document.getElementById("restore-form");
	var messageEl = document.getElementById("restore-modal-message");
	var warningEl = document.getElementById("restore-modal-warning");
	if (!form || !messageEl || !warningEl) return;

	messageEl.textContent = btn.dataset.message;
	warningEl.textContent = btn.dataset.warning || "";
	warningEl.hidden = !btn.dataset.warning;
	form.setAttribute("hx-post", btn.dataset.url);
	htmx.process(form);
});
