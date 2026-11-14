"use strict";

// CLI Tollgate UI. State comes from /api/state (long-polled); every change is a
// POST /api/<action>. Rendering is string templates per region, skipped while
// the user is typing inside that region.

const $ = (sel) => document.querySelector(sel);

const ICONS = {
  copy: '<path d="M8 2h7a2 2 0 0 1 2 2v9h-1.5V4a.5.5 0 0 0-.5-.5H8V2Zm-3 3h7a2 2 0 0 1 2 2v9a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V7a2 2 0 0 1 2-2Zm0 1.5a.5.5 0 0 0-.5.5v9c0 .28.22.5.5.5h7a.5.5 0 0 0 .5-.5V7a.5.5 0 0 0-.5-.5H5Z"/>',
  check: '<path d="M7.5 13.6 3.9 10a.75.75 0 1 0-1.06 1.06l4.13 4.13c.3.3.77.3 1.06 0l9.13-9.13A.75.75 0 1 0 16.1 5L7.5 13.6Z"/>',
  eye: '<path d="M10 7a3 3 0 1 1 0 6 3 3 0 0 1 0-6Zm0 1.5a1.5 1.5 0 1 0 0 3 1.5 1.5 0 0 0 0-3ZM10 4c3.6 0 6.7 2.4 7.9 5.8a.75.75 0 0 1 0 .4C16.7 13.6 13.6 16 10 16s-6.7-2.4-7.9-5.8a.75.75 0 0 1 0-.4C3.3 6.4 6.4 4 10 4Zm0 1.5c-2.8 0-5.3 1.8-6.4 4.5 1.1 2.7 3.6 4.5 6.4 4.5s5.3-1.8 6.4-4.5c-1.1-2.7-3.6-4.5-6.4-4.5Z"/>',
  eyeOff: '<path d="M2.2 2.2a.75.75 0 0 1 1.06 0l14.5 14.5a.75.75 0 1 1-1.06 1.06l-2.5-2.5A8.3 8.3 0 0 1 10 16c-3.6 0-6.7-2.4-7.9-5.8a.75.75 0 0 1 0-.4 8.6 8.6 0 0 1 2.8-3.8L2.2 3.26a.75.75 0 0 1 0-1.06Zm3.75 5.8a7 7 0 0 0-2.35 2c1.1 2.7 3.6 4.5 6.4 4.5 1.1 0 2.1-.27 3.05-.76l-1.6-1.6a3 3 0 0 1-4.14-4.14L5.95 8ZM10 4c3.6 0 6.7 2.4 7.9 5.8a.75.75 0 0 1 0 .4 8.6 8.6 0 0 1-1.86 3.06l-1.06-1.06A7 7 0 0 0 16.4 10c-1.1-2.7-3.6-4.5-6.4-4.5-.56 0-1.1.07-1.62.2L7.2 4.52A8.4 8.4 0 0 1 10 4Z"/>',
  more: '<path d="M5 10a1.25 1.25 0 1 1-2.5 0A1.25 1.25 0 0 1 5 10Zm6.25 0a1.25 1.25 0 1 1-2.5 0 1.25 1.25 0 0 1 2.5 0Zm5 1.25a1.25 1.25 0 1 0 0-2.5 1.25 1.25 0 0 0 0 2.5Z"/>',
  chat: '<path d="M3 5a3 3 0 0 1 3-3h6a3 3 0 0 1 3 3v3a3 3 0 0 1-3 3H8.6l-2.9 2.3A.75.75 0 0 1 4.5 12.7V10.8A3 3 0 0 1 3 8.2V5Zm3-1.5A1.5 1.5 0 0 0 4.5 5v3.2c0 .7.5 1.3 1.2 1.47.33.08.55.37.55.7v.83l1.93-1.54a.75.75 0 0 1 .47-.16H12A1.5 1.5 0 0 0 13.5 8V5A1.5 1.5 0 0 0 12 3.5H6Zm10.5 4.3a.75.75 0 0 1 1.5-.05V10a3 3 0 0 1-1.5 2.6v1.65a.75.75 0 0 1-1.2.6l-2.5-1.9H10a.75.75 0 0 1 0-1.5h3.05c.16 0 .32.05.45.15l1.5 1.14v-.58c0-.36.25-.66.6-.74A1.5 1.5 0 0 0 16.5 10V7.8Z"/>',
  sparkle: '<path d="M10 2c.33 0 .62.21.72.53l1.2 3.86a2.5 2.5 0 0 0 1.69 1.69l3.86 1.2a.75.75 0 0 1 0 1.44l-3.86 1.2a2.5 2.5 0 0 0-1.69 1.69l-1.2 3.86a.75.75 0 0 1-1.44 0l-1.2-3.86a2.5 2.5 0 0 0-1.69-1.69l-3.86-1.2a.75.75 0 0 1 0-1.44l3.86-1.2a2.5 2.5 0 0 0 1.69-1.69l1.2-3.86A.75.75 0 0 1 10 2Zm0 3.3-.48 1.54a4 4 0 0 1-2.68 2.68L5.3 10l1.54.48a4 4 0 0 1 2.68 2.68L10 14.7l.48-1.54a4 4 0 0 1 2.68-2.68L14.7 10l-1.54-.48a4 4 0 0 1-2.68-2.68L10 5.3Z"/>',
  warn: '<path d="M8.68 2.79a1.5 1.5 0 0 1 2.64 0l6.5 11.99A1.5 1.5 0 0 1 16.5 17h-13a1.5 1.5 0 0 1-1.32-2.22l6.5-11.99ZM10 7a.75.75 0 0 0-.75.75v4a.75.75 0 0 0 1.5 0v-4A.75.75 0 0 0 10 7Zm0 7.5a.9.9 0 1 0 0-1.8.9.9 0 0 0 0 1.8Z"/>',
  info: '<path d="M10 2a8 8 0 1 1 0 16 8 8 0 0 1 0-16Zm0 7a.75.75 0 0 0-.75.75v4a.75.75 0 0 0 1.5 0v-4A.75.75 0 0 0 10 9Zm0-3.25a.9.9 0 1 0 0 1.8.9.9 0 0 0 0-1.8Z"/>',
  close: '<path d="M4.4 4.4a.75.75 0 0 1 1.06 0L10 8.94l4.54-4.54a.75.75 0 1 1 1.06 1.06L11.06 10l4.54 4.54a.75.75 0 1 1-1.06 1.06L10 11.06 5.46 15.6a.75.75 0 0 1-1.06-1.06L8.94 10 4.4 5.46a.75.75 0 0 1 0-1.06Z"/>',
  chart: '<path d="M10 2.75a.75.75 0 0 1 .75.75v13a.75.75 0 0 1-1.5 0v-13A.75.75 0 0 1 10 2.75Zm5 5a.75.75 0 0 1 .75.75v8a.75.75 0 0 1-1.5 0v-8a.75.75 0 0 1 .75-.75Zm-10 3a.75.75 0 0 1 .75.75v5a.75.75 0 0 1-1.5 0v-5A.75.75 0 0 1 5 10.75Z"/>',
  key: '<path d="M12.5 2a5.5 5.5 0 1 1-1.67 10.74l-1.05 1.04a.75.75 0 0 1-.53.22H8v1.25a.75.75 0 0 1-.75.75H6v1.25a.75.75 0 0 1-.75.75h-2.5a.75.75 0 0 1-.75-.75v-2.5c0-.2.08-.39.22-.53l5.04-5.04A5.5 5.5 0 0 1 12.5 2Zm0 1.5a4 4 0 0 0-3.8 5.24.75.75 0 0 1-.18.77L3.5 14.56v1.44h1v-1.25c0-.41.34-.75.75-.75H6.5v-1.25c0-.41.34-.75.75-.75h1.69l1.23-1.23a.75.75 0 0 1 .77-.18A4 4 0 1 0 12.5 3.5Zm1 2a1 1 0 1 1 0 2 1 1 0 0 1 0-2Z"/>',
  stack: '<path d="M9.67 2.08a.75.75 0 0 1 .66 0l7.25 3.5a.75.75 0 0 1 0 1.35l-7.25 3.5a.75.75 0 0 1-.66 0l-7.25-3.5a.75.75 0 0 1 0-1.35l7.25-3.5ZM4.47 6.25 10 8.92l5.53-2.67L10 3.58 4.47 6.25Zm-2.4 3.9a.75.75 0 0 1 1-.35L10 13.17l6.93-3.37a.75.75 0 1 1 .66 1.35l-7.26 3.5a.75.75 0 0 1-.66 0l-7.25-3.5a.75.75 0 0 1-.35-1Z"/>',
};
const icon = (name) => `<svg viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">${ICONS[name]}</svg>`;
const PROVIDER_ICON = { codex: "chat", claude: "sparkle" };

