/*! gwx — tiny vanilla helpers (no library, no build step). */
(function () {
  "use strict";

  function humanDur(secs) {
    if (secs <= 0) return "now";
    var d = Math.floor(secs / 86400);
    var h = Math.floor((secs % 86400) / 3600);
    var m = Math.floor((secs % 3600) / 60);
    var s = secs % 60;
    if (d > 0) return d + "d " + h + "h " + m + "m";
    if (h > 0) return h + "h " + m + "m " + s + "s";
    if (m > 0) return m + "m " + s + "s";
    return s + "s";
  }

  // Live "finishes in …" countdown. The server renders the authoritative text
  // and a `data-countdown` unix-seconds attribute; this only re-renders between
  // htmx resyncs. When the element is swapped by htmx the new node carries a
  // fresh timestamp, so it self-heals.
  function tick() {
    var now = Math.floor(Date.now() / 1000);
    var els = document.querySelectorAll("[data-countdown]");
    for (var i = 0; i < els.length; i++) {
      var finish = parseInt(els[i].getAttribute("data-countdown"), 10);
      if (!finish) continue;
      els[i].textContent = humanDur(finish - now);
    }
  }

  document.addEventListener("DOMContentLoaded", tick);
  setInterval(tick, 1000);
})();
