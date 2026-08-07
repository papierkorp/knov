// theme: builtin — "chat" rail snippet: same generic content shape
// as browse/overview/etc. (its `.body` is assigned along with the rest of
// them in panel-content.js), plus scroll-to-top after every swap. Flagged
// `singleton: true` on its RAIL_SNIPPETS entry (rail-snippets.js) — its
// server-rendered bits (bulk-select bar, move-to-file forms, load-more
// button) all use page-wide fixed ids, so only one instance of it can
// safely exist across the whole rail layout at once; the settings builder
// (rail-layout-builder.js) enforces that when composing groups.

function scrollChatToTop() {
  var h = document.getElementById("component-chat-history");
  if (h) h.scrollTop = 0;
}

document.body.addEventListener("htmx:afterSwap", function (e) {
  var target = e.detail.target;
  if (target?.id === "component-chat-history" || target?.dataset?.snippet === "chat") {
    scrollChatToTop();
  }
});