const esc = (v) =>
  String(v ?? "").replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]);

// ---- Local UI state (never sent to the app) -------------------------------

const ui = {
  tab: "overview",
  revealKey: false,
  menu: null, // id of the open dropdown
  copied: null, // id of the copy button showing a check mark
  snippet: null, // selected setup snippet id
  provider: "codex",
  filter: "",
  portDraft: null,
};

let state = null;

// ---- API ------------------------------------------------------------------

async function call(action, body) {
  try {
    const res = await fetch(`/api/${action}`, {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-Tollgate": "1" },
      body: JSON.stringify(body || {}),
    });
    if (!res.ok) {
      const data = await res.json().catch(() => ({}));
      toast(data.error || t("Request failed ({0})", res.status));
      return false;
    }
    return true;
  } catch (e) {
    toast(t("CLI Tollgate is not responding."));
    return false;
  }
}

async function poll() {
  let since = "";
  for (;;) {
    try {
      const res = await fetch(`/api/state${since !== "" ? `?since=${since}` : ""}`, {
        headers: { "X-Tollgate": "1" },
        cache: "no-store",
      });
      if (res.status === 403) {
        showDisconnected(t("This page has expired. Open CLI Tollgate from its tray icon."));
        return;
      }
      if (!res.ok) throw new Error(String(res.status));
      state = await res.json();
      since = state.revision;
      render();
    } catch (e) {
      showDisconnected(t("CLI Tollgate is not running."));
      await new Promise((r) => setTimeout(r, 1500));
      since = "";
    }
  }
}

