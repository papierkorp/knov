// theme: builtin — settings page: info panel tab arranger, built entirely
// client-side against the fixed INFO_PANEL_TABS registry (the actual
// file-info-panel tab buttons in base.gohtml bind their order/visibility
// against this same id list via $store.filePanel — see panel-file.js). The
// server renders "infoPanelLayout" as a plain <textarea id="infoPanelLayout">
// (the generic settings-form "textarea" type, see render_themes.go — the
// server has no concept of the info panel's tabs at all). This file hides
// that textarea and replaces its UI with two drag lists ("hidden"/"shown"),
// same Sortable-owned two-container pattern as rail-layout-builder.js's
// top/bottom sections, re-serializing into the textarea on every drag.

const INFO_PANEL_TABS = [
  { id: "metadata", icon: "fa-circle-info", label: "metadata" },
  { id: "toc", icon: "fa-list", label: "table of contents" },
  { id: "history", icon: "fa-clock-rotate-left", label: "history" },
  { id: "chat", icon: "fa-comments", label: "chat" },
  { id: "references", icon: "fa-link", label: "references" },
  { id: "connections", icon: "fa-diagram-project", label: "connections" },
  { id: "find", icon: "fa-magnifying-glass", label: "find in file" },
];

function infoPanelLayoutParse(raw) {
  try {
    const parsed = JSON.parse(raw);
    if (Array.isArray(parsed) && parsed.length) return parsed;
  } catch (e) {
    // fall through to default
  }
  return INFO_PANEL_TABS.map((t) => t.id);
}

function infoPanelChipHTML(tab) {
  return `<li class="rail-snippet-chip" data-id="${tab.id}"><i class="fa ${tab.icon}"></i> ${tab.label}</li>`;
}

function infoPanelLayoutEmit(root, textarea) {
  const shown = Array.from(root.querySelector('[data-role="shown"]').children).map((li) => li.dataset.id);
  textarea.value = JSON.stringify(shown);
  textarea.dispatchEvent(new Event("change", { bubbles: true }));
}

function infoPanelLayoutBuilderInit(textarea) {
  const byId = Object.fromEntries(INFO_PANEL_TABS.map((t) => [t.id, t]));
  const shownIds = infoPanelLayoutParse(textarea.value).filter((id) => byId[id]);
  const hiddenIds = INFO_PANEL_TABS.map((t) => t.id).filter((id) => !shownIds.includes(id));

  const root = document.createElement("div");
  root.className = "rail-layout-builder";
  root.innerHTML =
    '<div class="rail-layout-section">' +
    '<div class="rail-layout-palette-label">hidden</div>' +
    `<ul class="rail-snippet-list" data-role="hidden">${hiddenIds.map((id) => infoPanelChipHTML(byId[id])).join("")}</ul>` +
    "</div>" +
    '<div class="rail-layout-section">' +
    '<div class="rail-layout-palette-label">shown</div>' +
    `<ul class="rail-snippet-list" data-role="shown">${shownIds.map((id) => infoPanelChipHTML(byId[id])).join("")}</ul>` +
    "</div>";

  root.querySelectorAll(".rail-snippet-list").forEach((list) => {
    new Sortable(list, {
      group: "info-panel-tabs",
      animation: 150,
      onEnd: () => infoPanelLayoutEmit(root, textarea),
    });
  });

  textarea.style.display = "none";
  textarea.insertAdjacentElement("afterend", root);
}

function infoPanelTryInitBuilder() {
  const textarea = document.getElementById("infoPanelLayout");
  if (!textarea || textarea.dataset.infoPanelBuilderInit) return;
  textarea.dataset.infoPanelBuilderInit = "1";
  infoPanelLayoutBuilderInit(textarea);
}

// same "reset to default" contract as rail-layout-builder.js: render_themes.go's
// generic textarea reset button writes the schema default into the textarea and
// fires this event - rebuild the drag UI from that new value.
document.addEventListener("settings-textarea-reset", (e) => {
  const textarea = e.target;
  if (textarea.id !== "infoPanelLayout") return;
  const old = textarea.nextElementSibling;
  if (old?.classList.contains("rail-layout-builder")) old.remove();
  infoPanelLayoutBuilderInit(textarea);
});

infoPanelTryInitBuilder();
document.addEventListener("htmx:after:settle", infoPanelTryInitBuilder);
