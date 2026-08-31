# Twitch Bits Extension (Panel)

The viewer-facing half of the overlay relay's Bits monetization. Viewers click a
product in this Panel, spend Bits, and Twitch's signed receipt is posted to
core-api (`POST /api/overlay/extension`), which verifies it and feeds the
interaction into the same overlay relay that cheers and power-ups use.

**Revenue:** Bits-in-Extensions is a native Twitch 80/20 split — the broadcaster
(Partner/Affiliate) receives 80%, the extension developer 20%, both paid by
Twitch. No payout code on our side. Broadcasters must be Affiliate or Partner to
use Bits at all.

## Files

- `panel.html` / `panel.js` / `panel.css` — the viewer Panel. `onAuthorized` →
  identity token (names the channel); `bits.getProducts()` → buttons;
  `bits.useBits(sku)` → purchase; `bits.onTransactionComplete` → POST the signed
  `transactionReceipt` to the EBS with the identity token as a bearer.
- `config.html` / `live_config.html` — required config views (minimal
  placeholders for now; the SKU→effect mapping lives in the overlay).

The backend host in `panel.js` (`EBS_URL`) must be added to the extension's
**Allowlist for URL Fetching Domains** in the developer console, or Twitch's CSP
blocks the fetch.

## One-time setup (developer + first broadcaster)

1. Create the extension at <https://dev.twitch.tv/console/extensions> → type
   **Panel**. Note the **Client ID**; under *Settings → Secret Keys* generate a
   **secret** (base64).
2. Under *Asset Hosting* set the **Panel Viewer Path** to `panel.html`, the
   **Config Path** to `config.html`, the **Live Config Path** to
   `live_config.html`, and a testing **Base URI** (Local Test) or upload the
   assets (Hosted Test).
3. Under *Capabilities*, add `peanutbudderbot.com` to the **Allowlist for URL
   Fetching Domains**, and enable **Bits**.
4. **Monetization → Bits Products**: add product `test_beans` (low cost, **In
   Development** ON — in-dev products let allow-listed testers transact without
   spending real Bits).
5. Set on the VPS master `.env` (`/home/deploy/actions-runner/.env`), then deploy
   core-api:
   ```
   OVERLAY_EXTENSION_CLIENT_ID=<extension client id>
   OVERLAY_EXTENSION_SECRET=<base64 secret from the console>
   # local browser testing only — overrides the CORS origin to the base URI:
   # OVERLAY_EXTENSION_ORIGIN=https://localhost:8080
   ```
6. In your channel's **Extensions Manager**, install your in-dev extension and
   **activate it in a Panel slot**.
7. Run the overlay (with the `on_extension_interaction` SKU map) using your
   device token.
8. Test: open your channel page, click the panel button, spend Bits on
   `test_beans` → 69 beans spawn (same as `!b 69`).

## Other broadcasters (after the extension is Released)

1. Be a Twitch **Affiliate or Partner**.
2. Extensions Manager → **Discover** → find the extension → **Install**.
3. **Activate** it in a **Panel** slot.
4. Sign into peanutbudderbot.com, mint an overlay device token, run the overlay.

## Review / release

Lifecycle: **Local Test → Hosted Test → Review → Approved → Released**. Only one
version can be in Review at a time; re-submitting resets the queue place. Review
needs the submitted test channels **live**, image assets, and a **walkthrough
video** (config → viewer experience). Non-intuitive setup is a common rejection.

- Life cycle: <https://dev.twitch.tv/docs/extensions/life-cycle/>
- Submission best practices: <https://dev.twitch.tv/docs/extensions/submission-best-practices/>
- Guidelines & policies: <https://dev.twitch.tv/docs/extensions/guidelines-and-policies/>
- Monetization: <https://dev.twitch.tv/docs/extensions/monetization/>
