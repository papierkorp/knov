// theme: builtin — generic "content"-kind rail snippets (tree,
// browse, overview, tags, folders, collections, dashboards, editor,
// filters, notifications, latest changes, chat). They all share one shape:
// an optional filter/fold search row plus a lazy-loaded content div, keyed
// by instanceID = groupID + "-" + snippetID so the same snippet can sit in
// several groups at once. One `.body` assignment loop covers all of them —
// see panel-tree.js and panel-chat.js for the bit of extra behavior tree
// and chat each need on top of this shared shape.

function railGenericContentBody(groupID, snippet) {
  const instanceID = groupID + "-" + snippet.id;
  let searchRow = "";
  if (snippet.search === "generic") {
    searchRow = `<div class="fp-browse-search-row">
        <input type="text" placeholder="filter..." oninput="filterGroupContent('${instanceID}', this.value)"/>
        <button type="button" class="fp-fold-all-btn" title="collapse all" onclick="setAllGroupFolds('${instanceID}', true)"><i class="fa fa-angles-up"></i></button>
        <button type="button" class="fp-fold-all-btn" title="expand all" onclick="setAllGroupFolds('${instanceID}', false)"><i class="fa fa-angles-down"></i></button>
      </div>`;
  } else if (snippet.search === "latest") {
    searchRow = `<div class="fp-latest-search-wrap">
        <input type="search" placeholder="filter..." oninput="filterGroupContent('${instanceID}', this.value)" autocomplete="off"/>
        <div class="fp-latest-date-range">
          <input type="date" class="fp-latest-from" title="from date" onchange="filterLatestDateRange('${instanceID}')"/>
          <span>&ndash;</span>
          <input type="date" class="fp-latest-to" title="to date" onchange="filterLatestDateRange('${instanceID}')"/>
        </div>
      </div>`;
  }
  const url = snippet.url ? snippet.url(groupID) : "";
  return `${searchRow}<div class="flyout-content" id="fp-${instanceID}-content" data-snippet="${railEsc(snippet.id)}"
       data-url="${railEsc(url)}" data-loaded="false">
      <span style="color:var(--text-secondary);font-style:italic;">loading...</span>
    </div>`;
}

RAIL_SNIPPETS.filter((s) => s.kind === "content").forEach((s) => {
  s.body = (groupID) => railGenericContentBody(groupID, s);
});

// ================================================================
// search — dispatches to the debounced server search for "latest changes"
// (also applying the from/to date range alongside it) or the generic
// client-side filter for everything else. instanceID is groupID + "-" +
// snippetID, matching the content div's own id.
// ================================================================
let _latestSearchTimers = {};
function filterGroupContent(instanceID, query) {
  const content = document.getElementById("fp-" + instanceID + "-content");
  if (!content) return;

  if (content.dataset.snippet === "latest") {
    clearTimeout(_latestSearchTimers[instanceID]);
    _latestSearchTimers[instanceID] = setTimeout(() => {
      fetchLatestChanges(instanceID, query.trim());
    }, 300);
    return;
  }

  const q = query.toLowerCase();

  if (q === "") {
    content.querySelectorAll("li").forEach((li) => {
      li.style.display = "";
      if (li.dataset.wasCollapsed) {
        li.classList.add("fp-tree-collapsed");
        delete li.dataset.wasCollapsed;
      }
    });
    content.querySelectorAll(".media-compact-item").forEach(
      (item) => (item.style.display = ""),
    );
    return;
  }

  const isTree = content.querySelector("a.fp-tree-file") !== null;

  if (isTree) {
    // expand all collapsed dirs so matches inside them are visible
    content.querySelectorAll("li.fp-tree-collapsed").forEach((li) => {
      li.dataset.wasCollapsed = "1";
      li.classList.remove("fp-tree-collapsed");
    });

    content.querySelectorAll("li").forEach((li) => {
      const fileLink =
        li.querySelector(":scope > a.fp-tree-file") ||
        li.querySelector(":scope > span.browse-item-row > a.fp-tree-file");
      if (!fileLink) return;
      const text = fileLink.textContent.toLowerCase();
      const href = (fileLink.getAttribute("href") || "").toLowerCase();
      li.style.display = text.includes(q) || href.includes(q) ? "" : "none";
    });

    content.querySelectorAll("li").forEach((li) => {
      const hasDirBtn =
        li.querySelector(":scope > button.fp-tree-dir") ||
        li.querySelector(":scope > span.browse-item-row > button.fp-tree-dir");
      if (!hasDirBtn) return;
      const hasVisible = [...li.querySelectorAll("a.fp-tree-file")].some(
        (a) => a.closest("li").style.display !== "none",
      );
      li.style.display = hasVisible ? "" : "none";
    });
  } else if (content.querySelector("li")) {
    // flat list (overview, tags, folders, collections)
    content.querySelectorAll("li").forEach((li) => {
      const link = li.querySelector("a");
      if (!link) return;
      const text = link.textContent.toLowerCase();
      const href = (link.getAttribute("href") || "").toLowerCase();
      li.style.display = text.includes(q) || href.includes(q) ? "" : "none";
    });
  } else {
    // media-compact items (no li present) — e.g. media snippet mixed in
    content.querySelectorAll(".media-compact-item").forEach((item) => {
      const name =
        item.querySelector(".media-compact-name")?.textContent.toLowerCase() ||
        "";
      const href = (
        item.getAttribute("href") ||
        item.querySelector("a")?.getAttribute("href") ||
        ""
      ).toLowerCase();
      item.style.display = name.includes(q) || href.includes(q) ? "" : "none";
    });
  }
}

