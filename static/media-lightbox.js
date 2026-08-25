// lightbox for inline image previews (render_media.go RenderMediaPreviewWithSize,
// "enlarge" image click behavior) - loaded once per page via injectDefaultJS.
window.openMediaLightbox = function (btn) {
  var dlg = document.getElementById("media-lightbox");
  if (!dlg) {
    dlg = document.createElement("dialog");
    dlg.id = "media-lightbox";
    dlg.className = "media-lightbox";
    var closeBtn = document.createElement("button");
    closeBtn.type = "button";
    closeBtn.className = "media-lightbox-close";
    closeBtn.setAttribute("aria-label", "close");
    closeBtn.innerHTML = "&times;";
    closeBtn.onclick = function () {
      dlg.close();
    };
    var content = document.createElement("div");
    content.className = "media-lightbox-content";
    dlg.appendChild(closeBtn);
    dlg.appendChild(content);
    dlg.addEventListener("click", function (e) {
      if (e.target === dlg) dlg.close();
    });
    document.body.appendChild(dlg);
  }
  var content = dlg.querySelector(".media-lightbox-content");
  content.innerHTML = "";
  var img = document.createElement("img");
  img.src = btn.dataset.lightboxSrc;
  img.alt = btn.dataset.lightboxAlt || "";
  content.appendChild(img);
  dlg.showModal();
};
