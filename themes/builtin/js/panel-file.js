// theme: builtin — file panel (metadata/toc/history/chat/references/connections/find sub-panels)

// ================================================================
// file sub-panel switching — Alpine.store since fp-file is a page-wide
// singleton (like $store.rail, see rail-core.js) rather than an x-for'd
// component; base.gohtml binds tab buttons/sub-panels straight to
// $store.filePanel.active via :class, so switch() only needs to own the
// side effects (persistence, clearing toc/find state).
// ================================================================
document.addEventListener("alpine:init", () => {
  Alpine.store("filePanel", {
    active: document.getElementById("fp-file")?.dataset.initSubpanel || "metadata",
    hasFile: false,

    switch(view) {
      this.active = view;
      localStorage.setItem("rail-file-subpanel", view);
      // clear toc filter when leaving toc panel
      if (view !== "toc") {
        const tocFilter = document.getElementById("fp-toc-filter");
        if (tocFilter) {
          tocFilter.value = "";
          filterTocItems("");
        }
      }
      // clear find highlights when leaving find panel
      if (view !== "find") {
        Alpine.store("fileFind").reset();
        const findInput = document.getElementById("fp-find-input");
        if (findInput) findInput.value = "";
      } else {
        setTimeout(() => document.getElementById("fp-find-input")?.focus(), 50);
      }
    },
  });
});

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

  // settings/admin pages — no file panel content applies here
  if (path === "/settings" || path === "/admin") {
    closePanel();
    return true;
  }

  // table editor page — no file panel metadata to populate
  if (path.match(/^\/files\/edittable\/(.+)/)) {
    closePanel();
    return true;
  }

  // edit pages — show file panel with metadata
  const editMatch = path.match(/^\/files\/edit\/(.+)/);
  if (editMatch) {
    const filepath = editMatch[1].split("?")[0];
    const fp = encodeURIComponent(filepath);
    document.body.setAttribute("data-has-file", "true");
    Alpine.store("filePanel").hasFile = true;
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
      htmx.ajax("GET", url, {
        target: el,
        swap: "innerHTML",
        headers: { Accept: "text/html" },
      });
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
      htmx.ajax(
        "GET",
        "/api/metadata/inline-display?field=" + field + "&filepath=" + fp,
        { source: el, target: el, swap: "outerHTML", headers: { Accept: "text/html" } },
      );
    }
    htmx.ajax("GET", "/api/metadata/references?filepath=" + fp, {
      source: document.getElementById("component-references-list"),
      target: document.getElementById("component-references-list"),
      swap: "outerHTML",
      headers: { Accept: "text/html" },
    });
    closePanel();
    return true;
  }

  // file pages
  const fileMatch = path.match(
    /^\/files\/(?!edit\/|new\/|history\/|edittable\/)(.+)/,
  );
  if (!fileMatch) return false;

  const filepath = fileMatch[1];
  // filepath is a path segment from window.location.pathname: safe to concatenate
  // into another path segment as-is, but "&"/"+" pass through unescaped there and
  // would corrupt a query string (delimiter / space substitution), so query uses
  // need their own encoding.
  const fp = encodeURIComponent(filepath);

  // reveal file rail button
  document.body.setAttribute("data-has-file", "true");

  // populate filename header
  const titleEl = document.getElementById("fp-file-title");
  if (titleEl) titleEl.textContent = decodeURIComponent(filepath);

  // wire action buttons
  const editLink = document.getElementById("fp-edit-link");
  if (editLink) editLink.href = "/files/edit/" + filepath;

  const exportPdfLink = document.getElementById("fp-export-pdf-link");
  if (exportPdfLink)
    exportPdfLink.href = "/api/files/export/pdf?filepath=" + fp;

  const rebuildBtn = document.getElementById("fp-rebuild-btn");
  if (rebuildBtn) {
    rebuildBtn.setAttribute("hx-post", "/api/metadata/rebuild/" + filepath);
    rebuildBtn.setAttribute("hx-target", "#fp-rebuild-result");
    htmx.process(rebuildBtn);
  }

  // dokuwiki detection - a rendered dokuwiki file still starts with its
  // "====== heading ======" syntax since no dokuwiki parser exists anymore,
  // only the plaintext fallback (which HTML-escapes but doesn't strip it)
  const convertLink = document.getElementById("fp-convert-dokuwiki-link");
  if (convertLink) {
    const article = document.querySelector("article.file-content");
    const isDokuwiki = !!article && article.textContent.trimStart().startsWith("======");
    convertLink.hidden = !isDokuwiki;
    if (isDokuwiki)
      convertLink.href = "/api/files/export/markdown?filepath=" + fp;
  }

  const renameForm = document.getElementById("rename-form");
  const renameInput = document.getElementById("rename-input");
  if (renameForm) {
    renameForm.setAttribute("hx-post", "/api/files/rename/" + filepath);
    htmx.process(renameForm);
  }
  if (renameInput) renameInput.value = filepath;

  const moveForm = document.getElementById("move-form");
  const moveFolderInput = document.getElementById("move-folder-input");
  const lastSlash = filepath.lastIndexOf("/");
  if (moveForm) {
    moveForm.setAttribute("hx-post", "/api/files/rename/" + filepath);
    moveForm.dataset.filename =
      lastSlash === -1 ? filepath : filepath.substring(lastSlash + 1);
    htmx.process(moveForm);
  }
  if (moveFolderInput)
    moveFolderInput.value = lastSlash === -1 ? "" : filepath.substring(0, lastSlash);

  const deleteForm = document.getElementById("delete-form");
  if (deleteForm) {
    deleteForm.setAttribute("hx-delete", "/api/files/delete/" + filepath);
    htmx.process(deleteForm);
  }

  const refFp = document.getElementById("fp-reference-filepath");
  if (refFp) refFp.value = filepath;
  htmx.ajax("GET", "/api/metadata/references?filepath=" + fp, {
    source: document.getElementById("component-references-list"),
    target: document.getElementById("component-references-list"),
    swap: "outerHTML",
    headers: { Accept: "text/html" },
  });

  // hide no-file message and show metadata rows
  Alpine.store("filePanel").hasFile = true;

  // fetch all sidebar metadata/link fragments in a single request instead of
  // firing one fetch per field, which used to queue up behind the browser's
  // per-origin connection limit and made the page look like it was still loading.
  // The API returns semantic field names; map them onto this theme's DOM ids here.
  const sidebarFieldTargets = {
    created: "fp-meta-created",
    edited: "fp-meta-edited",
    collection: "fp-meta-collection",
    folders: "fp-meta-folders",
    ancestors: "fp-ancestors",
    kids: "fp-children",
    grandchildren: "fp-grandchildren",
    usedLinks: "fp-links-to",
    mediaLinks: "fp-media-links",
    linksFrom: "fp-links-from",
    related: "fp-related",
  };
  fetch("/api/files/overview?filepath=" + fp, {
    headers: { Accept: "application/json" },
  })
    .then((r) => r.json())
    .then((fields) => {
      for (const [field, html] of Object.entries(fields)) {
        const el = document.getElementById(sidebarFieldTargets[field]);
        if (el) el.innerHTML = html;
      }
    })
    .catch(() => {});

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
    htmx.ajax(
      "GET",
      "/api/metadata/inline-display?field=" + field + "&filepath=" + fp,
      { source: el, target: el, swap: "outerHTML", headers: { Accept: "text/html" } },
    );
  }

  htmx.ajax("GET", "/api/files/versions/" + fp + "?output=full", {
    source: document.getElementById("fp-versions"),
    target: document.getElementById("fp-versions"),
    swap: "innerHTML",
    headers: { Accept: "text/html" },
  });

  const chatContentEl = document.getElementById("fp-file-chat-content");
  if (chatContentEl) {
    chatContentEl.dataset.url =
      "/api/chat/messages?file=" + fp + "&short=true";
    chatContentEl.dataset.loaded = "true";
    htmx.ajax("GET", chatContentEl.dataset.url, {
      target: chatContentEl,
      swap: "innerHTML",
      headers: { Accept: "text/html" },
    });
  }

  // auto-open file info panel
  togglePanel("fp-file");
  return true;
}