function showDisconnected(text) {
  const status = $("#status");
  status.className = "status failed";
  status.querySelector(".status-text").textContent = text;
}

// ---- Helpers --------------------------------------------------------------

let toastTimer;
function toast(text) {
  const el = $("#toast");
  el.textContent = text;
  el.classList.add("show");
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => el.classList.remove("show"), 1800);
}

function flashCopied(id) {
  ui.copied = id;
  render();
  setTimeout(() => {
    if (ui.copied === id) {
      ui.copied = null;
      render();
    }
  }, 1400);
}

function formatTokens(n) {
  if (n >= 1e6) return `${(n / 1e6).toFixed(1)}M`;
  if (n >= 1e3) return `${(n / 1e3).toFixed(1)}k`;
  return String(n);
}

function masked(key) {
  if (!key) return "";
  if (key.length <= 10) return "•".repeat(key.length);
  return key.slice(0, 3) + "•".repeat(8) + key.slice(-4);
}

const running = () => state && state.engine === "running";

function toggle(id, checked, action, { disabled = false, label = "" } = {}) {
  return `<label class="switch"><input type="checkbox" id="${id}" data-change="${action}" ${checked ? "checked" : ""} ${
    disabled ? "disabled" : ""
  } aria-label="${esc(label)}"><span class="track"><span class="knob"></span></span></label>`;
}

function copyButton(id, text, label) {
  const done = ui.copied === id;
  const title = esc(t("Copy {0}", label));
  return `<button class="icon-btn ${done ? "ok" : ""}" data-click="copy" data-id="${id}" data-text="${esc(text)}" title="${title}" aria-label="${title}">${icon(
    done ? "check" : "copy",
  )}</button>`;
}

// ---- Rendering ------------------------------------------------------------

const lastHTML = {};
function setRegion(el, html) {
  if (lastHTML[el.id] === html) return;
  // Don't yank a field out from under the user; re-render once they leave it.
  const active = document.activeElement;
  if (active && el.contains(active) && (active.tagName === "INPUT" || active.tagName === "SELECT") && active.type !== "checkbox") {
    el.dataset.stale = "1";
    return;
  }
  lastHTML[el.id] = html;
  el.innerHTML = html;
  delete el.dataset.stale;
}

function render() {
  if (!state) return;
  renderHeader();
  setRegion($("#banners"), bannersHTML());
  setRegion($("#view-overview"), overviewHTML());
  setRegion($("#view-models"), modelsHTML());
  setRegion($("#view-settings"), settingsHTML());
  renderAccess();
}

// ---- Access requests ------------------------------------------------------

function formatDate(iso) {
  const d = new Date(iso);
  return isNaN(d) ? "" : d.toLocaleDateString(LOCALE, { month: "short", day: "numeric", year: "numeric" });
}

// What to say about where a request comes from; warn marks the risky cases.
function originNote(origin) {
  if (origin === "local program") return { text: t("A program on this computer, not a web page."), warn: true };
  if (/^http:\/\/(localhost|127\.0\.0\.1|\[::1\])(:|$)/.test(origin)) return { text: t("A page served from this computer."), warn: false };
  if (origin.startsWith("http://")) return { text: t("Not a secure (https) site."), warn: true };
  return null;
}