// fetches the "latest changes" content for instanceID, combining the filename
// query with the from/to date-range inputs in its search row (immediately,
// no debounce — used both by the debounced text filter and the date inputs'
// own onchange).
function fetchLatestChanges(instanceID, query) {
  const content = document.getElementById("fp-" + instanceID + "-content");
  if (!content) return;
  const row = content.previousElementSibling;
  const from = row?.querySelector(".fp-latest-from")?.value || "";
  const to = row?.querySelector(".fp-latest-to")?.value || "";
  let url = content.dataset.url || "/api/git/latestchanges?count=50";
  if (query) url += `&q=${encodeURIComponent(query)}`;
  if (from) url += `&from=${encodeURIComponent(from)}`;
  if (to) url += `&to=${encodeURIComponent(to)}`;
  htmx.ajax("GET", url, {
    target: content,
    swap: "innerHTML",
    headers: { Accept: "text/html" },
  });
}

function filterLatestDateRange(instanceID) {
  const row = document.getElementById("fp-" + instanceID + "-content")?.previousElementSibling;
  const query = row?.querySelector("input[type=search]")?.value.trim() || "";
  fetchLatestChanges(instanceID, query);
}

function setAllGroupFolds(instanceID, collapse) {
  const content = document.getElementById("fp-" + instanceID + "-content");
  if (!content) return;
  content.querySelectorAll("li").forEach((li) => {
    const hasDirBtn =
      li.querySelector(":scope > button.fp-tree-dir") ||
      li.querySelector(":scope > span.browse-item-row > button.fp-tree-dir");
    if (!hasDirBtn) return;
    li.classList.toggle("fp-tree-collapsed", collapse);
    delete li.dataset.wasCollapsed;
  });
}

// ================================================================
// shared post-swap hook: dashboard edit-links. Runs on every htmx swap
// regardless of which group/snippet — cheap no-op where it doesn't apply.
// (chat's own post-swap hook — scroll-to-top — lives in panel-chat.js.)
// ================================================================
function decorateDashboardLinks(container) {
  container.querySelectorAll('a[href^="/dashboard/"]').forEach((link) => {
    const match = link.getAttribute("href").match(/^\/dashboard\/([^/]+)$/);
    if (!match || link.closest(".fp-dash-row")) return;
    const id = match[1];
    const row = document.createElement("div");
    row.className = "fp-dash-row";
    link.replaceWith(row);
    link.className = "fp-dash-name";
    link.title = link.textContent;
    row.appendChild(link);
    const edit = document.createElement("a");
    edit.href = `/dashboard/edit/${id}`;
    edit.className = "fp-dash-edit";
    edit.title = "edit";
    edit.innerHTML = '<i class="fa fa-pen"></i>';
    row.appendChild(edit);
  });
}

document.body.addEventListener("htmx:afterSwap", function (e) {
  var target = e.detail.target;
  if (!target) return;
  if (target.classList?.contains("flyout-content")) {
    decorateDashboardLinks(target);
  }
});

// ================================================================
// browse-metadata drill-down (tags/folders/collections links that load a
// filtered file list into the same content div, with a "back" button)
// ================================================================
function initGroupInterceptor() {
  const flyout = document.getElementById("flyout");
  if (!flyout) return;

  flyout.addEventListener("click", function (e) {
    const link = e.target.closest('a[href*="/browse/"]');
    if (!link) return;

    const content = link.closest(".flyout-content");
    if (!content || !content.id.endsWith("-content")) return;

    const match = (link.getAttribute("href") || "").match(
      /\/browse\/([^/]+)\/(.+)/,
    );
    if (!match) return;

    e.preventDefault();
    const metaType = match[1];
    const value = decodeURIComponent(match[2]);
    const url = `/api/files/browse?metadata=${metaType}&value=${encodeURIComponent(value)}&actions=true`;
    const previousUrl = content.dataset.url;

    htmx.ajax("GET", url, {
      target: content,
      swap: "innerHTML",
      headers: { Accept: "text/html" },
    });
    content.dataset.loaded = "true";

    content.addEventListener("htmx:afterSwap", function addBack() {
      content.removeEventListener("htmx:afterSwap", addBack);
      const btn = document.createElement("button");
      btn.className = "fp-browse-back";
      const groupID = content.closest(".flyout-panel")?.id.replace(/^fp-/, "");
      const modeBtn = document.querySelector(
        '.fp-browse-mode-btn[data-mode="' + content.dataset.snippet + '"]',
      );
      const label = modeBtn?.title || document.getElementById("rb-" + groupID)?.title || "back";
      btn.textContent = "← " + label;
      btn.onclick = () => {
        content.dataset.url = previousUrl;
        htmx.ajax("GET", previousUrl, {
          target: content,
          swap: "innerHTML",
          headers: { Accept: "text/html" },
        });
      };
      content.insertBefore(btn, content.firstChild);
    });

    const instanceID = content.id.replace(/^fp-/, "").replace(/-content$/, "");
    const search = document.getElementById("fp-" + instanceID + "-search");
    if (search) search.value = "";
  });
}
