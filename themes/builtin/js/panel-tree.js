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

      let url, values;
      if (type === "file") {
        url = pathURL("/api/files/rename/", path);
        values = { name: parentDir ? parentDir + "/" + newName : newName };
      } else {
        url = pathURL("/api/files/move-folder/", path);
        values = { target: parentDir || ".", name: newName };
      }

      // htmx handles the redirect, the notify toast and the panel reload (rail-core.js);
      // the row is the source (not the input) so it's still attached for the error toast;
      // finally also fires on network errors, which have no response
      row.addEventListener("htmx:finally:request", (e) => {
        if (e.detail.ctx.response?.status < 400) return;
        committed = false;
        cancel();
      }, { once: true });
      htmx.ajax("POST", url, { source: row, swap: "none", values });
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

    const { path: srcPath, type } = JSON.parse(payload);
    const targetDir = btn.dataset.path;
    const name = srcPath.split("/").pop();
    const newPath = targetDir + "/" + name;

    if (newPath === srcPath) return;
    // prevent folder drop into its own subtree
    if (type === "folder" && (newPath + "/").startsWith(srcPath + "/")) return;

    // htmx handles the redirect, the notify toast and the panel reload (rail-core.js)
    if (type === "folder") {
      htmx.ajax("POST", pathURL("/api/files/move-folder/", srcPath), { source: btn, swap: "none", values: { target: targetDir } });
    } else {
      htmx.ajax("POST", pathURL("/api/files/rename/", srcPath), { source: btn, swap: "none", values: { name: newPath } });
    }
  });
}