// A pending request takes over the window until it is answered.
function renderAccess() {
  const el = $("#access");
  const reqs = state.accessRequests || [];
  const req = reqs[0];
  if (!req) {
    if (!el.hidden) {
      el.hidden = true;
      el.innerHTML = "";
      lastHTML.access = "";
    }
    return;
  }
  const note = originNote(req.origin);
  const waiting = reqs.length - 1;
  const more = waiting > 0 ? `<p class="caption">${waiting === 1 ? t("1 more request waiting.") : t("{0} more requests waiting.", waiting)}</p>` : "";
  const html = `<div class="dialog access-dialog" role="alertdialog" aria-modal="true" aria-labelledby="access-title">
      <div class="body">
        <div class="access-icon">${icon("key")}</div>
        <h2 id="access-title">${esc(t("Allow “{0}” to use your subscriptions?", req.name))}</h2>
        <div class="origin mono" title="${esc(req.origin)}">${esc(req.origin)}</div>
        ${note ? `<p class="caption ${note.warn ? "warn" : ""}">${esc(note.text)}</p>` : ""}
        <p>${t("It will be able to send requests through your signed-in ChatGPT and Claude accounts. You can revoke access anytime in Settings.")}</p>
        ${more}
      </div>
      <div class="actions">
        <button class="btn accent" data-click="allowAccess" data-id="${esc(req.id)}" data-name="${esc(req.name)}">${t("Allow")}</button>
        <button class="btn" data-click="denyAccess" data-id="${esc(req.id)}">${t("Deny")}</button>
      </div>
    </div>`;
  if (lastHTML.access === html) return;
  lastHTML.access = html;
  el.innerHTML = html;
  el.hidden = false;
  // Deny has focus so a stray Enter never grants access.
  el.querySelector("[data-click='denyAccess']").focus();
}

function grantsHTML() {
  const grants = state.grants || [];
  const rows = grants.length
    ? grants
        .map(
          (g) => `<div class="row">
        <div class="tile">${icon("key")}</div>
        <div class="grow">
          <div class="label truncate" title="${esc(g.name)}">${esc(g.name)}</div>
          <div class="caption truncate" title="${esc(g.origin)}">${esc(g.origin)} · ${esc(t("allowed {0}", formatDate(g.granted_at)))} · <span class="mono">${esc(g.key_hint)}</span></div>
        </div>
        <button class="btn danger" data-click="revokeGrant" data-id="${esc(g.id)}" data-name="${esc(g.name)}" ${running() ? "" : "disabled"}>${t("Revoke")}</button>
      </div>`,
        )
        .join("")
    : `<div class="row"><div class="grow caption">${t("No apps yet. When a web app asks for access, CLI Tollgate asks you first.")}</div></div>`;
  return `<h2 class="section-title">${t("Connected apps")}</h2><div class="card">${rows}</div>`;
}

function renderHeader() {
  const status = $("#status");
  status.className = `status ${state.engine}`;
  const text = {
    running: t("Running on {0}", state.baseURL),
    starting: t("Starting…"),
    stopped: t("Stopped"),
    failed: state.engineMessage || t("The engine failed to start."),
  }[state.engine];
  status.querySelector(".status-text").textContent = text;
  status.title = text;
  const toggleEl = $("#engine-toggle");
  toggleEl.checked = state.engine === "running" || state.engine === "starting";
  document.title = "CLI Tollgate";
}

function selectTab(tab) {
  if (!["overview", "models", "settings"].includes(tab)) tab = "overview";
  ui.tab = tab;
  ui.menu = null;
  document.querySelectorAll(".pivot a").forEach((a) => a.setAttribute("aria-selected", String(a.dataset.tab === tab)));
  document.querySelectorAll(".view").forEach((v) => v.classList.toggle("active", v.id === `view-${tab}`));
  if (tab === "models") call("refresh");
}

function bannersHTML() {
  let html = "";
  if (state.lastError) {
    html += `<div class="infobar error">${icon("warn")}<div class="msg">${esc(state.lastError)}</div>
      <button class="icon-btn" data-click="dismissError" title="${t("Dismiss")}" aria-label="${t("Dismiss")}">${icon("close")}</button></div>`;
  }
  if (state.update.available) {
    html += `<div class="infobar info">${icon("info")}<div class="msg">${esc(t("CLI Tollgate {0} is ready to install.", state.update.available))}</div>
      <button class="btn small accent" data-click="installUpdate">${t("Restart to update")}</button></div>`;
  }
  return html;
}

