// theme: builtin-reworked — "media" rail snippet: all/used/orphaned filter
// switch, client-side name filter, and a hidden-file warning banner fed by
// a response header on the list request. instanceID is groupID + "-media",
// matching the ids `body` renders.

railSnippetByID("media").body = (groupID, instanceID) => `<div class="fp-media-controls">
    <input type="text" id="fp-${instanceID}-search" placeholder="filter..." oninput="filterMediaList('${instanceID}', this.value)"/>
    <div class="fp-media-filter-btns">
      <button class="fp-media-btn active" onclick="switchMediaFilter('${instanceID}', 'all', this)">all</button>
      <button class="fp-media-btn" onclick="switchMediaFilter('${instanceID}', 'used', this)">used</button>
      <button class="fp-media-btn" onclick="switchMediaFilter('${instanceID}', 'orphaned', this)">orphaned</button>
      <a href="/browse/media" class="flyout-header-link" title="open media page"><i class="fa fa-arrow-up-right-from-square"></i></a>
    </div>
  </div>
  <div id="fp-${instanceID}-warning" class="fp-media-warning" style="display:none;"></div>
  <div class="flyout-content" id="fp-${instanceID}-content" data-snippet="media"
       data-url="/api/media/list?mode=compact&filter=all" data-loaded="false">
    <span style="color:var(--text-secondary);font-style:italic;">loading...</span>
  </div>`;

// ================================================================
// media list filter (client-side, no extra API call)
// ================================================================
function filterMediaList(instanceID, query) {
  const items = document.querySelectorAll(
    "#fp-" + instanceID + "-content .media-compact-item",
  );
  const q = query.toLowerCase();
  items.forEach((item) => {
    const name =
      item.querySelector(".media-compact-name")?.textContent.toLowerCase() ||
      "";
    item.style.display = name.includes(q) ? "" : "none";
  });
}

// ================================================================
// media panel — server-side filter switch + hidden warning
// ================================================================
function switchMediaFilter(instanceID, filter, btn) {
  // update active button
  btn.closest(".fp-media-filter-btns")
    ?.querySelectorAll(".fp-media-btn")
    .forEach((b) => b.classList.remove("active"));
  if (btn) btn.classList.add("active");

  // clear search input
  const search = document.getElementById("fp-" + instanceID + "-search");
  if (search) search.value = "";

  const el = document.getElementById("fp-" + instanceID + "-content");
  if (!el) return;

  const url = "/api/media/list?mode=compact&filter=" + filter;
  el.dataset.url = url;
  htmx.ajax("GET", url, {
    target: el,
    swap: "innerHTML",
    headers: { Accept: "text/html" },
  });
}

function updateMediaHiddenWarning(instanceID, message) {
  const el = document.getElementById("fp-" + instanceID + "-warning");
  if (!el) return;
  if (message) {
    el.textContent = message;
    el.style.display = "block";
  } else {
    el.style.display = "none";
  }
}
