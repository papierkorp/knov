// theme: builtin-reworked — "search" rail snippet: a filter-scoped search
// box that hx-gets straight into its own results div. No client-side state
// of its own beyond what htmx already tracks via the DOM, so `body` is the
// whole thing.
railSnippetByID("search").body = (groupID, instanceID) => `<div class="flyout-content" data-snippet="search">
  <div class="fp-search-options">
    <label class="fp-search-option-label"><input type="checkbox" id="fp-${instanceID}-title-only" name="titleonly" value="true"/><span>title only</span></label>
    <label class="fp-search-option-label"><input type="checkbox" id="fp-${instanceID}-history" name="history" value="true"/><span>search history</span></label>
    <a href="/search" class="flyout-header-link" style="margin-left:auto" title="open search page"><i class="fa fa-arrow-up-right-from-square"></i></a>
  </div>
  <div class="fp-search-row">
    <input type="text" class="fp-search-input" name="q" placeholder="search..."
           hx-get="/api/search?format=list"
           hx-target="#fp-${instanceID}-results"
           hx-include="#fp-${instanceID}-title-only, #fp-${instanceID}-history"
           hx-trigger="keyup changed delay:300ms, change from:#fp-${instanceID}-title-only, change from:#fp-${instanceID}-history"/>
  </div>
  <div id="fp-${instanceID}-results" class="fp-search-results"></div>
</div>`;
