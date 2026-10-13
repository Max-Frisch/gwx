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

  // formatInt renders a whole number with thousands separators, mirroring the
  // server-side fmtInt used for the initial card values.
  function formatInt(n) {
    var s = String(n);
    var out = "";
    for (var i = 0; i < s.length; i++) {
      if (i > 0 && (s.length - i) % 3 === 0) out += ",";
      out += s[i];
    }
    return out;
  }

  // refreshUnitCosts rescales the resource costs on a ship/defense card to the
  // quantity typed (or stepped) into the build form. Each cost cell carries the
  // raw per-unit price and the planet's stock in data attributes.
  function refreshUnitCosts(input) {
    var card = input.closest(".bcard");
    if (!card) return;
    var qty = parseInt(input.value, 10);
    if (isNaN(qty) || qty < 0) qty = 0;
    var cells = card.querySelectorAll(".cost-amt");
    for (var i = 0; i < cells.length; i++) {
      var base = parseFloat(cells[i].getAttribute("data-base"));
      if (!isFinite(base)) continue;
      var have = parseFloat(cells[i].getAttribute("data-have"));
      if (!isFinite(have)) have = 0;
      var total = base * qty;
      cells[i].textContent = formatInt(Math.round(total));
      var ok = have >= total;
      cells[i].classList.toggle("v-ok", ok);
      cells[i].classList.toggle("v-bad", !ok);
    }
  }

  // Delegated so it keeps working after htmx swaps the page body.
  function onQtyChanged(e) {
    var t = e.target;
    if (t && t.matches && t.matches('input[name="qty"]')) refreshUnitCosts(t);
  }
  document.addEventListener("input", onQtyChanged);
  document.addEventListener("change", onQtyChanged);

  document.addEventListener("DOMContentLoaded", tick);
  setInterval(tick, 1000);
})();