// ================================================================
// file panel overflow menu (rename / move / rebuild / export / delete) —
// toggle + viewport-aware positioning + close-on-outside-click/scroll, all
// declarative on the .fp-menu-wrap element in base.gohtml (@click.outside,
// @scroll.window.capture, :hidden="!open"); this component just owns
// `open` and the position() math.
// ================================================================
document.addEventListener("alpine:init", () => {
  Alpine.data("fpFileMenu", () => ({
    open: false,

    toggle() {
      this.open = !this.open;
      if (this.open) this.$nextTick(() => this.position());
    },

    close() {
      this.open = false;
    },

    position() {
      const rect = this.$refs.btn.getBoundingClientRect();
      const menu = this.$refs.menu;
      menu.style.left = "auto";
      menu.style.right = window.innerWidth - rect.right + "px";
      menu.style.top = rect.bottom + 2 + "px";
      menu.style.bottom = "auto";

      const menuRect = menu.getBoundingClientRect();
      if (menuRect.bottom > window.innerHeight) {
        menu.style.top = "auto";
        menu.style.bottom = window.innerHeight - rect.top + 2 + "px";
      }
    },
  }));
});

// builds the new full path for the move modal from the folder input plus
// the filename stashed on the form when the file panel opened
function buildMoveTargetName() {
  const folderEl = document.getElementById("move-folder-input");
  const form = document.getElementById("move-form");
  if (!folderEl || !form) return "";
  const folder = folderEl.value.replace(/\/+$/, "");
  const filename = form.dataset.filename || "";
  return folder ? folder + "/" + filename : filename;
}

