// theme: builtin-reworked — settings page: dynamic rail composition widget, built
// entirely client-side from the same RAIL_SNIPPETS registry rail-render.js
// resolves the actual rail against (rail-snippets.js) — no separate/parallel
// snippet list here. The server renders "railLayout" as a plain
// <textarea id="railLayout"> (the generic settings-form "textarea" type —
// see render_themes.go — the server has no concept of "rail" at all). This
// file hides that textarea and replaces its visible UI with a
// drag-and-drop-only builder (palette clone-drag is the sole way to place a
// component into a group — no add/remove-button path runs alongside it),
// keeping the textarea itself as the single source of truth: every change
// re-serializes into it and dispatches a "change" event (kept for any other
// listeners), but nothing auto-saves — the textarea's wrapping <form> only
// POSTs on submit, i.e. when the settings-form "save" button is clicked, so
// mid-drag reordering never triggers a save/reload.
//
// Structural composition (group order, group<->group/palette snippet
// membership) stays Sortable-owned imperative DOM, same call as tree's
// rename/dnd in phase 3/4 — expressing a Sortable-driven drag surface as
// reactive alpine x-for state would fight Sortable for DOM ownership
// (Sortable moves nodes directly; alpine's keyed diffing doesn't know about
// that move) for no behavioral gain. Only the icon/label/link fields are
// plain inputs re-serialized on "input", same as before — this now includes
// "link" chips dropped into a group (rail-snippet-list), not just the
// standalone link-group card, since a link is the one snippet configured
// per-placement rather than picked as-is (see railLinkChipHTML/
// railSerializeChip).
//
// Groups render in a rail "top" section (before the rail-spacer, e.g.
// browse/search/...) or "bottom" section (after it, e.g. settings/admin/
// server links by default) — see base.gohtml. The builder mirrors that as
// two Sortable-owned .rail-layout-groups containers sharing one Sortable
// group name, so dragging a card from one into the other is how its
// position changes; railLayoutSerialize reads it back off whichever
// container currently holds the card (railBuildSection/initRailGroupOrder).

function railLayoutParse(raw) {
  if (!raw) return [];
  try {
    const parsed = JSON.parse(raw);
    return Array.isArray(parsed) ? parsed : [];
  } catch (e) {
    return [];
  }
}

function railLayoutSerialize(root) {
  const groups = [];
  root.querySelectorAll(".rail-layout-group").forEach((card) => {
    const icon = card.querySelector(".rail-group-icon-input")?.value.trim() || "fa-star";
    const label = card.querySelector(".rail-group-label-input")?.value.trim() || "";
    // position comes from which of the two Sortable-owned containers (top/bottom
    // sections, see railBuildSection) currently holds the card, not a field on
    // the card itself — dragging between sections is how position changes.
    const entry = { id: card.dataset.groupId, icon, label };
    if (card.closest(".rail-layout-groups")?.dataset.position === "bottom") entry.position = "bottom";

    if (card.dataset.groupKind === "link") {
      const link = card.querySelector(".rail-group-url-input")?.value.trim() || "";
      if (!link) return; // no target yet — don't save a dead link
      entry.link = link;
      groups.push(entry);
      return;
    }

    const ul = card.querySelector('.rail-snippet-list[data-role="group"]');
    const snippets = ul
      ? Array.from(ul.children).map(railSerializeChip).filter((s) => s !== undefined)
      : [];
    if (snippets.length === 0) return; // drop empty groups
    entry.snippets = snippets;
    groups.push(entry);
  });
  return groups;
}

// most chips just carry their fixed RAIL_SNIPPETS id; "link" chips read
// back as an instance object instead (see railLinkChipHTML/railResolveSnippets).
// an unconfigured link (no target yet) is dropped, same as an empty
// standalone link group.
function railSerializeChip(li) {
  if (li.dataset.id !== "link") return li.dataset.id;
  const link = li.querySelector(".rail-link-url-input")?.value.trim() || "";
  if (!link) return undefined;
  return {
    id: "link",
    instanceId: li.dataset.instanceId,
    icon: li.querySelector(".rail-link-icon-input")?.value.trim() || "fa-link",
    label: li.querySelector(".rail-link-label-input")?.value.trim() || "",
    link,
  };
}

