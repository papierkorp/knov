// theme: builtin-reworked — core rail/flyout plumbing: an alpine store owns
// panel open/close/toggle state + persistence + the parsed rail groups, and
// this file keeps the lazy-load/reload helpers every content snippet relies
// on.
//
// Every rail button and flyout panel (rail-render.js's panelGroup component,
// base.gohtml's static fp-file) binds straight to `$store.rail` via
// `:class`/`:hidden`/`:data-active`/`@click`, so the store only needs to hold
// state — no DOM syncing of its own.
//
// `togglePanel`/`closePanel`/`reloadPanel`/`lazyLoad` stay as plain global
// functions — every rail-*.js snippet file and every gohtml onclick="..."
// attribute still calls these by name, they just delegate into the store
// now instead of duplicating its state. Group panel refresh buttons call
// their own panelGroup component's `refresh()` method directly instead
// (rail-render.js) — it already knows its active tab, no DOM query needed.
//
// This script is NOT deferred, so it runs eagerly wherever it sits in the
// page and registers its `alpine:init` listener well before alpine.js's own
// <script defer> tag (loaded last in base.gohtml) executes and fires that
// event.

const RAIL_PANEL_KEY = "rail-panel";
const RAIL_SUBPANEL_KEY = "rail-file-subpanel";

document.addEventListener("alpine:init", () => {
  Alpine.store("rail", {
    active: null,
    groups: [],

    init() {
      this.active = document.documentElement.getAttribute("data-init-panel") || null;
      this.groups = railParseLayout();
    },

    isActive(panelId) {
      return this.active === panelId;
    },

    open(panelId) {
      this.active = panelId;
      localStorage.setItem(RAIL_PANEL_KEY, panelId);
      lazyLoadActivePanel(panelId);
    },

    close() {
      this.active = null;
      document.documentElement.removeAttribute("data-init-panel");
      localStorage.removeItem(RAIL_PANEL_KEY);
    },

    toggle(panelId) {
      if (this.active === panelId) this.close();
      else this.open(panelId);
    },
  });
});

function togglePanel(panelId) {
  Alpine.store("rail").toggle(panelId);
}
function closePanel() {
  Alpine.store("rail").close();
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
  htmx.ajax("GET", url, {
    target: el,
    swap: "innerHTML",
    headers: { Accept: "text/html" },
  });
}

// a group panel (fp-<groupID>) has no content div of its own anymore — each
// of its snippets does, wrapped in its own ".rail-tab-panel". Load whichever
// one is currently visible. Panels with no tab-panel at all (fp-file, which
// populates itself via setupFilePage) fall through to a direct lazyLoad,
// which safely no-ops since they have no "-content" data-url element either.
function lazyLoadActivePanel(panelId) {
  const panelEl = document.getElementById(panelId);
  const active = panelEl?.querySelector(".rail-tab-panel:not([hidden])") || panelEl?.querySelector(".rail-tab-panel");
  lazyLoad(active ? active.id : panelId);
}

// ================================================================
// reload panel — clears cache and re-fetches content
// ================================================================
function reloadPanel(panelId) {
  const el = document.getElementById(panelId + "-content");
  if (!el) return;
  const url = el.dataset.url;
  if (!url) return;
  htmx.ajax("GET", url, {
    target: el,
    swap: "innerHTML",
    headers: { Accept: "text/html" },
  });
  el.dataset.loaded = "true";
}

// ================================================================
// auto-reload panels after mutating API calls — groups are user-composed,
// so instead of hardcoded panel ids this scans every already-loaded content
// div and reloads the ones whose own snippet is relevant.
// ================================================================
function reloadLoadedGroupsMatching(predicate) {
  document
    .querySelectorAll("#flyout .flyout-content[data-loaded='true']")
    .forEach((el) => {
      if (!predicate(el.dataset.snippet)) return;
      reloadPanel(el.id.replace(/-content$/, ""));
    });
}

document.body.addEventListener("htmx:afterRequest", function (e) {
  if (!e.detail.successful) return;
  const method = (e.detail.requestConfig?.verb || "").toUpperCase();
  const url = e.detail.requestConfig?.path || "";

  if (method === "GET") {
    // media list loads carry the hidden-files warning in a response header
    if (url.startsWith("/api/media/list")) {
      const instanceID = e.detail.target?.id?.replace(/^fp-/, "").replace(/-content$/, "");
      if (instanceID && typeof updateMediaHiddenWarning === "function") {
        updateMediaHiddenWarning(instanceID, e.detail.xhr?.getResponseHeader("X-Hidden-Message"));
      }
    }
    return;
  }

  if (/^\/api\/(files|editor|metadata)\//.test(url)) {
    // every content-kind snippet except chat is file/metadata-derived
    reloadLoadedGroupsMatching((snippet) => snippet !== "chat");
  }
  if (/^\/api\/media\//.test(url)) {
    reloadLoadedGroupsMatching((snippet) => snippet === "media");
  }
});

// ================================================================
// flyout drag resize — self-contained, wired from base.gohtml's
// x-init="initFlyoutResize($el)" on #flyout instead of a global
// DOMContentLoaded hook
// ================================================================
function initFlyoutResize(flyout) {
  const MIN_WIDTH = 180;
  const STORAGE_KEY = "rail-flyout-width";
  const resizer = flyout.querySelector("#flyout-resizer");
  if (!resizer) return;

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
// page bootstrap — decides which panel/sub-panel is open on load.
// per-snippet init functions live in their own rail-<name>.js file; those
// that are conditionally loaded (filter/media) are guarded here in case
// that file isn't present. Runs on alpine:initialized, once the store
// above and every alpine component on the page already exist.
// ================================================================
document.addEventListener("alpine:initialized", () => {
  if (typeof initTreeRename === "function") initTreeRename();
  if (typeof initTreeDragDrop === "function") initTreeDragDrop();
  if (typeof initGroupInterceptor === "function") initGroupInterceptor();
  if (typeof restoreAllFpFilterStates === "function") restoreAllFpFilterStates();
  if (typeof restoreTocFromData === "function") restoreTocFromData();

  const isSystemPage = !document.getElementById("fp-file-modes");
  if (isSystemPage) {
    document.body.setAttribute("data-has-file", "true");
    Alpine.store("rail").open("fp-file");
  }

  const isFilePage =
    typeof setupFilePage === "function" ? setupFilePage() : false;
  const isHistoryPage =
    window.location.pathname.startsWith("/files/history/") ||
    window.location.pathname === "/history";
  if (!isFilePage && !isHistoryPage && !isSystemPage) {
    const saved = localStorage.getItem(RAIL_PANEL_KEY);
    if (saved && saved !== "fp-file" && document.getElementById(saved))
      Alpine.store("rail").open(saved);
  }

  // restore saved file sub-panel selection (skip on system pages — TOC is set in HTML)
  const savedSubPanel = localStorage.getItem(RAIL_SUBPANEL_KEY);
  if (savedSubPanel && !isSystemPage) {
    Alpine.store("filePanel").switch(savedSubPanel);
  }

  // re-enable transitions after first state restore so user interactions animate
  requestAnimationFrame(() => {
    document.documentElement.classList.remove("no-transition");
  });
});