// restore the server-rendered TOC data block into the nav + set up folding
// (system pages render their TOC directly, so this is a no-op there)
function restoreTocFromData() {
  const tocData = document.getElementById("fp-toc-data");
  const tocNav = document.getElementById("fp-toc-nav");
  if (!tocData || !tocNav) return;
  tocNav.innerHTML = tocData.innerHTML;
  setupTocFolding();
}

// ================================================================
// toc filter — client-side, no API call
// matches stay in the DOM (just hidden) and auto-unfold their
// collapsed ancestors so hits are never hidden by a fold
// ================================================================
function filterTocItems(query) {
  const nav = document.getElementById("fp-toc-nav");
  if (!nav) return;
  const items = Array.from(nav.querySelectorAll("a[data-level]"));
  const q = query.toLowerCase().trim();

  if (q === "") {
    items.forEach((a) => {
      a.style.display = "";
    });
    updateTocFoldVisibility();
    return;
  }

  const showSet = new Set();
  items.forEach((a, i) => {
    if (!a.textContent.toLowerCase().includes(q)) return;
    showSet.add(i);
    // reveal the ancestor chain, unfolding any that are collapsed
    let level = parseInt(a.dataset.level, 10);
    for (let j = i - 1; j >= 0 && level > 0; j--) {
      const jLevel = parseInt(items[j].dataset.level, 10);
      if (jLevel < level) {
        showSet.add(j);
        level = jLevel;
        if (items[j].classList.contains("fp-toc-collapsed")) {
          items[j].classList.remove("fp-toc-collapsed");
          const toggle = items[j].querySelector(".fp-toc-fold-toggle");
          if (toggle) toggle.textContent = "▾";
        }
      }
    }
  });

  updateTocFoldVisibility();
  items.forEach((a, i) => {
    a.style.display = showSet.has(i) ? "" : "none";
  });
}

// ================================================================
// toc folding — collapse/expand headings by nesting level
// ================================================================
function setupTocFolding() {
  const nav = document.getElementById("fp-toc-nav");
  if (!nav) return;
  const items = Array.from(nav.querySelectorAll("a[data-level]"));
  items.forEach((a, i) => {
    if (a.querySelector(".fp-toc-fold-gutter")) return;
    const level = parseInt(a.dataset.level, 10);
    const next = items[i + 1];
    const hasChildren = !!next && parseInt(next.dataset.level, 10) > level;
    // every item gets a fixed-width gutter so indentation lines up
    // whether or not it has a toggle glyph in it
    const gutter = document.createElement("span");
    gutter.className = "fp-toc-fold-gutter";
    if (hasChildren) {
      a.classList.add("fp-toc-has-children");
      const toggle = document.createElement("span");
      toggle.className = "fp-toc-fold-toggle";
      toggle.textContent = "▾";
      toggle.addEventListener("click", (e) => {
        e.preventDefault();
        e.stopPropagation();
        toggleTocFold(a);
      });
      gutter.appendChild(toggle);
    }
    a.insertBefore(gutter, a.firstChild);
  });
  updateTocFoldVisibility();
}

function toggleTocFold(a) {
  const collapsed = a.classList.toggle("fp-toc-collapsed");
  const toggle = a.querySelector(".fp-toc-fold-toggle");
  if (toggle) toggle.textContent = collapsed ? "▸" : "▾";
  updateTocFoldVisibility();
}

function setAllTocFolds(collapse) {
  const nav = document.getElementById("fp-toc-nav");
  if (!nav) return;
  nav.querySelectorAll("a.fp-toc-has-children").forEach((a) => {
    a.classList.toggle("fp-toc-collapsed", collapse);
    const toggle = a.querySelector(".fp-toc-fold-toggle");
    if (toggle) toggle.textContent = collapse ? "▸" : "▾";
  });
  updateTocFoldVisibility();
}