function overviewHTML() {
  const accounts = state.accounts
    .map((a) => {
      let actions;
      if (a.loggingIn) {
        actions = `<span class="spinner" aria-label="${t("Waiting for sign in")}"></span><button class="btn small" data-click="cancelLogin">${t("Cancel")}</button>`;
      } else if (a.signedIn) {
        const id = `menu-${a.provider}`;
        actions = `<div class="menu-anchor"><button class="icon-btn" data-click="menu" data-id="${id}" ${
          state.loginActive ? "disabled" : ""
        } title="${t("Account options")}" aria-label="${t("Account options")}">${icon("more")}</button>${
          ui.menu === id
            ? `<div class="menu"><button data-click="login" data-provider="${a.provider}">${t("Switch account…")}</button><button class="danger" data-click="logout" data-provider="${a.provider}">${t("Sign out")}</button></div>`
            : ""
        }</div>`;
      } else {
        actions = `<button class="btn small accent" data-click="login" data-provider="${a.provider}" ${
          !running() || state.loginActive ? "disabled" : ""
        }>${t("Sign in")}</button>`;
      }
      return `<div class="row">
        <div class="tile">${icon(PROVIDER_ICON[a.provider])}</div>
        <div class="grow">
          <div class="label">${esc(a.title)}${a.plan ? `<span class="badge">${esc(a.plan)}</span>` : ""}</div>
          <div class="caption truncate ${a.warning ? "warn" : ""}" title="${esc(a.detail)}">${esc(a.detail)}</div>
        </div>
        ${actions}
      </div>`;
    })
    .join("");

  const snippets = state.snippets || [];
  if (!ui.snippet || !snippets.some((s) => s.id === ui.snippet)) ui.snippet = snippets[0]?.id;
  const snippet = snippets.find((s) => s.id === ui.snippet);
  const key = state.apiKey;

  const revealLabel = ui.revealKey ? t("Hide API key") : t("Show API key");
  const connect = key
    ? `<div class="kv"><div class="k">${t("Base URL")}</div><div class="v mono truncate">${esc(state.baseURL)}</div>${copyButton(
        "copy-url",
        state.baseURL,
        t("base URL"),
      )}</div>
      <div class="kv"><div class="k">${t("API key")}</div><div class="v mono truncate" title="${t("Double-click to reveal")}" data-dblclick="reveal">${esc(
        ui.revealKey ? key : masked(key),
      )}</div>
        <button class="icon-btn" data-click="reveal" title="${revealLabel}" aria-label="${revealLabel}">${icon(
          ui.revealKey ? "eyeOff" : "eye",
        )}</button>${copyButton("copy-key", key, t("API key"))}</div>
      <div class="snippet">
        <div class="snippet-head">
          <select class="select" data-change="snippet" aria-label="${t("Setup snippet")}">${snippets
            .map((s) => `<option value="${esc(s.id)}" ${s.id === ui.snippet ? "selected" : ""}>${esc(s.title)}</option>`)
            .join("")}</select>
          <button class="btn" data-click="copy" data-id="copy-snippet" data-text="${esc(snippet?.text)}" data-label="${esc(
            t("{0} setup", snippet?.title),
          )}">${icon(ui.copied === "copy-snippet" ? "check" : "copy")}${t("Copy")}</button>
        </div>
        <pre class="code">${esc(snippet?.text)}</pre>
      </div>`
    : `<div class="row"><div class="grow caption">${
        state.engine === "running" ? t("Loading…") : t("Start the proxy to see the base URL and API key.")
      }</div></div>`;

  const usage = state.usage
    ? `<div class="usage" title="${t("{0} failed requests today", state.usage.failures)}">${icon("chart")}${t(
        "{0} requests · {1} tokens today",
        state.usage.requests,
        formatTokens(state.usage.tokens.total),
      )}</div>`
    : "";

  return `
    <h2 class="section-title">${t("Accounts")}</h2>
    <div class="card">${accounts}</div>
    <h2 class="section-title">${t("Connect")}</h2>
    <div class="card">${connect}</div>
    ${usage}`;
}

