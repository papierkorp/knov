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
  let parsed;
  try {
    parsed = JSON.parse(raw);
  } catch (e) {
    return [];
  }
  if (!Array.isArray(parsed)) return [];

  // the /chat page already renders the full chat UI in <main> with fixed,
  // non-namespaced ids (component-chat-history, chat-input, ...) — the
  // rail's "chat" snippet renders that exact same markup, so having both in
  // the DOM at once collides ids and htmx starts targeting the wrong copy.
  // it's redundant here anyway, so drop it from every group on this page.
  if (window.location.pathname === "/chat") {
    return parsed.map((g) => ({
      ...g,
      snippets: (g.snippets || []).filter((s) => (typeof s === "string" ? s : s.id) !== "chat"),
    }));
  }
  return parsed;
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
