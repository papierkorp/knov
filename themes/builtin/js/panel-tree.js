// theme: builtin — "tree" rail snippet: same generic content shape
// as browse/overview/etc. (its `.body` is assigned along with the rest of
// them in panel-content.js), plus inline rename and drag&drop move on top.
// Event delegation is registered once against #flyout rather than per group
// instance, since a tree item is identified by its own data-path/data-type
// attributes, not by which group/instanceID it's rendered under — so this
// already supports the same snippet sitting in multiple groups at once.

// ================================================================
// tree inline rename
// ================================================================
function initTreeRename() {
  const flyout = document.getElementById("flyout");
  if (!flyout) return;

  flyout.addEventListener("click", (e) => {
    const renameBtn = e.target.closest(".browse-rename-btn");
    if (!renameBtn) return;
    e.preventDefault();
    e.stopPropagation();

    const path = renameBtn.dataset.path;
    const type = renameBtn.dataset.type;
    const currentName = path.split("/").pop();
    const parentDir = path.includes("/") ? path.slice(0, path.lastIndexOf("/")) : "";
    // reload the specific snippet instance this happened in, not the whole group
    const panelId = renameBtn.closest(".rail-tab-panel")?.id;

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
          } else if (panelId) {
            reloadPanel(panelId);
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
  const flyout = document.getElementById("flyout");
  if (!flyout) return;

  flyout.addEventListener("dragstart", (e) => {
    const el = e.target.closest("[data-path][draggable]");
    if (!el) return;
    e.dataTransfer.setData(TREE_DND_TYPE, JSON.stringify({ path: el.dataset.path, type: el.dataset.type }));
    e.dataTransfer.effectAllowed = "move";
  });

  flyout.addEventListener("dragend", () => {
    flyout
      .querySelectorAll(".fp-tree-dir.drag-over")
      .forEach((b) => b.classList.remove("drag-over"));
  });

  flyout.addEventListener("dragover", (e) => {
    if (!e.dataTransfer.types.includes(TREE_DND_TYPE)) return;
    const btn = e.target.closest("button.fp-tree-dir");
    if (!btn) return;
    e.preventDefault();
    e.dataTransfer.dropEffect = "move";
  });

  flyout.addEventListener("dragenter", (e) => {
    if (!e.dataTransfer.types.includes(TREE_DND_TYPE)) return;
    const btn = e.target.closest("button.fp-tree-dir");
    if (!btn) return;
    flyout
      .querySelectorAll(".fp-tree-dir.drag-over")
      .forEach((b) => b.classList.remove("drag-over"));
    btn.classList.add("drag-over");
  });

  flyout.addEventListener("dragleave", (e) => {
    const btn = e.target.closest("button.fp-tree-dir");
    if (!btn) return;
    if (!btn.contains(e.relatedTarget)) btn.classList.remove("drag-over");
  });

  flyout.addEventListener("drop", (e) => {
    const btn = e.target.closest("button.fp-tree-dir");
    if (!btn) return;
    const payload = e.dataTransfer.getData(TREE_DND_TYPE);
    if (!payload) return;
    e.preventDefault();
    btn.classList.remove("drag-over");
    const panelId = btn.closest(".rail-tab-panel")?.id;

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
          if (panelId) reloadPanel(panelId);
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
          } else if (panelId) {
            reloadPanel(panelId);
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