function modelsHTML() {
  const all = state.models[ui.provider] || [];
  const q = ui.filter.trim().toLowerCase();
  const shown = q ? all.filter((m) => m.id.toLowerCase().includes(q) || (m.displayName || "").toLowerCase().includes(q)) : all;
  const exposed = all.filter((m) => m.exposed).length;
  const account = state.accounts.find((a) => a.provider === ui.provider);
  const title = account ? account.title : ui.provider;

  const seg = state.accounts
    .map((a) => `<button data-click="provider" data-provider="${a.provider}" aria-pressed="${a.provider === ui.provider}">${esc(a.title)}</button>`)
    .join("");

  let list;
  if (!all.length) {
    list = `<div class="empty">${icon("stack")}<div>${running() ? t("No models in the catalog.") : t("Start the proxy to load models.")}</div></div>`;
  } else if (!shown.length) {
    list = `<div class="empty"><div>${esc(t("No models match “{0}”.", ui.filter))}</div></div>`;
  } else {
    list = `<div class="card">${shown
      .map((m) => {
        const meta = [m.displayName, m.contextLength > 0 ? t("{0}k context", Math.round(m.contextLength / 1000)) : ""].filter(Boolean).join(" · ");
        return `<div class="model ${m.exposed ? "" : "hidden-model"}">
          <input type="checkbox" class="check" data-change="exposed" data-model="${esc(m.id)}" ${m.exposed ? "checked" : ""} title="${t("Expose this model to clients")}" aria-label="${esc(t("Expose {0}", m.id))}">
          <div class="grow"><div class="mono truncate" title="${esc(m.id)}">${esc(m.id)}</div>${
            meta ? `<div class="caption truncate">${esc(meta)}</div>` : ""
          }</div>
          <input class="input small mono alias" data-alias="${esc(m.id)}" value="${esc(m.alias)}" placeholder="${t("alias")}" spellcheck="false" title="${t("Extra client-visible name. The original ID keeps working.")}" aria-label="${esc(
            t("Alias for {0}", m.id),
          )}">
        </div>`;
      })
      .join("")}</div>`;
  }

  return `
    <div class="toolbar">
      <div class="segmented" role="group" aria-label="${t("Provider")}">${seg}</div>
      <input class="input" id="filter" type="search" placeholder="${t("Filter")}" value="${esc(ui.filter)}" spellcheck="false" aria-label="${t("Filter models")}">
    </div>
    <div class="list-head">
      <span>${t("{0} of {1} exposed", exposed, all.length)}</span>
      ${account && !account.signedIn ? `<span class="caption warn" title="${esc(t("Not signed in to {0}", title))}">${t("· not signed in")}</span>` : ""}
      <span class="grow"></span>
      <button class="btn small" data-click="allExposed" data-on="false" ${all.length ? "" : "disabled"}>${t("Hide all")}</button>
      <button class="btn small" data-click="allExposed" data-on="true" ${all.length ? "" : "disabled"}>${t("Show all")}</button>
    </div>
    ${list}`;
}

function settingsHTML() {
  const s = state.settings;
  const u = state.update;
  const port = ui.portDraft ?? String(state.port);
  const portValid = /^\d+$/.test(port) && +port >= 1024 && +port <= 65535;
  const commit = state.engineCommit ? `CLIProxyAPI @ ${state.engineCommit.slice(0, 12)}` : t("bundled");
  const version = state.build ? `${state.version} (${state.build})` : state.version;
  const cloak = state.cloaking;
  const cloakCaption =
    cloak.claude && cloak.codex
      ? t("Claude models go out as Claude Code, GPT / Codex as Codex CLI.")
      : !cloak.claude && !cloak.codex
        ? t("Requests keep the calling client's identity.")
        : t("Partially on. Adjust each side below.");

  return `
    <h2 class="section-title">${t("General")}</h2>
    <div class="card">
      <div class="row"><div class="grow label">${t("Launch at sign-in")}</div>${toggle("lal", state.launchAtLogin, "setLaunchAtLogin", { label: t("Launch at sign-in") })}</div>
    </div>

    <h2 class="section-title">${t("Proxy")}</h2>
    <div class="card">
      <div class="row">
        <div class="grow label">${t("Port")}</div>
        <div class="inline">
          <input class="input port-input" id="port" inputmode="numeric" value="${esc(port)}" aria-label="${t("Port")}">
          <button class="btn" data-click="applyPort" ${!portValid || +port === state.port || !running() ? "disabled" : ""}>${t("Apply")}</button>
        </div>
      </div>
      <div class="row"><div class="grow label">${t("Listens on")}</div><span class="caption">${t("127.0.0.1 only")}</span></div>
      <div class="row"><div class="grow label">${t("API key")}</div><button class="btn" data-click="regenerate" ${running() ? "" : "disabled"}>${t("Regenerate…")}</button></div>
    </div>
    <p class="footnote">${t("Web pages can call the proxy (CORS is open), but every request needs the API key.")}</p>

    ${grantsHTML()}
    <p class="footnote">${t("Revoking an app turns off its key right away. Regenerating your API key leaves connected apps working.")}</p>

    <h2 class="section-title">${t("Updates")}</h2>
    <div class="card">
      <div class="row"><div class="grow label">${t("Check for updates automatically")}</div>${toggle("u1", s.autoCheckUpdates, "setAutoCheckUpdates", {
        disabled: !u.supported,
        label: t("Check for updates automatically"),
      })}</div>
      <div class="row"><div class="grow"><div class="label">${t("Install updates when I quit")}</div><div class="caption">${t(
        "Downloaded updates are verified with an Ed25519 signature.",
      )}</div></div>${toggle(
        "u2",
        s.autoInstallUpdates,
        "setAutoInstallUpdates",
        { disabled: !u.supported || !s.autoCheckUpdates, label: t("Install updates when I quit") },
      )}</div>
      <div class="row">
        <div class="grow caption">${esc(u.message || (u.supported ? "" : t("Updates are only available in released builds.")))}</div>
        ${u.checking ? `<span class="spinner"></span>` : ""}
        ${
          u.available
            ? `<button class="btn accent" data-click="installUpdate">${esc(t("Install {0}", u.available))}</button>`
            : `<button class="btn" data-click="checkForUpdates" ${u.supported && !u.checking ? "" : "disabled"}>${t("Check now")}</button>`
        }
      </div>
    </div>

    <h2 class="section-title">${t("Advanced: Cloaking")}</h2>
    <div class="card">
      <div class="row">
        <div class="grow"><div class="label">${t("Cloak requests")}</div><div class="caption">${esc(cloakCaption)}</div></div>
        ${toggle("cloak", cloak.claude && cloak.codex, "setCloaking", { disabled: !running(), label: t("Cloak requests") })}
      </div>
      <div class="row"><div class="grow label">${t("Disguise Claude requests as Claude Code")}</div>${toggle("c1", state.cloaking.claude, "setClaudeCloaking", {
        disabled: !running(),
        label: t("Disguise Claude requests as Claude Code"),
      })}</div>
      <div class="row"><div class="grow label">${t("Send official Codex CLI headers for GPT / Codex")}</div>${toggle("c2", state.cloaking.codex, "setCodexCloaking", {
        disabled: !running(),
        label: t("Send official Codex CLI headers"),
      })}</div>
      <div class="row"><div class="grow label">${t("Cloak Claude model IDs in {0}", `<span class="mono">/v1/models</span>`)}</div>${toggle(
        "c3",
        state.cloaking.claudeModelList,
        "setClaudeCloakModelList",
        { disabled: !running(), label: t("Cloak Claude model IDs") },
      )}</div>
    </div>
    <p class="footnote">${t("The disguise follows the requested model. Requests that already come from Claude Code are passed through unchanged.")}</p>

    <h2 class="section-title">${t("About")}</h2>
    <div class="card">
      <div class="row"><div class="grow label">${t("Version")}</div><span class="caption">${esc(version)}</span></div>
      <div class="row"><div class="grow label">${t("Engine")}</div><span class="caption mono">${esc(commit)}</span></div>
      <div class="row"><div class="grow"><div class="label">${t("Data folder")}</div><div class="caption truncate" title="${esc(state.dataDir)}">${esc(
        state.dataDir,
      )}</div></div><button class="btn" data-click="openDataFolder">${t("Open")}</button></div>
      <div class="row"><div class="grow label">${t("Engine log")}</div><button class="btn" data-click="openLogs">${t("Open")}</button><button class="btn" data-click="restartEngine">${t(
        "Restart engine",
      )}</button></div>
      <div class="row"><div class="grow label">${t("Quit CLI Tollgate")}</div><button class="btn danger" data-click="quit">${t("Quit")}</button></div>
    </div>`;
}