function railLayoutEmit(root) {
  const textarea = document.getElementById(root.dataset.key);
  if (!textarea) return;
  textarea.value = JSON.stringify(railLayoutSerialize(root));
  textarea.dispatchEvent(new Event("change", { bubbles: true }));
}

function railChipHTML(s) {
  return `<li class="rail-snippet-chip" data-id="${s.id}" data-kind="${s.kind}" draggable="true"><i class="fa ${s.icon}"></i> ${s.label}</li>`;
}

// a chip placed inside a group is usually just the plain palette chip
// (railChipHTML) for a fixed snippet id. "link" is the exception: it needs
// its own icon/label/target inputs right on the chip, same fields as a
// standalone link group (railBuildGroupCard) — see railSerializeChip for
// the matching read-back.
function railLinkChipHTML(entry) {
  const s = { ...railSnippetByID("link"), ...entry };
  return `<li class="rail-snippet-chip rail-link-chip" data-id="link" data-kind="link" data-instance-id="${railEsc(s.instanceId)}" draggable="true">` +
    '<i class="fa fa-grip-vertical rail-link-chip-handle"></i>' +
    `<input type="text" class="form-input rail-link-icon-input" value="${railEsc(s.icon || "")}" placeholder="fa-link" title="font awesome icon class, e.g. fa-star"/>` +
    `<input type="text" class="form-input rail-link-label-input" value="${railEsc(s.label || "")}" placeholder="name"/>` +
    `<input type="text" class="form-input rail-link-url-input" value="${railEsc(s.link || "")}" placeholder="/kanban"/>` +
    "</li>";
}

function railGroupChipHTML(entry) {
  if (typeof entry === "string") {
    const s = railSnippetByID(entry);
    return s ? railChipHTML(s) : "";
  }
  return entry.id === "link" ? railLinkChipHTML(entry) : "";
}

function railNewGroupId() {
  return "g" + Date.now().toString(36) + Math.floor(Math.random() * 1000).toString(36);
}

function railNewInstanceId() {
  return "i" + Date.now().toString(36) + Math.floor(Math.random() * 1000).toString(36);
}

function railBuildGroupCard(g) {
  const card = document.createElement("div");
  card.className = "rail-layout-group";
  card.dataset.groupId = g.id;

  if (g.link !== undefined) {
    card.dataset.groupKind = "link";
    card.innerHTML =
      '<div class="rail-layout-group-header">' +
      '<i class="fa fa-grip-vertical rail-group-handle"></i>' +
      `<input type="text" class="form-input rail-group-icon-input" value="${railEsc(g.icon || "")}" placeholder="fa-star" title="font awesome icon class, e.g. fa-star"/>` +
      `<input type="text" class="form-input rail-group-label-input" value="${railEsc(g.label || "")}" placeholder="name"/>` +
      `<input type="text" class="form-input rail-group-url-input" value="${railEsc(g.link || "")}" placeholder="/kanban"/>` +
      '<button type="button" class="rail-group-remove-btn" title="remove group"><i class="fa fa-trash"></i></button>' +
      "</div>";
    return card;
  }

  card.dataset.groupKind = "panel";
  const chips = (g.snippets || []).map(railGroupChipHTML).join("");
  card.innerHTML =
    '<div class="rail-layout-group-header">' +
    '<i class="fa fa-grip-vertical rail-group-handle"></i>' +
    `<input type="text" class="form-input rail-group-icon-input" value="${railEsc(g.icon || "")}" placeholder="fa-star" title="font awesome icon class, e.g. fa-star"/>` +
    `<input type="text" class="form-input rail-group-label-input" value="${railEsc(g.label || "")}" placeholder="name"/>` +
    '<button type="button" class="rail-group-remove-btn" title="remove group"><i class="fa fa-trash"></i></button>' +
    "</div>" +
    `<ul class="rail-snippet-list" data-role="group">${chips}</ul>`;
  return card;
}

