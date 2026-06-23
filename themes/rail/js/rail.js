// theme: rail

// ================================================================
// media list filter (client-side, no extra API call)
// ================================================================
function filterMediaList(query) {
  const items = document.querySelectorAll(
    "#fp-media-content .media-compact-item",
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
function switchMediaFilter(filter, btn) {
  // update active button
  document
    .querySelectorAll(".fp-media-btn")
    .forEach((b) => b.classList.remove("active"));
  if (btn) btn.classList.add("active");

  // clear search input
  const search = document.getElementById("fp-media-search");
  if (search) search.value = "";

  const el = document.getElementById("fp-media-content");
  if (!el) return;

  const url = "/api/media/list?mode=compact&filter=" + filter;
  fetch(url, { headers: { Accept: "text/html" } })
    .then((r) => {
      const hidden = parseInt(r.headers.get("X-Hidden-Count") || "0", 10);
      updateMediaHiddenWarning(hidden);
      return r.text();
    })
    .then((html) => {
      el.innerHTML = html;
    });
}

function updateMediaHiddenWarning(hiddenCount) {
  const el = document.getElementById("fp-media-warning");
  if (!el) return;
  if (hiddenCount > 0) {
    el.textContent = hiddenCount + " files not shown (hidden in settings)";
    el.style.display = "block";
  } else {
    el.style.display = "none";
  }
}

// ================================================================
// browse panel — mode switch + title filter
// ================================================================
function switchBrowseMode(mode) {
  const el = document.getElementById("fp-browse-content");
  if (!el) return;
  const urls = {
    tree: "/api/files/tree?actions=true",
    browse: "/api/files/folder?path=&target=%23fp-browse-content",
    overview: "/api/files/list?actions=true",
    tags: "/api/metadata/tags?actions=true",
    folders: "/api/metadata/folders?actions=true",
    collections: "/api/metadata/collections?actions=true",
    dashboards: "/api/dashboards",
    editor: "/api/metadata/editors",
    filters: "/api/files/browse?metadata=editor&value=filter-editor",
    notifications: "/api/notifications",
  };
  const url = urls[mode];
  if (!url) return;
  el.dataset.loaded = "true";
  localStorage.setItem("rail-browse-mode", mode);

  // update active button
  document.querySelectorAll(".fp-browse-mode-btn").forEach((btn) => {
    btn.classList.toggle("active", btn.dataset.mode === mode);
  });

  const search = document.getElementById("fp-browse-search");
  if (search) search.value = "";
  htmx
    .ajax("GET", url, {
      target: el,
      swap: "innerHTML",
      headers: { Accept: "text/html" },
    })
    .then(() => {
      if (mode === "dashboards") initDashboardEditButtons(el);
    });
}

function filterBrowseContent(query) {
  const q = query.toLowerCase();
  const el = document.getElementById("fp-browse-content");
  if (!el) return;

  if (q === "") {
    el.querySelectorAll("li").forEach((li) => {
      li.style.display = "";
      // restore collapsed state when clearing filter
      if (li.dataset.wasCollapsed) {
        li.classList.add("fp-tree-collapsed");
        delete li.dataset.wasCollapsed;
      }
    });
    el.querySelectorAll(".media-compact-item").forEach(
      (item) => (item.style.display = ""),
    );
    return;
  }

  const isTree = el.querySelector("a.fp-tree-file") !== null;

  if (isTree) {
    // expand all collapsed dirs so matches inside them are visible
    el.querySelectorAll("li.fp-tree-collapsed").forEach((li) => {
      li.dataset.wasCollapsed = "1";
      li.classList.remove("fp-tree-collapsed");
    });

    // hide/show file rows
    el.querySelectorAll("li").forEach((li) => {
      const fileLink =
        li.querySelector(":scope > a.fp-tree-file") ||
        li.querySelector(":scope > span.browse-item-row > a.fp-tree-file");
      if (!fileLink) return;
      const text = fileLink.textContent.toLowerCase();
      const href = (fileLink.getAttribute("href") || "").toLowerCase();
      li.style.display = text.includes(q) || href.includes(q) ? "" : "none";
    });

    // hide/show dir rows based on whether any child file matches
    el.querySelectorAll("li").forEach((li) => {
      const hasDirBtn =
        li.querySelector(":scope > button.fp-tree-dir") ||
        li.querySelector(":scope > span.browse-item-row > button.fp-tree-dir");
      if (!hasDirBtn) return;
      const hasVisible = [...li.querySelectorAll("a.fp-tree-file")].some(
        (a) => a.closest("li").style.display !== "none",
      );
      li.style.display = hasVisible ? "" : "none";
    });
  } else if (el.querySelector("li")) {
    // flat list (overview, tags, folders, collections)
    el.querySelectorAll("li").forEach((li) => {
      const link = li.querySelector("a");
      if (!link) return;
      const text = link.textContent.toLowerCase();
      const href = (link.getAttribute("href") || "").toLowerCase();
      li.style.display = text.includes(q) || href.includes(q) ? "" : "none";
    });
  } else {
    // media compact items (no li present)
    el.querySelectorAll(".media-compact-item").forEach((item) => {
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

function closePanel() {
  const flyout = document.getElementById("flyout");
  flyout
    .querySelectorAll(".flyout-panel")
    .forEach((p) => p.classList.remove("active"));
  document
    .querySelectorAll("#rail-site .rail-btn")
    .forEach((b) => b.classList.remove("active"));
  flyout.removeAttribute("data-active");
  document.documentElement.removeAttribute("data-init-panel");
  localStorage.removeItem("rail-panel");
}

// ================================================================
// panel toggle — single shared flyout
// ================================================================
function togglePanel(panelId) {
  const flyout = document.getElementById("flyout");
  const panels = flyout.querySelectorAll(".flyout-panel");
  const target = document.getElementById(panelId);
  const railBtn = document.getElementById("rb-" + panelId.replace("fp-", ""));
  const isOpen = target.classList.contains("active");

  panels.forEach((p) => p.classList.remove("active"));
  document
    .querySelectorAll("#rail-site .rail-btn")
    .forEach((b) => b.classList.remove("active"));

  if (isOpen) {
    flyout.removeAttribute("data-active");
    localStorage.removeItem("rail-panel");
  } else {
    target.classList.add("active");
    flyout.setAttribute("data-active", panelId);
    railBtn?.classList.add("active");
    lazyLoad(panelId);
    localStorage.setItem("rail-panel", panelId);
  }
}

// ================================================================
// lazy load panel content on first open
// ================================================================
function lazyLoad(panelId) {
  const el = document.getElementById(panelId + "-content");
  if (!el || el.dataset.loaded === "true") return;
  const url = el.dataset.url;
  if (!url) return;
  el.dataset.loaded = "true";
  // media panel: use fetch to read X-Hidden-Count header
  if (panelId === "fp-media") {
    fetch(url, { headers: { Accept: "text/html" } })
      .then((r) => {
        const hidden = parseInt(r.headers.get("X-Hidden-Count") || "0", 10);
        updateMediaHiddenWarning(hidden);
        return r.text();
      })
      .then((html) => {
        el.innerHTML = html;
      });
    return;
  }
  htmx.ajax("GET", url, {
    target: el,
    swap: "innerHTML",
    headers: { Accept: "text/html" },
  });
}

// ================================================================
// reload panel — clears cache and re-fetches content
// ================================================================
function reloadPanel(panelId) {
  const el = document.getElementById(panelId + "-content");
  if (!el) return;
  const url = el.dataset.url;
  if (!url) return;
  el.dataset.loaded = "false";
  if (panelId === "fp-media") {
    fetch(url, { headers: { Accept: "text/html" } })
      .then((r) => {
        const hidden = parseInt(r.headers.get("X-Hidden-Count") || "0", 10);
        updateMediaHiddenWarning(hidden);
        return r.text();
      })
      .then((html) => {
        el.innerHTML = html;
        el.dataset.loaded = "true";
      });
    return;
  }
  htmx.ajax("GET", url, {
    target: el,
    swap: "innerHTML",
    headers: { Accept: "text/html" },
  });
  el.dataset.loaded = "true";
}

// public helper called by refresh buttons in panel headers
function refreshPanel(panelId) {
  reloadPanel(panelId);
}

// ================================================================
// auto-reload panels after mutating API calls
// ================================================================
document.body.addEventListener("htmx:afterRequest", function (e) {
  const method = (e.detail.requestConfig?.verb || "").toUpperCase();
  if (method === "GET") return;
  if (!e.detail.successful) return;
  const url = e.detail.requestConfig?.path || "";

  if (/^\/api\/(files|editor|metadata)\//.test(url)) {
    reloadPanel("fp-browse");
    reloadPanel("fp-latest");
  }
  if (/^\/api\/media\//.test(url)) {
    reloadPanel("fp-media");
  }
});

// ================================================================
// search options toggles (title-only + search history)
// ================================================================
function updateSearchMode() {
  const input = document.getElementById("fp-search-input");
  if (!input) return;
  const titleOnly = document.getElementById("fp-search-title-only")?.checked;
  const history = document.getElementById("fp-search-history")?.checked;
  let url = "/api/search?format=list";
  if (titleOnly) url += "&titleonly=true";
  if (history) url += "&history=true";
  input.setAttribute("hx-get", url);
  htmx.process(input);
}

// ================================================================
// file sub-panel switching
// ================================================================
function switchFileSubPanel(view) {
  document
    .querySelectorAll(".fp-sub-panel")
    .forEach((p) => p.classList.remove("active"));
  const target = document.getElementById("fps-" + view);
  if (target) target.classList.add("active");
  // sync mode button active state
  document.querySelectorAll(".fp-file-mode-btn").forEach((btn) => {
    btn.classList.toggle("active", btn.dataset.mode === view);
  });
  localStorage.setItem("rail-file-subpanel", view);
  // clear toc filter when leaving toc panel
  if (view !== "toc") {
    const tocFilter = document.getElementById("fp-toc-filter");
    if (tocFilter) {
      tocFilter.value = "";
      filterTocItems("");
    }
  }
}

// ================================================================
// file page setup
// ================================================================
function setupFilePage() {
  const path = window.location.pathname;

  // dashboard modals
  const dashMatch = path.match(/^\/dashboard\/([^/]+)/);
  if (dashMatch && !path.includes("/edit/") && !path.includes("/new")) {
    const id = dashMatch[1];
    document
      .getElementById("rename-form")
      ?.setAttribute("hx-post", "/api/dashboards/" + id + "/rename");
    document
      .getElementById("delete-form")
      ?.setAttribute("hx-delete", "/api/dashboards/" + id);
    htmx.process(document.getElementById("rename-form"));
    htmx.process(document.getElementById("delete-form"));
  }

  // filter modals
  const filterMatch = path.match(/^\/filters\/(?!new$|edit\/)(.+)/);
  if (filterMatch) {
    document
      .getElementById("delete-form")
      ?.setAttribute("hx-delete", "/api/filters/" + filterMatch[1]);
    htmx.process(document.getElementById("delete-form"));
  }

  // search page — close panel
  if (path === "/search") {
    closePanel();
    return true;
  }

  // edit pages — show file panel with metadata
  const editMatch = path.match(/^\/files\/edit\/(.+)/);
  if (editMatch) {
    const filepath = editMatch[1].split("?")[0];
    const fp = encodeURIComponent(filepath);
    document.body.setAttribute("data-has-file", "true");
    const noFile = document.getElementById("fp-no-file");
    if (noFile) noFile.style.display = "none";
    const pathEl = document.getElementById("fp-meta-path");
    if (pathEl)
      pathEl.innerHTML = '<a href="/files/' + fp + '">' + filepath + "</a>";
    const refFp = document.getElementById("fp-reference-filepath");
    if (refFp) refFp.value = filepath;
    const editFields = {
      "fp-meta-created": "/api/metadata/createdat?filepath=" + fp,
      "fp-meta-edited": "/api/metadata/lastedited?filepath=" + fp,
      "fp-meta-collection": "/api/metadata/collection?filepath=" + fp,
      "fp-meta-folders": "/api/metadata/folders?filepath=" + fp,
    };
    for (const [id, url] of Object.entries(editFields)) {
      const el = document.getElementById(id);
      if (!el) continue;
      fetch(url, { headers: { Accept: "text/html" } })
        .then((r) => r.text())
        .then((html) => {
          el.innerHTML = html;
        })
        .catch(() => {});
    }
    // inline-edit fields: swap outerHTML so edit button HTMX works
    const editInlineFields = {
      "fp-meta-tags": "tags",
      "fp-meta-editor": "editor",
      "fp-meta-path": "path",
    };
    for (const [id, field] of Object.entries(editInlineFields)) {
      const el = document.getElementById(id);
      if (!el) continue;
      fetch("/api/metadata/inline-display?field=" + field + "&filepath=" + fp, {
        headers: { Accept: "text/html" },
      })
        .then((r) => r.text())
        .then((html) => {
          const tmp = document.createElement("div");
          tmp.innerHTML = html;
          const newEl = tmp.firstElementChild;
          if (newEl) {
            el.replaceWith(newEl);
            htmx.process(newEl);
          }
        })
        .catch(() => {});
    }
    htmx.ajax("GET", "/api/metadata/references?filepath=" + fp, {
      target: document.getElementById("component-references-list"),
      swap: "outerHTML",
      headers: { Accept: "text/html" },
    });
    closePanel();
    return true;
  }

  // file pages
  const fileMatch = path.match(/^\/files\/(?!edit\/|new\/|history\/)(.+)/);
  if (!fileMatch) return false;

  const filepath = fileMatch[1];
  const fp = filepath; // already percent-encoded from window.location.pathname

  // reveal file rail button
  document.body.setAttribute("data-has-file", "true");

  // populate filename header
  const titleEl = document.getElementById("fp-file-title");
  if (titleEl) titleEl.textContent = decodeURIComponent(filepath);

  // wire action buttons
  const editLink = document.getElementById("fp-edit-link");
  if (editLink) editLink.href = "/files/edit/" + filepath;

  const rebuildBtn = document.getElementById("fp-rebuild-btn");
  if (rebuildBtn) {
    rebuildBtn.setAttribute("hx-post", "/api/metadata/rebuild/" + filepath);
    rebuildBtn.setAttribute("hx-target", "#fp-rebuild-result");
    htmx.process(rebuildBtn);
  }

  const renameForm = document.getElementById("rename-form");
  const renameInput = document.getElementById("rename-input");
  if (renameForm) {
    renameForm.setAttribute("hx-post", "/api/files/rename/" + filepath);
    htmx.process(renameForm);
  }
  if (renameInput) renameInput.value = filepath;

  const deleteForm = document.getElementById("delete-form");
  if (deleteForm) {
    deleteForm.setAttribute("hx-delete", "/api/files/delete/" + filepath);
    htmx.process(deleteForm);
  }

  const refFp = document.getElementById("fp-reference-filepath");
  if (refFp) refFp.value = filepath;
  htmx.ajax("GET", "/api/metadata/references?filepath=" + fp, {
    target: document.getElementById("component-references-list"),
    swap: "outerHTML",
    headers: { Accept: "text/html" },
  });

  // hide no-file message and show metadata rows
  const noFile = document.getElementById("fp-no-file");
  if (noFile) noFile.style.display = "none";
  const htmxFields = {
    "fp-meta-created": "/api/metadata/createdat?filepath=" + fp,
    "fp-meta-edited": "/api/metadata/lastedited?filepath=" + fp,
    "fp-meta-collection": "/api/metadata/collection?filepath=" + fp,
    "fp-meta-folders": "/api/metadata/folders?filepath=" + fp,
    "fp-ancestors": "/api/links/ancestors?filepath=" + fp,
    "fp-children": "/api/links/kids?filepath=" + fp,
    "fp-grandchildren": "/api/links/grandchildren?filepath=" + fp,
    "fp-links-to": "/api/links/used?filepath=" + fp,
    "fp-media-links": "/api/links/media?filepath=" + fp,
    "fp-links-from": "/api/links/linkstohere?filepath=" + fp,
    "fp-related": "/api/links/related?filepath=" + fp,
  };

  for (const [id, url] of Object.entries(htmxFields)) {
    const el = document.getElementById(id);
    if (!el) continue;
    fetch(url, { headers: { Accept: "text/html" } })
      .then((r) => r.text())
      .then((html) => {
        el.innerHTML = html;
      })
      .catch(() => {});
  }

  // inline-edit fields: swap outerHTML so edit button HTMX works
  const inlineFields = {
    "fp-meta-tags": "tags",
    "fp-meta-editor": "editor",
    "fp-meta-path": "path",
    "fp-parents": "parents",
  };
  for (const [id, field] of Object.entries(inlineFields)) {
    const el = document.getElementById(id);
    if (!el) continue;
    fetch("/api/metadata/inline-display?field=" + field + "&filepath=" + fp, {
      headers: { Accept: "text/html" },
    })
      .then((r) => r.text())
      .then((html) => {
        const tmp = document.createElement("div");
        tmp.innerHTML = html;
        const newEl = tmp.firstElementChild;
        if (newEl) {
          el.replaceWith(newEl);
          htmx.process(newEl);
        }
      })
      .catch(() => {});
  }

  htmx.ajax("GET", "/api/files/versions/" + fp + "?output=full", {
    target: document.getElementById("fp-versions"),
    swap: "innerHTML",
    headers: { Accept: "text/html" },
  });
  // auto-open file info panel
  togglePanel("fp-file");
  return true;
}

// ================================================================
// chat scroll — scroll to top on load and new messages (newest on top)
// ================================================================
function scrollChatToTop() {
  var h = document.getElementById("component-chat-history");
  if (h) h.scrollTop = 0;
}

document.addEventListener("htmx:afterSwap", function (e) {
  var target = e.detail.target;
  if (!target) return;
  if (
    target.id === "component-chat-history" ||
    target.id === "fp-chat-content"
  ) {
    scrollChatToTop();
  }
});

// ================================================================
// flyout drag resize
// ================================================================
function initFlyoutResize() {
  const MIN_WIDTH = 180;
  const STORAGE_KEY = "rail-flyout-width";
  const flyout = document.getElementById("flyout");
  const resizer = document.getElementById("flyout-resizer");
  if (!flyout || !resizer) return;

  // restore saved width
  const saved = localStorage.getItem(STORAGE_KEY);
  if (saved) flyout.style.setProperty("--fw", saved + "px");

  resizer.addEventListener("mousedown", (e) => {
    const startX = e.clientX;
    const startWidth = flyout.offsetWidth;
    resizer.classList.add("dragging");
    document.body.style.cursor = "col-resize";
    document.body.style.userSelect = "none";
    // disable transition while dragging for instant feedback
    flyout.style.transition = "none";

    function onMove(e) {
      const w = Math.max(MIN_WIDTH, startWidth + (e.clientX - startX));
      flyout.style.setProperty("--fw", w + "px");
    }

    function onUp() {
      resizer.classList.remove("dragging");
      document.body.style.cursor = "";
      document.body.style.userSelect = "";
      flyout.style.transition = "";
      localStorage.setItem(STORAGE_KEY, flyout.offsetWidth);
      document.removeEventListener("mousemove", onMove);
      document.removeEventListener("mouseup", onUp);
    }

    document.addEventListener("mousemove", onMove);
    document.addEventListener("mouseup", onUp);
    e.preventDefault();
  });
}

// ================================================================
// filter panel — persist state across page loads
// ================================================================
function saveFpFilterState() {
  const criteria = document.getElementById("fp-filter-criteria");
  const cb = document.getElementById("fp-logic-checkbox");
  if (!criteria) return;
  const logicVal = cb?.checked ? "or" : "and";
  const hidden = document.getElementById("fp-logic-value");
  if (hidden) hidden.value = logicVal;
  const label = document.querySelector(".fp-logic-label");
  if (label)
    label.textContent = cb?.checked
      ? label.dataset.on || "or"
      : label.dataset.off || "and";

  // stamp text/date input values into attributes so innerHTML captures them
  criteria
    .querySelectorAll('input[type="text"], input[type="date"]')
    .forEach((inp) => {
      inp.setAttribute("value", inp.value);
    });

  // save select name→value explicitly
  const selectValues = {};
  criteria.querySelectorAll("select").forEach((sel) => {
    if (sel.name) selectValues[sel.name] = sel.value;
  });

  localStorage.setItem(
    "fp-filter-state",
    JSON.stringify({
      criteria: criteria.innerHTML,
      logic: logicVal,
      selectValues,
    }),
  );
}

function restoreFpFilterState() {
  const raw = localStorage.getItem("fp-filter-state");
  const criteria = document.getElementById("fp-filter-criteria");
  if (!criteria) return;

  if (raw) {
    try {
      const state = JSON.parse(raw);
      const cb = document.getElementById("fp-logic-checkbox");
      const hidden = document.getElementById("fp-logic-value");
      const label = document.querySelector(".fp-logic-label");
      if (state.criteria) {
        criteria.innerHTML = state.criteria;
        // use option.selected (not sel.value) to set the dirty flag,
        // so later DOM manipulation of defaultSelected can't reset the value
        if (state.selectValues) {
          criteria.querySelectorAll("select").forEach((sel) => {
            const saved = state.selectValues[sel.name];
            if (saved !== undefined) {
              Array.from(sel.options).forEach((opt) => {
                opt.selected = opt.value === saved;
              });
            }
          });
        }
        // defer htmx.process so select values are set before hx-get fires
        setTimeout(() => htmx.process(criteria), 0);
      }
      if (state.logic === "or") {
        if (cb) cb.checked = true;
        if (hidden) hidden.value = "or";
        if (label) label.textContent = label.dataset.on || "or";
      }
      return;
    } catch (_) {}
  }

  // nothing saved — load one default criteria row with unique index
  const idx = Date.now() % 1000000;
  criteria.addEventListener("htmx:afterSwap", function saveOnce() {
    criteria.removeEventListener("htmx:afterSwap", saveOnce);
    saveFpFilterState();
  });
  htmx.ajax("GET", `/api/filters/criteria-row?row_index=${idx}`, {
    target: criteria,
    swap: "innerHTML",
    headers: { Accept: "text/html" },
  });
}

// save after criteria row removal
document.addEventListener("click", function (e) {
  const btn = e.target.closest(".filter-criteria-row .btn-danger");
  if (!btn) return;
  const container = document.getElementById("fp-filter-criteria");
  if (!container) return;
  setTimeout(saveFpFilterState, 0);
});

// save when user changes a select or input inside criteria rows
document.addEventListener("change", function (e) {
  const container = document.getElementById("fp-filter-criteria");
  if (!container || !container.contains(e.target)) return;
  saveFpFilterState();
});
document.addEventListener("input", function (e) {
  const container = document.getElementById("fp-filter-criteria");
  if (!container || !container.contains(e.target)) return;
  saveFpFilterState();
});

function initDashboardEditButtons(container) {
  // if called with a container, decorate it immediately (e.g. from switchBrowseMode)
  if (container) {
    decorateDashboardLinks(container);
    return;
  }
  // listen for any htmx swap inside the flyout and decorate when appropriate
  document
    .getElementById("flyout")
    ?.addEventListener("htmx:afterSwap", function (e) {
      const id = e.detail.target.id;
      if (id === "fp-dashboards-content") {
        // legacy standalone panel (kept for safety)
        decorateDashboardLinks(e.detail.target);
      } else if (id === "fp-browse-content") {
        // browse panel — only decorate when dashboards mode is active
        const mode = document.querySelector(".fp-browse-mode-btn.active")
          ?.dataset.mode;
        if (mode === "dashboards") decorateDashboardLinks(e.detail.target);
      }
    });
}

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

function initBrowseInterceptor() {
  const flyout = document.getElementById("flyout");
  if (!flyout) return;

  flyout.addEventListener("click", function (e) {
    const link = e.target.closest('a[href*="/browse/"]');
    if (!link) return;

    const content = document.getElementById("fp-browse-content");
    if (!content || !content.contains(link)) return;

    const match = (link.getAttribute("href") || "").match(
      /\/browse\/([^/]+)\/(.+)/,
    );
    if (!match) return;

    e.preventDefault();
    const metaType = match[1];
    const value = decodeURIComponent(match[2]);
    const url = `/api/files/browse?metadata=${metaType}&value=${encodeURIComponent(value)}&actions=true`;

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
      const activeBtn = document.querySelector(".fp-browse-mode-btn.active");
      btn.textContent = "← " + (activeBtn?.title || "back");
      btn.onclick = () => switchBrowseMode(activeBtn?.dataset.mode || "tree");
      content.insertBefore(btn, content.firstChild);
    });

    const search = document.getElementById("fp-browse-search");
    if (search) search.value = "";
  });
}

// ================================================================
// tree inline rename
// ================================================================
function initTreeRename() {
  const browse = document.getElementById("fp-browse-content");
  if (!browse) return;

  browse.addEventListener("click", (e) => {
    const renameBtn = e.target.closest(".browse-rename-btn");
    if (!renameBtn) return;
    e.preventDefault();
    e.stopPropagation();

    const path = renameBtn.dataset.path;
    const type = renameBtn.dataset.type;
    const currentName = path.split("/").pop();
    const parentDir = path.includes("/") ? path.slice(0, path.lastIndexOf("/")) : "";

    const row = renameBtn.closest(".browse-item-row");
    const labelEl = type === "folder"
      ? row?.querySelector("button.fp-tree-dir")
      : row?.querySelector("a.fp-tree-file");
    if (!labelEl) return;

    const input = document.createElement("input");
    input.type = "text";
    input.value = currentName;
    input.className = "fp-tree-rename-input";
    labelEl.style.display = "none";
    renameBtn.style.display = "none";
    if (row.draggable) row.draggable = false;
    labelEl.parentNode.insertBefore(input, labelEl);
    input.focus();
    input.select();

    let committed = false;

    function cancel() {
      input.remove();
      labelEl.style.display = "";
      renameBtn.style.display = "";
      if (row) row.draggable = true;
    }

    function commit() {
      if (committed) return;
      const newName = input.value.trim();
      if (!newName || newName === currentName) { cancel(); return; }
      committed = true;

      const encodedPath = path.split("/").map(encodeURIComponent).join("/");
      let url, body;
      if (type === "file") {
        const newPath = parentDir ? parentDir + "/" + newName : newName;
        url = "/api/files/rename/" + encodedPath;
        body = new URLSearchParams({ name: newPath });
      } else {
        url = "/api/files/move-folder/" + encodedPath;
        body = new URLSearchParams({ target: parentDir || ".", name: newName });
      }

      fetch(url, { method: "POST", body }).then((res) => {
        if (res.ok) {
          const redirect = res.headers.get("HX-Redirect");
          const curPath = window.location.pathname;
          if (type === "file" && redirect && curPath.includes(path)) {
            window.location.href = redirect;
          } else {
            reloadPanel("fp-browse");
          }
        } else {
          committed = false;
          res.text().then((html) => {
            const tmp = document.createElement("div");
            tmp.innerHTML = html;
            alert(tmp.textContent.trim());
            cancel();
          });
        }
      });
    }

    input.addEventListener("keydown", (e) => {
      if (e.key === "Enter") { e.preventDefault(); commit(); }
      if (e.key === "Escape") cancel();
    });
    input.addEventListener("blur", commit);
  });
}

// ================================================================
// tree drag and drop — move files into folders
// ================================================================
const TREE_DND_TYPE = "application/x-knov-filepath";

function initTreeDragDrop() {
  const browse = document.getElementById("fp-browse-content");
  if (!browse) return;

  browse.addEventListener("dragstart", (e) => {
    const el = e.target.closest("[data-path][draggable]");
    if (!el) return;
    e.dataTransfer.setData(TREE_DND_TYPE, JSON.stringify({ path: el.dataset.path, type: el.dataset.type }));
    e.dataTransfer.effectAllowed = "move";
  });

  browse.addEventListener("dragend", () => {
    browse
      .querySelectorAll(".fp-tree-dir.drag-over")
      .forEach((b) => b.classList.remove("drag-over"));
  });

  browse.addEventListener("dragover", (e) => {
    if (!e.dataTransfer.types.includes(TREE_DND_TYPE)) return;
    const btn = e.target.closest("button.fp-tree-dir");
    if (!btn) return;
    e.preventDefault();
    e.dataTransfer.dropEffect = "move";
  });

  browse.addEventListener("dragenter", (e) => {
    if (!e.dataTransfer.types.includes(TREE_DND_TYPE)) return;
    const btn = e.target.closest("button.fp-tree-dir");
    if (!btn) return;
    browse
      .querySelectorAll(".fp-tree-dir.drag-over")
      .forEach((b) => b.classList.remove("drag-over"));
    btn.classList.add("drag-over");
  });

  browse.addEventListener("dragleave", (e) => {
    const btn = e.target.closest("button.fp-tree-dir");
    if (!btn) return;
    if (!btn.contains(e.relatedTarget)) btn.classList.remove("drag-over");
  });

  browse.addEventListener("drop", (e) => {
    const btn = e.target.closest("button.fp-tree-dir");
    if (!btn) return;
    const payload = e.dataTransfer.getData(TREE_DND_TYPE);
    if (!payload) return;
    e.preventDefault();
    btn.classList.remove("drag-over");

    const { path: srcPath, type } = JSON.parse(payload);
    const targetDir = btn.dataset.path;
    const name = srcPath.split("/").pop();
    const newPath = targetDir + "/" + name;

    if (newPath === srcPath) return;
    // prevent folder drop into its own subtree
    if (type === "folder" && (newPath + "/").startsWith(srcPath + "/")) return;

    const encodedSrc = srcPath.split("/").map(encodeURIComponent).join("/");

    if (type === "folder") {
      fetch("/api/files/move-folder/" + encodedSrc, {
        method: "POST",
        body: new URLSearchParams({ target: targetDir }),
      }).then((res) => {
        if (res.ok) {
          reloadPanel("fp-browse");
        } else {
          res.text().then((html) => {
            const tmp = document.createElement("div");
            tmp.innerHTML = html;
            alert(tmp.textContent.trim());
          });
        }
      });
    } else {
      fetch("/api/files/rename/" + encodedSrc, {
        method: "POST",
        body: new URLSearchParams({ name: newPath }),
      }).then((res) => {
        if (res.ok) {
          const redirect = res.headers.get("HX-Redirect");
          const curPath = window.location.pathname;
          if (redirect && curPath.startsWith("/files/") && curPath.includes(srcPath)) {
            window.location.href = redirect;
          } else {
            reloadPanel("fp-browse");
          }
        } else {
          res.text().then((html) => {
            const tmp = document.createElement("div");
            tmp.innerHTML = html;
            alert(tmp.textContent.trim());
          });
        }
      });
    }
  });
}

document.addEventListener("DOMContentLoaded", () => {
  initFlyoutResize();
  initTreeRename();
  initTreeDragDrop();
  initDashboardEditButtons();
  initBrowseInterceptor();
  restoreFpFilterState();

  // restore saved browse mode — update active button + data-url before first lazyLoad
  const savedMode = localStorage.getItem("rail-browse-mode");
  if (savedMode) {
    document.querySelectorAll(".fp-browse-mode-btn").forEach((btn) => {
      btn.classList.toggle("active", btn.dataset.mode === savedMode);
    });
    const urls = {
      tree: "/api/files/tree?actions=true",
      browse: "/api/files/folder?path=&target=%23fp-browse-content",
      overview: "/api/files/list?actions=true",
      tags: "/api/metadata/tags?actions=true",
      folders: "/api/metadata/folders?actions=true",
      collections: "/api/metadata/collections?actions=true",
      dashboards: "/api/dashboards",
      editor: "/api/metadata/editors",
      filters: "/api/files/browse?metadata=editor&value=filter-editor",
      notifications: "/api/notifications",
    };
    const el = document.getElementById("fp-browse-content");
    if (el && urls[savedMode]) el.dataset.url = urls[savedMode];
  }

  const tocData = document.getElementById("fp-toc-data");
  const tocNav = document.getElementById("fp-toc-nav");
  if (tocData && tocNav) tocNav.innerHTML = tocData.innerHTML;

  const isSystemPage = !document.getElementById("fp-file-modes");
  if (isSystemPage) {
    document.body.setAttribute("data-has-file", "true");
    togglePanel("fp-file");
  }

  const isFilePage = setupFilePage();
  const isHistoryPage =
    window.location.pathname.startsWith("/files/history/") ||
    window.location.pathname === "/history";
  if (!isFilePage && !isHistoryPage && !isSystemPage) {
    const saved = localStorage.getItem("rail-panel");
    if (saved && document.getElementById(saved)) togglePanel(saved);
  }

  // restore saved file sub-panel selection (skip on system pages — TOC is set in HTML)
  const savedSubPanel = localStorage.getItem("rail-file-subpanel");
  if (savedSubPanel && !isSystemPage) {
    switchFileSubPanel(savedSubPanel);
  }

  // re-enable transitions after first state restore so user interactions animate
  requestAnimationFrame(() => {
    document.documentElement.classList.remove("no-transition");
  });
});

// ================================================================
// toc filter — client-side, no API call
// ================================================================
function filterTocItems(query) {
  const nav = document.getElementById("fp-toc-nav");
  if (!nav) return;
  const q = query.toLowerCase().trim();
  nav.querySelectorAll("a").forEach((a) => {
    const text = a.textContent.toLowerCase();
    a.style.display = q === "" || text.includes(q) ? "" : "none";
  });
}

// ================================================================
// history versions filter — client-side, no API call
// ================================================================
function filterFpVersions(query) {
  const container = document.getElementById("fp-versions");
  if (!container) return;
  const q = query.toLowerCase().trim();
  container.querySelectorAll(".version-item").forEach((li) => {
    li.style.display =
      q === "" || li.textContent.toLowerCase().includes(q) ? "" : "none";
  });
}

// ================================================================
// latest changes filter — client-side, no API call
// ================================================================
let _latestSearchTimer = null;
function filterLatestChanges(query) {
  const container = document.getElementById("fp-latest-content");
  if (!container) return;
  clearTimeout(_latestSearchTimer);
  _latestSearchTimer = setTimeout(() => {
    const q = query.trim();
    const base = container.dataset.url || "/api/git/latestchanges?count=50";
    const url = q
      ? `/api/git/latestchanges?count=50&q=${encodeURIComponent(q)}`
      : base;
    htmx.ajax("GET", url, {
      target: container,
      swap: "innerHTML",
      headers: { Accept: "text/html" },
    });
  }, 300);
}