// hides an item if any ancestor heading above it in the outline is collapsed
function updateTocFoldVisibility() {
  const nav = document.getElementById("fp-toc-nav");
  if (!nav) return;
  const items = Array.from(nav.querySelectorAll("a[data-level]"));
  const collapsedLevels = [];
  items.forEach((a) => {
    const level = parseInt(a.dataset.level, 10);
    while (
      collapsedLevels.length &&
      collapsedLevels[collapsedLevels.length - 1] >= level
    ) {
      collapsedLevels.pop();
    }
    a.classList.toggle("fp-toc-hidden", collapsedLevels.length > 0);
    if (a.classList.contains("fp-toc-collapsed")) collapsedLevels.push(level);
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
// find in file — client-side text search + highlight. Alpine.store since
// the results/count live in the find sub-panel but the <mark> highlights
// live inside article.file-content, elsewhere on the page — a page-wide
// singleton like $store.filePanel/$store.rail, not an x-for'd component.
// `matches` holds plain render data (before/text/after) for base.gohtml's
// x-for results list; the actual <mark> element refs stay in a parallel
// module-scope array (_findMarkEls) instead of inside the reactive store,
// so Alpine's reactivity never has to proxy live DOM nodes.
// ================================================================
let _findMarkEls = [];

document.addEventListener("alpine:init", () => {
  Alpine.store("fileFind", {
    query: "",
    matches: [],
    index: -1,

    get count() {
      if (!this.query.trim()) return "";
      return this.matches.length > 0 ? `${this.index + 1} / ${this.matches.length}` : "0";
    },

    search(query) {
      this.query = query;
      clearFindHighlights();
      this.matches = [];
      this.index = -1;
      _findMarkEls = [];
      if (!query.trim()) return;
      const article = document.querySelector("article.file-content");
      if (!article) return;

      const walker = document.createTreeWalker(article, NodeFilter.SHOW_TEXT);
      const textNodes = [];
      let node;
      while ((node = walker.nextNode())) {
        const tag = node.parentNode?.tagName?.toLowerCase();
        if (tag === "script" || tag === "style") continue;
        textNodes.push(node);
      }

      const q = query.toLowerCase();
      textNodes.forEach((n) => _highlightInNode(n, q, this.matches, _findMarkEls));

      if (this.matches.length > 0) this.goTo(0);
    },

    goTo(i) {
      this.index = i;
      document.querySelectorAll("mark.kfind").forEach((m) => m.classList.remove("current"));
      const el = _findMarkEls[this.index];
      if (!el) return;
      el.classList.add("current");
      el.scrollIntoView({ behavior: "smooth", block: "center" });
      Alpine.nextTick(() => {
        document.querySelector(".fp-find-item.active")?.scrollIntoView({ block: "nearest" });
      });
    },

    next() {
      if (this.matches.length === 0) return;
      this.goTo((this.index + 1) % this.matches.length);
    },

    prev() {
      if (this.matches.length === 0) return;
      this.goTo((this.index - 1 + this.matches.length) % this.matches.length);
    },

    reset() {
      clearFindHighlights();
      this.query = "";
      this.matches = [];
      this.index = -1;
      _findMarkEls = [];
    },
  });
});

function _highlightInNode(textNode, q, result, marks) {
  const text = textNode.textContent;
  const lower = text.toLowerCase();
  if (!lower.includes(q)) return;
  const parent = textNode.parentNode;
  const frag = document.createDocumentFragment();
  const newMarks = [];
  let last = 0;
  let idx = lower.indexOf(q);
  while (idx !== -1) {
    if (idx > last) frag.appendChild(document.createTextNode(text.slice(last, idx)));
    const mark = document.createElement("mark");
    mark.className = "kfind";
    mark.textContent = text.slice(idx, idx + q.length);
    frag.appendChild(mark);
    newMarks.push(mark);
    last = idx + q.length;
    idx = lower.indexOf(q, last);
  }
  if (last < text.length) frag.appendChild(document.createTextNode(text.slice(last)));
  parent.replaceChild(frag, textNode);
  // Context is read from DOM siblings AFTER insertion.
  // Staying within the same parent means no before context when at the start of a block element.
  for (const mark of newMarks) {
    marks.push(mark);
    result.push({ before: _ctxBefore(mark, 2), text: mark.textContent, after: _ctxAfter(mark, 3) });
  }
}

function _ctxBefore(node, numWords) {
  // No before context if the match is the first child of its parent (= start of line)
  const prev = node.previousSibling;
  if (!prev) return "";
  const text = prev.textContent.replace(/\s+/g, " ").trimEnd();
  if (!text.trim()) return "";
  return text.split(/\s+/).filter(Boolean).slice(-numWords).join(" ");
}

function _ctxAfter(node, numWords) {
  const next = node.nextSibling;
  if (!next || next.nodeName === "MARK") return "";
  const text = next.textContent.replace(/\s+/g, " ").trimStart();
  if (!text.trim()) return "";
  return text.split(/\s+/).filter(Boolean).slice(0, numWords).join(" ");
}

function clearFindHighlights() {
  document.querySelectorAll("mark.kfind").forEach((m) => {
    m.replaceWith(document.createTextNode(m.textContent));
  });
  document.querySelector("article.file-content")?.normalize();
}