// every snippet's panel is self-contained and instance-scoped (rail-render.js
// derives all its element ids from groupID + snippetID), so the same
// snippet can be dropped into as many groups as you like — except ones
// flagged RAIL_SNIPPETS[].singleton (currently just "chat": its
// server-rendered component, bulk-select bar and move-to-file forms all use
// page-wide fixed ids, so only one instance of it can safely exist at once).
// A snippet still can't appear twice *within the same group* either way.
function railSnippetInUse(snippetId, excludeEl) {
  return Array.from(
    document.querySelectorAll('.rail-snippet-list[data-role="group"] > li'),
  ).some((li) => li !== excludeEl && li.dataset.id === snippetId);
}

function canAcceptRailDrop(toEl, dragEl) {
  if (toEl.dataset.role === "palette") return true;
  const snippet = railSnippetByID(dragEl.dataset.id);
  // each "link" placement is independently configured, so any number of
  // them can sit in the same group or across groups — no dedup
  if (snippet?.kind === "link") return true;
  if (snippet?.singleton) return !railSnippetInUse(dragEl.dataset.id, dragEl);
  return !Array.from(toEl.children).some(
    (li) => li !== dragEl && li.dataset.id === dragEl.dataset.id,
  );
}

// a "link" chip dropped fresh from the palette is still the plain,
// un-editable palette clone (Sortable clones the DOM node directly, it
// doesn't go through railGroupChipHTML) — swap it for the editable,
// instance-scoped version the first time it lands in a group. A chip
// dragged between two groups already carries data-instance-id, so this is
// a no-op for it and its configured icon/label/link survive the move.
function railUpgradeDroppedChip(li) {
  if (li.dataset.id !== "link" || li.dataset.instanceId) return;
  const wrap = document.createElement("div");
  wrap.innerHTML = railLinkChipHTML({ instanceId: railNewInstanceId() });
  li.replaceWith(wrap.firstElementChild);
}

function initRailSnippetLists(root) {
  root.querySelectorAll(".rail-snippet-list").forEach((list) => {
    if (list.dataset.sortableInit) return;
    list.dataset.sortableInit = "1";
    const isPalette = list.dataset.role === "palette";
    new Sortable(list, {
      group: {
        name: "rail-snippets",
        // dragging *out of* the palette copies (the palette always shows
        // every snippet, even ones already placed in a group — put() below
        // rejects the drop if canAcceptRailDrop finds it already in use)
        pull: isPalette ? "clone" : true,
        put: (to, _from, dragEl) => canAcceptRailDrop(to.el, dragEl),
      },
      // link chips carry their own text inputs — let clicks/drags on those
      // edit text instead of starting a chip drag (the chip stays
      // draggable via its grip icon or any of its non-input area)
      filter: "input",
      preventOnFilter: false,
      animation: 150,
      onAdd: isPalette ? undefined : (evt) => railUpgradeDroppedChip(evt.item),
      onEnd: () => railLayoutEmit(root),
    });
  });
}

// the rail's actual order — dragging group cards up/down here reorders it;
// both sections share one Sortable group name so a card can also be dragged
// from one section into the other, which is how a group's top/bottom
// position changes (read back off the containing section in
// railLayoutSerialize — nothing to update on the card itself).
function initRailGroupOrder(root) {
  root.querySelectorAll(".rail-layout-groups").forEach((groupsContainer) => {
    if (groupsContainer.dataset.sortableInit) return;
    groupsContainer.dataset.sortableInit = "1";
    new Sortable(groupsContainer, {
      group: "rail-layout-groups",
      handle: ".rail-group-handle",
      animation: 150,
      onEnd: () => railLayoutEmit(root),
    });
  });
}

