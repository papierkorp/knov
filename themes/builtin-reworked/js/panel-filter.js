// theme: builtin-reworked — "filter" rail snippet: a live criteria builder
// whose state (rows + and/or logic) persists per rail-group instance in
// localStorage — a snippet can be placed in multiple groups (see
// rail-layout-builder.js), so state is keyed and looked up by instanceID
// rather than a fixed id.

railSnippetByID("filter").body = (groupID, instanceID) => `<div class="fp-filter-controls">
    <button type="submit" form="filter-form-${instanceID}" class="btn-primary btn-small">apply</button>
    <button type="button" class="fp-filter-add btn-icon" title="add filter"
        hx-post="/api/filters/add-criteria" hx-target="#fp-${instanceID}-criteria" hx-swap="beforeend"
        hx-on--after-swap="saveFpFilterState('${instanceID}')"><i class="fa fa-plus"></i></button>
    <label class="fp-logic-toggle" title="and / or">
      <input type="checkbox" id="fp-${instanceID}-logic-checkbox" onchange="saveFpFilterState('${instanceID}')"/>
      <span class="fp-logic-label" data-off="and" data-on="or">and</span>
    </label>
  </div>
  <div class="flyout-content" data-snippet="filter">
    <form id="filter-form-${instanceID}" hx-post="/api/filters" hx-target="#fp-${instanceID}-results">
      <input type="hidden" name="display" value="list"/>
      <input type="hidden" name="limit" value="100"/>
      <input type="hidden" name="logic" id="fp-${instanceID}-logic-value" value="and"/>
      <div id="fp-${instanceID}-criteria" class="filter-criteria-container"></div>
    </form>
    <div id="fp-${instanceID}-results" class="fp-filter-results"></div>
  </div>`;

function saveFpFilterState(instanceID) {
  if (!instanceID) return;
  const criteria = document.getElementById("fp-" + instanceID + "-criteria");
  if (!criteria) return;
  const cb = document.getElementById("fp-" + instanceID + "-logic-checkbox");
  const logicVal = cb?.checked ? "or" : "and";
  const hidden = document.getElementById("fp-" + instanceID + "-logic-value");
  if (hidden) hidden.value = logicVal;
  const label = cb?.closest(".fp-logic-toggle")?.querySelector(".fp-logic-label");
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
    "fp-filter-state-" + instanceID,
    JSON.stringify({
      criteria: criteria.innerHTML,
      logic: logicVal,
      selectValues,
    }),
  );
}

function restoreFpFilterState(instanceID) {
  const criteria = document.getElementById("fp-" + instanceID + "-criteria");
  if (!criteria) return;
  const raw = localStorage.getItem("fp-filter-state-" + instanceID);

  if (raw) {
    try {
      const state = JSON.parse(raw);
      const cb = document.getElementById("fp-" + instanceID + "-logic-checkbox");
      const hidden = document.getElementById("fp-" + instanceID + "-logic-value");
      const label = cb?.closest(".fp-logic-toggle")?.querySelector(".fp-logic-label");
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
    saveFpFilterState(instanceID);
  });
  htmx.ajax("GET", `/api/filters/criteria-row?row_index=${idx}`, {
    target: criteria,
    swap: "innerHTML",
    headers: { Accept: "text/html" },
  });
}

// restore every filter panel instance present on the page
function restoreAllFpFilterStates() {
  document
    .querySelectorAll('.filter-criteria-container[id^="fp-"][id$="-criteria"]')
    .forEach((criteria) => {
      restoreFpFilterState(criteria.id.replace(/^fp-/, "").replace(/-criteria$/, ""));
    });
}

function fpFilterInstanceFor(el) {
  const criteria = el.closest(".filter-criteria-container");
  if (!criteria?.id) return null;
  return criteria.id.replace(/^fp-/, "").replace(/-criteria$/, "");
}

// save after criteria row removal
document.addEventListener("click", function (e) {
  const btn = e.target.closest(".filter-criteria-row .btn-danger");
  if (!btn) return;
  const instanceID = fpFilterInstanceFor(btn);
  if (!instanceID) return;
  setTimeout(() => saveFpFilterState(instanceID), 0);
});

// save when user changes a select or input inside criteria rows
document.addEventListener("change", function (e) {
  const instanceID = fpFilterInstanceFor(e.target);
  if (instanceID) saveFpFilterState(instanceID);
});
document.addEventListener("input", function (e) {
  const instanceID = fpFilterInstanceFor(e.target);
  if (instanceID) saveFpFilterState(instanceID);
});
