// Fires Umami events for signed-in pages and the gone page. Never loaded on the read page.
(function () {
  function send(name, props) {
    if (window.umami && typeof window.umami.track === "function") {
      window.umami.track(name, props || {});
    }
  }
  var body = document.body;
  if (body.dataset.track) {
    window.addEventListener("load", function () {
      var props = {};
      try { props = JSON.parse(body.dataset.trackProps || "{}"); } catch (e) {}
      send(body.dataset.track, props);
    });
  }
  document.addEventListener("click", function (ev) {
    var el = ev.target.closest("[data-copy]");
    if (!el || !navigator.clipboard) return;
    navigator.clipboard.writeText(el.dataset.copy).then(function () {
      var label = el.querySelector("[data-copy-label]") || el;
      var original = label.textContent;
      label.textContent = "Link copied";
      el.classList.add("is-copied");
      send("link-copy", { slug: el.dataset.slug || "" });
      setTimeout(function () { label.textContent = original; el.classList.remove("is-copied"); }, 2000);
    });
  });
})();