// one half of the builder — a "top" or "bottom" section, each its own drop
// zone + its own add-group/add-link buttons, so placement is chosen at
// add-time (or later, by dragging the card into the other section).
function railBuildSection(position, groups) {
  const section = document.createElement("div");
  section.className = "rail-layout-section";

  const label = document.createElement("div");
  label.className = "rail-layout-palette-label";
  label.textContent = position;
  section.appendChild(label);

  const groupsContainer = document.createElement("div");
  groupsContainer.className = "rail-layout-groups";
  groupsContainer.dataset.position = position;
  groups.forEach((g) => groupsContainer.appendChild(railBuildGroupCard(g)));
  section.appendChild(groupsContainer);

  const addButtons = document.createElement("div");
  addButtons.className = "rail-layout-add-buttons";
  addButtons.innerHTML =
    `<button type="button" class="btn-secondary btn-small" data-action="add-group" data-position="${position}">+ add group</button>` +
    `<button type="button" class="btn-secondary btn-small" data-action="add-link" data-position="${position}">+ add link</button>`;
  section.appendChild(addButtons);

  return section;
}

function railLayoutBuilderInit(textarea) {
  const root = document.createElement("div");
  root.className = "rail-layout-builder";
  root.dataset.key = textarea.id;

  const palette = document.createElement("div");
  palette.className = "rail-layout-palette";
  palette.innerHTML =
    `<div class="rail-layout-palette-label">available</div>` +
    `<ul class="rail-snippet-list" data-role="palette">${RAIL_SNIPPETS.map(railChipHTML).join("")}</ul>`;
  root.appendChild(palette);

  const allGroups = railLayoutParse(textarea.value);
  root.appendChild(railBuildSection("top", allGroups.filter((g) => g.position !== "bottom")));
  root.appendChild(railBuildSection("bottom", allGroups.filter((g) => g.position === "bottom")));

  root.addEventListener("input", (e) => {
    if (
      e.target.matches(
        ".rail-group-icon-input, .rail-group-label-input, .rail-group-url-input, " +
          ".rail-link-icon-input, .rail-link-label-input, .rail-link-url-input",
      )
    ) {
      railLayoutEmit(root);
    }
  });

  root.addEventListener("click", (e) => {
    const removeBtn = e.target.closest(".rail-group-remove-btn");
    if (removeBtn) {
      removeBtn.closest(".rail-layout-group")?.remove();
      railLayoutEmit(root);
      return;
    }
    const addGroupBtn = e.target.closest('[data-action="add-group"]');
    if (addGroupBtn) {
      const container = root.querySelector(`.rail-layout-groups[data-position="${addGroupBtn.dataset.position}"]`);
      container.appendChild(railBuildGroupCard({ id: railNewGroupId(), icon: "fa-star", label: "", snippets: [] }));
      initRailSnippetLists(root);
      return;
    }
    const addLinkBtn = e.target.closest('[data-action="add-link"]');
    if (addLinkBtn) {
      const container = root.querySelector(`.rail-layout-groups[data-position="${addLinkBtn.dataset.position}"]`);
      container.appendChild(railBuildGroupCard({ id: railNewGroupId(), icon: "fa-link", label: "", link: "" }));
    }
  });

  textarea.style.display = "none";
  textarea.insertAdjacentElement("afterend", root);
  initRailSnippetLists(root);
  initRailGroupOrder(root);
}

function railTryInitBuilder() {
  const textarea = document.getElementById("railLayout");
  if (!textarea || textarea.dataset.railBuilderInit) return;
  textarea.dataset.railBuilderInit = "1";
  railLayoutBuilderInit(textarea);
}

// the generic "reset to default" button (render_themes.go's textarea case)
// writes the schema default straight into the textarea and fires this event
// - rebuild the drag-and-drop UI from that new value so it doesn't go stale
// next to the (still unsaved, until "save" is clicked) reset text.
document.addEventListener("settings-textarea-reset", (e) => {
  const textarea = e.target;
  if (textarea.id !== "railLayout") return;
  const old = textarea.nextElementSibling;
  if (old?.classList.contains("rail-layout-builder")) old.remove();
  railLayoutBuilderInit(textarea);
});

railTryInitBuilder();
document.addEventListener("htmx:afterSettle", railTryInitBuilder);
