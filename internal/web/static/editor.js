// Trix set-up: prose formatting only. Loaded on the create and edit pages.
(function () {
  // No file attachments, by drop, paste or picker.
  document.addEventListener("trix-file-accept", function (e) { e.preventDefault(); });
  document.addEventListener("trix-attachment-add", function (e) { e.attachment.remove(); });
  document.addEventListener("trix-initialize", function (e) {
    var toolbar = e.target.toolbarElement;
    if (!toolbar) return;
    // Remove the code-block button and the attach-files group.
    [".trix-button--icon-code", ".trix-button-group--file-tools"].forEach(function (sel) {
      var el = toolbar.querySelector(sel);
      if (el) el.remove();
    });
  });
})();
