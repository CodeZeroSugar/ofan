(function () {
  var host = null;

  function toastHost() {
    if (!host) {
      host = document.createElement("div");
      host.className =
        "pointer-events-none fixed bottom-4 right-4 z-50 flex flex-col items-end gap-2";
      host.setAttribute("role", "status");
      host.setAttribute("aria-live", "polite");
      document.body.appendChild(host);
    }
    return host;
  }

  window.ofan = {
    toast: function (text, ok) {
      var el = document.createElement("div");
      el.className =
        "pointer-events-auto rounded-lg border px-4 py-2 text-sm font-medium shadow-lg " +
        (ok
          ? "border-emerald-800 bg-emerald-950 text-emerald-200"
          : "border-red-800 bg-red-950 text-red-200");
      el.textContent = text;
      toastHost().appendChild(el);
      setTimeout(function () {
        el.remove();
      }, ok ? 4000 : 7000);
    },

    busy: function (el, on) {
      if (!el) return;
      el.disabled = !!on;
      el.setAttribute("aria-busy", on ? "true" : "false");
      el.classList.toggle("opacity-60", !!on);
      el.classList.toggle("cursor-wait", !!on);
    },

    // First call arms the button and swaps its label; a second call before the
    // timeout returns true (confirmed). Armed state always auto-disarms.
    arm: function (btn, armedLabel) {
      if (btn.getAttribute("data-armed")) {
        clearTimeout(btn._armTimer);
        btn.removeAttribute("data-armed");
        btn.textContent = btn.getAttribute("data-label");
        return true;
      }
      btn.setAttribute("data-armed", "1");
      btn.setAttribute("data-label", btn.textContent);
      btn.textContent = armedLabel;
      btn._armTimer = setTimeout(function () {
        btn.removeAttribute("data-armed");
        btn.textContent = btn.getAttribute("data-label");
      }, 4000);
      return false;
    },

    escapeHtml: function (s) {
      return String(s)
        .replace(/&/g, "&amp;")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;")
        .replace(/"/g, "&quot;");
    },
  };
})();