// ---- Dialogs --------------------------------------------------------------

function confirmDialog({ title, message, confirm, danger }) {
  return new Promise((resolve) => {
    const modal = $("#modal");
    modal.innerHTML = `<div class="dialog" role="alertdialog" aria-modal="true" aria-labelledby="dlg-title">
      <div class="body"><h2 id="dlg-title">${esc(title)}</h2><p>${esc(message)}</p></div>
      <div class="actions"><button class="btn accent ${danger ? "danger-accent" : ""}" data-r="1">${esc(confirm)}</button><button class="btn" data-r="0">${t("Cancel")}</button></div>
    </div>`;
    modal.hidden = false;
    const done = (v) => {
      modal.hidden = true;
      modal.innerHTML = "";
      document.removeEventListener("keydown", onKey, true);
      resolve(v);
    };
    const onKey = (e) => {
      if (e.key === "Escape") {
        e.stopPropagation();
        done(false);
      }
    };
    document.addEventListener("keydown", onKey, true);
    modal.onclick = (e) => {
      const b = e.target.closest("[data-r]");
      if (b) done(b.dataset.r === "1");
      else if (e.target === modal) done(false);
    };
    modal.querySelector("[data-r='0']").focus();
  });
}

// ---- Events ---------------------------------------------------------------

