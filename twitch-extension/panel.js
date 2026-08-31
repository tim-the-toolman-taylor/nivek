/*
 * Twitch Extension panel: a viewer clicks a product, spends Bits, and the signed
 * receipt is posted to the nivek backend (the EBS), which verifies it and feeds
 * the interaction into the overlay relay. See cmd/core-api/endpoints/overlay/
 * extension.go.
 *
 * Trust flow:
 *   - Twitch.ext.onAuthorized gives a per-viewer identity JWT (auth.token) that
 *     names the channel. We send it as a bearer so the backend knows which
 *     broadcaster to deliver to (a Bits receipt carries no channel).
 *   - Twitch.ext.bits.onTransactionComplete gives the signed transactionReceipt
 *     JWT (proof of purchase + product SKU). We post it in the body.
 *
 * NOTE: the backend host below must be added to the extension's "Allowlist for
 * URL Fetching Domains" in the developer console, or Twitch's CSP blocks fetch.
 */
(function () {
  "use strict";

  var EBS_URL = "https://peanutbudderbot.com/api/overlay/extension";

  var twitch = window.Twitch ? window.Twitch.ext : null;
  var authToken = null;
  var channelId = null;

  var productsEl = document.getElementById("products");
  var statusEl = document.getElementById("status");

  function setStatus(msg) {
    statusEl.textContent = msg || "";
  }

  if (!twitch) {
    setStatus("Twitch extension helper failed to load.");
    return;
  }

  twitch.onAuthorized(function (auth) {
    authToken = auth.token;
    channelId = auth.channelId;
    loadProducts();
  });

  // Some viewers can't use Bits (not logged in, unsupported client). Reflect it
  // rather than showing dead buttons.
  twitch.bits.onTransactionComplete(function (tx) {
    sendReceipt(tx);
  });

  twitch.bits.onTransactionCancelled(function () {
    setStatus("Cancelled.");
    setButtonsDisabled(false);
  });

  function loadProducts() {
    twitch.bits
      .getProducts()
      .then(function (products) {
        renderProducts(products || []);
      })
      .catch(function (err) {
        setStatus("Could not load products.");
        if (window.console) console.error("getProducts failed", err);
      });
  }

  function renderProducts(products) {
    productsEl.innerHTML = "";
    var usable = products.filter(function (p) {
      return p && p.sku;
    });
    if (usable.length === 0) {
      setStatus("No interactions available yet.");
      return;
    }
    usable.forEach(function (p) {
      var btn = document.createElement("button");
      btn.className = "product";
      btn.dataset.sku = p.sku;

      var name = document.createElement("span");
      name.textContent = p.displayName || p.sku;

      var cost = document.createElement("span");
      cost.className = "cost";
      cost.textContent = (p.cost && p.cost.amount ? p.cost.amount : "?") + " Bits";

      btn.appendChild(name);
      btn.appendChild(cost);
      btn.addEventListener("click", function () {
        setStatus("");
        setButtonsDisabled(true);
        twitch.bits.useBits(p.sku);
      });
      productsEl.appendChild(btn);
    });
  }

  function setButtonsDisabled(disabled) {
    var btns = productsEl.querySelectorAll("button.product");
    for (var i = 0; i < btns.length; i++) btns[i].disabled = disabled;
  }

  function sendReceipt(tx) {
    if (!authToken) {
      setStatus("Not authorized yet, try again.");
      setButtonsDisabled(false);
      return;
    }
    fetch(EBS_URL, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: "Bearer " + authToken,
      },
      body: JSON.stringify({ receipt: tx.transactionReceipt }),
    })
      .then(function (res) {
        if (res.ok) {
          setStatus("Sent! Watch the stream.");
        } else {
          setStatus("Something went wrong (" + res.status + ").");
        }
      })
      .catch(function (err) {
        setStatus("Network error sending the interaction.");
        if (window.console) console.error("send receipt failed", err);
      })
      .finally(function () {
        setButtonsDisabled(false);
      });
  }
})();
