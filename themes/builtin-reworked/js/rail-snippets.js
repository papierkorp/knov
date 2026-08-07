// theme: builtin-reworked — rail snippet pool: the fixed, code-defined building
// blocks available for rail groups (tree, browse, chat, ...). This is the
// client-side source of truth now — the server only stores/serves the
// opaque "railLayout" JSON setting (see theme.json), it has no concept of
// "snippet" or "rail" at all. Shared by rail-render.js (renders the actual
// rail) and rail-layout-builder.js (settings page builder UI).
//
// labels are plain English, not run through the server's translation
// system — there's no client-side i18n bridge anywhere else in the app
// either, so this matches existing precedent rather than inventing one.
const RAIL_SNIPPETS = [
  { id: "new", icon: "fa-plus", label: "new", kind: "custom" },
  // the only snippet configured per-drop rather than picked as-is — each
  // placement gets its own icon/label/target instead of the fixed ones
  // above, so a group entry for it is an instance object, not this id
  // string (see railResolveSnippets below and rail-layout-builder.js,
  // which is where instanceId gets assigned).
  { id: "link", icon: "fa-link", label: "link", kind: "link" },
  {
    id: "tree", icon: "fa-folder-tree", label: "tree", kind: "content", search: "generic",
    url: () => "/api/files/tree?actions=true",
  },
  {
    id: "browse", icon: "fa-folder-open", label: "browse", kind: "content", search: "generic",
    url: (groupID) => "/api/files/folder?path=&target=%23fp-" + groupID + "-browse-content",
  },
  {
    id: "overview", icon: "fa-list", label: "overview", kind: "content", search: "generic",
    url: () => "/api/files/list?actions=true",
  },
  {
    id: "tags", icon: "fa-tag", label: "tags", kind: "content", search: "generic",
    url: () => "/api/metadata/tags?actions=true",
  },
  {
    id: "folders", icon: "fa-folder", label: "folders", kind: "content", search: "generic",
    url: () => "/api/metadata/folders?actions=true",
  },
  {
    id: "collections", icon: "fa-layer-group", label: "collections", kind: "content", search: "generic",
    url: () => "/api/metadata/collections?actions=true",
  },
  {
    id: "dashboards", icon: "fa-gauge", label: "dashboards", kind: "content", search: "generic",
    url: () => "/api/dashboards",
  },
  {
    id: "editor", icon: "fa-pen-nib", label: "editor", kind: "content", search: "generic",
    url: () => "/api/metadata/editors",
  },
  {
    id: "filters", icon: "fa-filter", label: "filters", kind: "content", search: "generic",
    url: () => "/api/files/browse?metadata=editor&value=filter-editor",
  },
  {
    id: "notifications", icon: "fa-bell", label: "notifications", kind: "content", search: "generic",
    url: () => "/api/notifications",
  },
  { id: "search", icon: "fa-magnifying-glass", label: "search", kind: "custom" },
  { id: "filter", icon: "fa-filter", label: "filter", kind: "custom" },
  { id: "media", icon: "fa-photo-film", label: "media", kind: "custom" },
  {
    id: "latest", icon: "fa-clock-rotate-left", label: "latest changes", kind: "content", search: "latest",
    url: () => "/api/git/latestchanges?count=50",
  },
  {
    // singleton: its server-rendered component (bulk-select bar, move-to-file
    // forms, load-more button) all use page-wide fixed ids — a second
    // instance would collide with the first rather than getting its own
    // independent panel, so the builder restricts it to one group.
    id: "chat", icon: "fa-message", label: "chat", kind: "content", singleton: true,
    url: () => "/api/chat/messages?short=true",
  },
  // fixed-target links, same "kind: link" navigate-away behaviour as the
  // generic "link" snippet above, just with a pre-set icon/label/url instead
  // of a per-instance one — drag straight into any group, or leave as its
  // own group (see theme.json's default railLayout). settings/admin are
  // full pages you act on (forms, navigation of their own) so they stay
  // navigate-away "link" snippets; logs/jobs/version/changelog are read-only
  // views, so they're "content" snippets instead — loaded as a flyout
  // slideout (like tree/browse/...) off the same fragment endpoint the full
  // /system/* page uses (see render_system.go's RenderVersionInfo/
  // RenderChangelog and api_system.go's handleAPIGetJobs/GetLogs/
  // GetSystemVersion/GetSystemChangelog).
  { id: "settings", icon: "fa-gear", label: "settings", kind: "link", link: "/settings" },
  { id: "admin", icon: "fa-screwdriver-wrench", label: "admin", kind: "link", link: "/admin" },
  { id: "logs", icon: "fa-file-lines", label: "logs", kind: "content", url: () => "/api/logs" },
  { id: "jobs", icon: "fa-list-check", label: "jobs", kind: "content", url: () => "/api/system/jobs" },
  { id: "version", icon: "fa-circle-info", label: "version", kind: "content", url: () => "/api/system/version" },
  { id: "changelog", icon: "fa-scroll", label: "changelog", kind: "content", url: () => "/api/system/changelog" },
];

function railSnippetByID(id) {
  return RAIL_SNIPPETS.find((s) => s.id === id);
}

// a group's `snippets` entries are usually just a plain RAIL_SNIPPETS id
// string. "link" entries are instance objects instead
// ({ id: "link", instanceId, icon, label, link }), since each placement is
// independently configured. `tabId` is what panelGroup (rail-render.js)
// uses to key/switch tabs — plain ids aren't unique once a group can hold
// more than one link instance.
function railResolveSnippets(group) {
  return (group.snippets || [])
    .map((entry) => {
      if (typeof entry === "string") {
        const base = railSnippetByID(entry);
        return base ? { ...base, tabId: base.id } : null;
      }
      const base = railSnippetByID(entry.id);
      return base ? { ...base, ...entry, tabId: entry.instanceId } : null;
    })
    .filter(Boolean);
}

function railEsc(s) {
  return String(s == null ? "" : s).replace(/[&<>"']/g, (c) => ({
    "&": "&amp;",
    "<": "&lt;",
    ">": "&gt;",
    '"': "&quot;",
    "'": "&#39;",
  })[c]);
}