const clicks = {
  async copy(el) {
    await call("copy", { text: el.dataset.text });
    flashCopied(el.dataset.id);
    if (el.dataset.label) toast(t("Copied {0}", el.dataset.label));
  },
  reveal() {
    ui.revealKey = !ui.revealKey;
    render();
  },
  menu(el) {
    ui.menu = ui.menu === el.dataset.id ? null : el.dataset.id;
    render();
  },
  login(el) {
    ui.menu = null;
    call("login", { provider: el.dataset.provider });
  },
  logout(el) {
    ui.menu = null;
    call("logout", { provider: el.dataset.provider });
  },
  cancelLogin: () => call("cancelLogin"),
  dismissError: () => call("dismissError"),
  installUpdate: () => call("installUpdate"),
  checkForUpdates: () => call("checkForUpdates"),
  openDataFolder: () => call("openDataFolder"),
  openLogs: () => call("openLogs"),
  restartEngine: () => call("restartEngine"),
  quit: () => call("quit"),
  provider(el) {
    ui.provider = el.dataset.provider;
    render();
  },
  allExposed(el) {
    call("setAllExposed", { provider: ui.provider, on: el.dataset.on === "true" });
  },
  async applyPort() {
    const port = parseInt(ui.portDraft ?? "", 10);
    if (!port) return;
    if (await call("setPort", { port })) {
      ui.portDraft = null;
      toast(t("Restarting on port {0}…", port));
    }
  },
  async allowAccess(el) {
    el.closest(".actions").querySelectorAll("button").forEach((b) => (b.disabled = true));
    if (await call("allowAccess", { id: el.dataset.id })) toast(t("Allowed {0}", el.dataset.name));
  },
  async denyAccess(el) {
    el.closest(".actions").querySelectorAll("button").forEach((b) => (b.disabled = true));
    await call("denyAccess", { id: el.dataset.id });
  },
  async revokeGrant(el) {
    const ok = await confirmDialog({
      title: t("Revoke access for “{0}”?", el.dataset.name),
      message: t("Its key stops working right away. It can ask for access again later."),
      confirm: t("Revoke"),
      danger: true,
    });
    if (ok && (await call("revokeGrant", { id: el.dataset.id }))) toast(t("Access revoked"));
  },
  async regenerate() {
    const ok = await confirmDialog({
      title: t("Regenerate the API key?"),
      message: t("Clients using the current key will stop working until you update them. Connected apps keep their own keys."),
      confirm: t("Regenerate"),
      danger: true,
    });
    if (ok && (await call("regenerateAPIKey"))) toast(t("New API key created"));
  },
};

document.addEventListener("click", (e) => {
  const el = e.target.closest("[data-click]");
  if (!el) {
    if (ui.menu && !e.target.closest(".menu")) {
      ui.menu = null;
      render();
    }
    return;
  }
  if (el.disabled) return;
  if (ui.menu && el.dataset.click !== "menu" && !el.closest(".menu")) ui.menu = null;
  clicks[el.dataset.click]?.(el);
});

document.addEventListener("dblclick", (e) => {
  if (e.target.closest("[data-dblclick='reveal']")) clicks.reveal();
});

document.addEventListener("change", (e) => {
  const el = e.target;
  const action = el.dataset.change;
  if (!action) return;
  if (action === "snippet") {
    ui.snippet = el.value;
    el.blur();
    render();
  } else if (action === "exposed") {
    call("setExposed", { provider: ui.provider, model: el.dataset.model, on: el.checked });
  } else {
    call(action, { on: el.checked });
  }
});

document.addEventListener("input", (e) => {
  const el = e.target;
  if (el.id === "filter") {
    ui.filter = el.value;
    renderFiltered();
  } else if (el.id === "port") {
    ui.portDraft = el.value.replace(/\D/g, "");
    const btn = document.querySelector("[data-click='applyPort']");
    const p = +ui.portDraft;
    if (btn) btn.disabled = !(p >= 1024 && p <= 65535) || p === state.port || !running();
  }
});

// The filter lives inside the models region, so re-render the list around it
// and put the caret back where it was.
function renderFiltered() {
  const input = $("#filter");
  const pos = input.selectionStart;
  input.blur();
  setRegion($("#view-models"), modelsHTML());
  const again = $("#filter");
  again.focus();
  again.setSelectionRange(pos, pos);
}

function commitAlias(el) {
  const model = el.dataset.alias;
  const current = (state.models[ui.provider] || []).find((m) => m.id === model)?.alias || "";
  if (el.value.trim() !== current) call("setAlias", { provider: ui.provider, model, alias: el.value });
}

document.addEventListener("keydown", (e) => {
  const el = e.target;
  if (e.key === "Enter" && el.dataset.alias !== undefined) {
    el.blur();
  } else if (e.key === "Enter" && el.id === "port") {
    clicks.applyPort();
  } else if (e.key === "Escape" && ui.menu) {
    ui.menu = null;
    render();
  }
});

document.addEventListener("focusout", (e) => {
  const el = e.target;
  if (el.dataset && el.dataset.alias !== undefined) commitAlias(el);
  // A region skipped a render while this field had focus; catch up now.
  setTimeout(() => {
    const region = el.closest && el.closest("[data-stale]");
    if (region) render();
  });
});

window.addEventListener("hashchange", () => selectTab(location.hash.slice(1)));

// The Windows shell calls this to switch tabs on an already open window.
window.tollgateShow = (tab) => {
  if (tab) location.hash = tab;
};

translateStatic();
selectTab(location.hash.slice(1) || "overview");
call("refresh");
poll();
