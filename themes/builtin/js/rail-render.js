// theme: builtin — rail rendering: base.gohtml's #rail-site /
// #flyout templates iterate `$store.rail.groups` (the parsed "railLayout"
// setting, an opaque JSON string as far as the server is concerned) directly
// with alpine x-for; this file supplies:
//   - railParseLayout(): reads + parses that JSON off <body>
//   - panelGroup(group): the alpine component backing one flyout panel —
//     resolves the group's snippet ids against the registry (rail-snippets.js),
//     and owns which snippet tab is active plus its lazy-load/refresh
//
// Every RAIL_SNIPPETS entry supplies its own `.body(groupID, instanceID)`,
// set by its own panel-<name>.js file (e.g. panel-tree.js, panel-search.js)
// — this file just calls it, it has no per-snippet knowledge at all.

function railParseLayout() {
  const raw = document.body.dataset.railLayout;
  if (!raw) return [];
  try {
    const parsed = JSON.parse(raw);
    return Array.isArray(parsed) ? parsed : [];
  } catch (e) {
    return [];
  }
}

document.addEventListener("alpine:init", () => {
  Alpine.data("panelGroup", (group) => ({
    group,
    snippets: railResolveSnippets(group),
    activeSnippet: null,

    init() {
      // "link" entries have no tab content — never the initial active tab
      this.activeSnippet = this.snippets.find((s) => s.kind !== "link")?.tabId ?? null;
    },

    switchSnippet(tabId) {
      this.activeSnippet = tabId;
      lazyLoad("fp-" + this.group.id + "-" + tabId);
    },

    refresh() {
      if (this.activeSnippet) reloadPanel("fp-" + this.group.id + "-" + this.activeSnippet);
    },

    bodyHTML(snippet) {
      return snippet.body(this.group.id, this.group.id + "-" + snippet.tabId);
    },
  }));
});
