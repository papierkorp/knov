// theme: builtin — in-app confirmation dialog, same shared-popover-modal
// mechanic as rename/move/delete: the #confirm-modal markup lives in
// base.gohtml's rail-modals, this only fills its message and shows/hides it.
//
// Every element carrying hx-confirm routes through here via the htmx:confirm
// event instead of the browser's window.confirm. Other scripts can call
// window.showConfirm(message) directly and await the boolean result.

(function () {
  var resolveFn = null;

  // resolve the pending promise (if any) and close the popover
  function settle(result) {
    if (!resolveFn) return;
    var fn = resolveFn;
    resolveFn = null;
    fn(result);
    var modal = document.getElementById("confirm-modal");
    try { if (modal) modal.hidePopover(); } catch (_) {}
  }

  window.showConfirm = function (message) {
    var modal = document.getElementById("confirm-modal");
    var msg = document.getElementById("confirm-modal-message");
    // no modal on this page (e.g. a bare editor iframe) — fall back
    if (!modal || !msg) return Promise.resolve(window.confirm(message));
    settle(false); // cancel any in-flight prompt
    msg.textContent = message || "";
    return new Promise(function (res) {
      resolveFn = res;
      try { modal.showPopover(); } catch (_) {}
    });
  };

  document.addEventListener("click", function (ev) {
    if (ev.target.closest("#confirm-modal-ok")) settle(true);
    else if (ev.target.closest("#confirm-modal-cancel")) settle(false);
  });

  // Esc / light-dismiss closes the popover — count that as cancel.
  // toggle doesn't bubble, so listen in the capture phase.
  document.addEventListener("toggle", function (ev) {
    if (ev.target.id === "confirm-modal" && ev.newState === "closed") settle(false);
  }, true);

  // htmx 4.0: htmx:confirm only fires when hx-confirm is set; the text is
  // e.detail.ctx.confirm; resume via issueRequest() / dropRequest().
  // preventDefault() suppresses the native window.confirm fallback.
  document.body.addEventListener("htmx:confirm", function (e) {
    e.preventDefault();
    window.showConfirm(e.detail.ctx.confirm).then(function (ok) {
      if (ok) e.detail.issueRequest();
      else e.detail.dropRequest();
    });
  });
})();
