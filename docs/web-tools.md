# Web tools for the admin assistant: Tollgate API contract

Status: **v1.1** (client implemented in `apps/admin`, server implemented in
CLI Tollgate: `dylancetin/cli-tollgate`, engine submodule `dylancetin/CLIProxyAPI`, package
`internal/webtools`). This copy matches the client's `docs/tollgate-web-tools.md`; where the server
had to settle a detail the contract left open, it says so in **Implementation notes** at the end.

**v1.1 (keyless search)** is backward compatible with v1 clients: search works with no API key
(five of the six providers have a keyless mode), three providers were added (`parallel`,
`firecrawl`, `tinyfish`), the probe gained `keyless` and `mode`, and a search that names no
provider moves on to another provider when one is rate limited. See **Changes in v1.1** at the end
for the list and what a client may want to adopt.

The admin panel's AI assistant runs in the browser. To let it search the web and read pages it
needs a proxy (browsers block cross-origin reads), and that proxy is Tollgate, the desktop app
that already serves the chat API on `http://127.0.0.1:8317`. This document is the contract
between the two: the permission flow, the capability probe, both endpoints, errors, limits and
the safeguards each side owns.

Units: "characters" means Unicode code points. Offsets are opaque to the client: it only ever
passes back a `nextOffset` it received.

---

## 1. Scopes and the permission flow

### 1.1 Scopes

| Scope    | Grants                                                                 |
| -------- | ---------------------------------------------------------------------- |
| `models` | Everything `/v1` serves today (chat, responses, messages, models, …).  |
| `web`    | `GET /v1/tools`'s web entries and `POST /v1/tools/web_search`, `POST /v1/tools/web_fetch`. |

- `models` is always granted to an approved app in this version. A request without it gets it
  added; Tollgate does not enforce `models` on existing routes (no behaviour change for them).
- Grants written before scopes existed, and requests without `scopes`, mean `["models"]`.
- The user's own API key(s) from `access.api-keys` that have **no grant** (the key shown in
  Tollgate, a pasted "manual" key) have every scope.
- Unknown scope names are rejected: `400 {"error": "unknown scope: <name>", "code": "unknown_scope"}`.
  The `/access/*` routes keep their string `error`; new refusals add a machine-readable `code`
  next to it, which clients match on (they fall back to the text only for old servers).

### 1.2 New connection

```
POST /access/requests
{ "name": "KEL Derneği Yönetim Paneli", "key": "tg-app-…", "scopes": ["models", "web"] }
→ 202 { "id": "…", "status": "pending", "expires_at": "…", "scopes": ["models", "web"] }
```

All existing rules stay (loopback peer only, a management client must be watching, one pending
request per origin, 30 s cooldown after a denial, 5 min expiry, key format 32–256 of
`A-Z a-z 0-9 - _ .`).

The prompt names what is asked:

- macOS / Windows title: **Allow “{name}” to use your subscriptions and search the web?**
  (without `web`: the current title).
- Body adds, when `web` is requested: *It will also be able to search the web and read web pages
  through this computer. Sites see your IP address.* Turkish strings go with the existing
  localization (e.g. *“{name}” aboneliklerinizi kullanabilsin ve web'de arama yapabilsin mi?* /
  *Bu bilgisayar üzerinden web'de arama yapıp sayfaları okuyabilecek. Siteler IP adresinizi görür.*).
- Deny stays the default button. Allow grants every requested scope.

Polling is unchanged except that an approved request reports the granted scopes:

```
GET /access/requests/:id?wait=50
→ 200 { "id": "…", "status": "approved", "scopes": ["models", "web"] }
```

`scopes` is present only when `status` is `approved`. A client that sees no `scopes` there is
talking to an older Tollgate and must assume `["models"]`.

### 1.3 Scope upgrade (existing keys)

Every admin user already has an approved key without `web`. Re-pairing would leave a stale grant
behind, so an app can ask for more scopes **for the key it already holds**:

```
POST /access/requests
{ "name": "KEL Derneği Yönetim Paneli", "key": "<the already approved key>", "scopes": ["models", "web"] }
```

- Key belongs to a grant, the request's origin equals the grant's origin, and `scopes` adds at
  least one scope the grant lacks → `202` as above; it is an **upgrade request**.
- Key belongs to a grant but nothing new is asked →
  `409 {"error": "this key already has these scopes", "code": "scopes_already_granted"}`.
- Key belongs to a grant from another origin →
  `409 {"error": "this key belongs to another app", "code": "key_owned_by_other_app"}`.
- Key is one of the user's own keys (no grant) →
  `409 {"error": "this key is already allowed", "code": "key_already_allowed"}` (same text as today).

Management listing marks it so the prompt can say so:

```
GET /v8/management/access/requests
{ "requests": [ { "id", "name", "origin", "user_agent", "status", "created_at", "expires_at",
                  "scopes": ["models", "web"],
                  "upgrade": true, "grant_id": "…", "added_scopes": ["web"] } ] }
```

Upgrade prompt title: **Let “{name}” also search the web?** (TR: *“{name}” web'de de arama
yapabilsin mi?*), same body line about the IP address. Approving updates the grant's `scopes`
(no new grant, no new key). Denying leaves the grant as it was and starts the usual cooldown.

### 1.4 Grants

`AccessGrant` gains `"scopes": ["models", "web"]` (missing in old files → `["models"]`).
`GET /v8/management/access/grants` returns it; Settings → Connected apps shows a small "Web"
badge for grants that have it. Revoke stays whole-grant.

---

## 2. Capability probe

```
GET /v1/tools            Authorization: Bearer <key>
```

Under `/v1` with the normal auth middleware, so `401` for an unknown key.

```json
{
  "scopes": ["models", "web"],
  "tools": {
    "web_search": {
      "available": true,
      "provider": "brave",
      "providers": ["brave", "tavily", "exa", "parallel", "firecrawl", "tinyfish"],
      "keyless": ["tavily", "exa", "parallel", "firecrawl", "tinyfish"],
      "mode": "auto"
    },
    "web_fetch":  { "available": true, "render": false }
  },
  "limits": {
    "fetch_max_chars": 20000,
    "search_per_minute": 20,
    "fetch_per_minute": 40
  }
}
```

- Always `200` for an authenticated key, whatever its scopes, so the client can tell "needs
  permission" from "old Tollgate".
- `scopes`: what **this key** has.
- `available: false` comes with `"reason"`:
  - `"scope_required"`: the key lacks `web`;
  - `"disabled"`: web tools are switched off in Tollgate's settings;
  - `"not_configured"`: (`web_search` only) no search provider can search: the user switched
    keyless search off and set no API key. With the default settings this never happens.
    `web_fetch` works without any key.
- `providers`: the search providers that **can search**, in the order `brave`, `tavily`, `exa`,
  `parallel`, `firecrawl`, `tinyfish`. A provider can search when it has an API key, or when it
  supports keyless use (all but Brave) and keyless search is on (the default). Clients ignore
  names they don't know.
- `keyless` (v1.1): the subset of `providers` that will answer **without** an API key, with the
  provider's lower free limits (a daily or per-minute cap per IP address). Providers in
  `providers` but not here have a key.
- `mode` (v1.1): `"default"` when the user picked a provider as the default and it can search;
  `"auto"` when Tollgate picks (the default setting, or a chosen provider that can't search).
  In `auto` mode Tollgate keeps using the provider an app used last, else picks one at random,
  preferring providers with a key over keyless ones.
- `provider`: the provider the next search that names none will use: in `default` mode the
  chosen default; in `auto` mode the provider this key is bound to (picked on first use, at random
  among the providers with a key, else among the keyless ones; the probe makes that pick if no
  search has yet). A rate-limited (paused, §3) provider is skipped. Always one of `providers`. `provider`, `providers`, `keyless` and `mode` are omitted when `available` is
  false.
- `render` is `false` until a headless renderer exists; it is reserved, and clients send nothing
  for it.

Client behaviour:

| Probe result                                    | Admin does                                                      |
| ----------------------------------------------- | --------------------------------------------------------------- |
| `404` (route unknown) / network error           | No web tools. Nothing shown.                                    |
| `401`                                           | No web tools (the chat path reports the bad key).               |
| `200`, `web` not in `scopes`                    | No web tools; Connections view offers **Web erişimi iste** (upgrade request, §1.3). |
| `200`, tool `available: true`                   | Offers that tool (`webSearch` ↔ `web_search`, `fetchPage` ↔ `web_fetch`). |
| `200`, tool unavailable (`disabled`/`not_configured`) | That tool is not offered; Connections view says why.      |
| `200`, `web_search` available, `keyless` non-empty | Offers search. Optionally notes that keyless providers have lower limits (v1.1). |

The client caches the probe per (base URL, key) for 60 s and re-probes after an upgrade or a
`scope_required`/`disabled`/`not_configured` error from a tool call.

---

## 3. `POST /v1/tools/web_search`

Request:

```json
{ "query": "kabataş erkek lisesi 1908", "count": 5, "freshness": "year", "site": "wikipedia.org", "lang": "tr", "provider": "exa" }
```

| Field       | Type                                   | Rules                                                     |
| ----------- | -------------------------------------- | --------------------------------------------------------- |
| `query`     | string, required                       | trimmed, 1–200 characters                                 |
| `count`     | integer                                | 1–10, default 5                                           |
| `freshness` | `"day" \| "week" \| "month" \| "year"` | optional                                                  |
| `site`      | string                                 | optional hostname (`example.com`, no scheme/path); restricts results to it and its subdomains |
| `lang`      | string                                 | optional ISO 639-1 (`tr`, `en`); a hint for result language |
| `provider`  | `"brave" \| "tavily" \| "exa" \| "parallel" \| "firecrawl" \| "tinyfish"` | optional; one of the probe's `providers`. Absent → the default, with failover (below) |

Unknown fields are ignored. Response:

```json
{
  "provider": "exa",
  "results": [
    {
      "title": "Kabataş Erkek Lisesi - Vikipedi",
      "url": "https://tr.wikipedia.org/wiki/Kabata%C5%9F_Erkek_Lisesi",
      "snippet": "Kabataş Erkek Lisesi, 1908 yılında …",
      "published": "2025-03-01",
      "site": "tr.wikipedia.org"
    }
  ]
}
```

- `url`: absolute `https` URL. Results with other schemes, private hosts or blocked domains are
  dropped (not reported). Duplicate URLs collapse.
- `title` ≤ 200, `snippet` ≤ 300 characters, plain text (provider HTML/highlight tags stripped,
  entities decoded, whitespace collapsed).
- `published`: optional, ISO 8601 date or date-time, only when the provider gives one.
- `site`: the URL's hostname, lowercase.
- Empty `results` is a success. `provider`: the provider that answered. It may differ from the
  probe's `provider` after a failover.
- **Rate-limit failover (v1.1)**, only for a search that names **no** `provider`: when the
  provider answers `429` (or `402`, which keyless modes use once a free cap is spent), Tollgate
  pauses it for its `Retry-After` (60 s when absent) and asks the next provider that can search
  and isn't paused or already tried in this search (providers with a key first). A paused
  provider is skipped without a call until its pause ends. Any other failure (`5xx`, a bad key,
  garbage, a timeout) is returned as is, without trying another provider.
- A search that **names** a `provider` gets that provider only, as in v1. If it is rate limited
  the error is `search_failed` with `status: 429` and `retry_after`.
- When every provider that can search is paused, or all were tried and each answered `429`, the
  error is `502 search_failed` with `status: 429` and `retry_after` (seconds until the first
  pause ends).

Providers (Tollgate-internal; the client only sees the normalized shape):

| Provider    | Without a key                                    | With a key                         |
| ----------- | ------------------------------------------------ | ---------------------------------- |
| `brave`     | not available                                    | Brave Search API                   |
| `tavily`    | Tavily API, keyless access mode (small daily cap per IP) | Tavily API                 |
| `exa`       | Exa's hosted MCP server (`web_search_exa`)       | Exa REST API                       |
| `parallel`  | Parallel's search MCP server (`web_search`)      | same, with the key                 |
| `firecrawl` | Firecrawl's MCP server (`firecrawl_search`)      | same, with the key                 |
| `tinyfish`  | TinyFish's MCP server (`search`), keyless mode   | same, with the key (MCP session)   |

Field mapping:

| Field       | Brave Search API                     | Tavily                         | Exa (key: REST)                        | Exa (keyless: MCP) | Parallel | Firecrawl | TinyFish |
| ----------- | ------------------------------------ | ------------------------------ | -------------------------------------- | ------------------ | -------- | --------- | -------- |
| `query`     | `q`                                  | `query`                        | `query` (type `auto`)                  | `query` | `search_queries: [query]` | `query` | `query` |
| `count`     | `count`                              | `max_results`                  | `numResults`                           | `numResults` | trimmed | `limit` | trimmed |
| `freshness` | `freshness=pd/pw/pm/py`              | `time_range=day/week/month/year` | `startPublishedDate` = now − period  | hint in `objective` | hint in `objective` | `tbs=qdr:d/w/m/y` | `after_date` = now − period |
| `site`      | append ` site:<host>` to `q`         | `include_domains: [host]`      | `includeDomains: [host]`               | hint in `objective` | ` site:<host>` in the query | `includeDomains: [host]` | `include_domains` |
| `lang`      | `search_lang`                        | ignored                        | ignored                                | hint in `objective` | hint in `objective` | ignored | `language` |
| snippet     | `description`                        | `content`                      | `highlights`/`summary`/text excerpt (request a short one) | `Highlights:` text | `excerpts` | `description` | `snippet` |
| published   | `page_age` / `age` when parseable    | `published_date`               | `publishedDate`                        | `Published:` | `publish_date` | — | `date` |

"Hint in `objective`" means the filter is put into the natural-language goal these tools take;
results outside `site` are dropped by Tollgate either way (§6.1), so `site` is always enforced.
`freshness` and `lang` are best effort for those providers.

What each is good at (the admin puts this in the tool description so the agent can choose):
Brave, a general web index, best for facts, names, news and keyword queries; Tavily, answer-oriented
results with longer clean excerpts, good for direct questions; Exa, semantic search that finds pages
by meaning, good for "pages like this", research and discovery; Parallel, search built for AI
agents that returns long excerpts from each page, good for research questions that need detail;
Firecrawl, a general web index with search operators, good for keyword queries and recent pages;
TinyFish, general web search with publication dates on many results, good for news and recent
events.

---

## 4. `POST /v1/tools/web_fetch`

Request:

```json
{ "url": "https://tr.wikipedia.org/wiki/Kabata%C5%9F_Erkek_Lisesi", "maxChars": 8000, "offset": 0, "find": "1908" }
```

| Field      | Type             | Rules                                                                 |
| ---------- | ---------------- | --------------------------------------------------------------------- |
| `url`      | string, required | absolute `https://` URL, port 443 only, ≤ 2048 characters, no userinfo |
| `maxChars` | integer          | 500–20000, default 8000                                               |
| `offset`   | integer          | ≥ 0, default 0: where the returned window starts in the page markdown |
| `find`     | string           | optional, 1–200 characters; see below                                 |
| `render`   | —                | reserved; ignored in v1                                               |

Response:

```json
{
  "url": "https://tr.wikipedia.org/wiki/Kabata%C5%9F_Erkek_Lisesi",
  "title": "Kabataş Erkek Lisesi",
  "siteName": "Vikipedi",
  "byline": null,
  "published": "2024-11-02T10:00:00Z",
  "lang": "tr",
  "contentType": "text/html",
  "markdown": "# Kabataş Erkek Lisesi\n\nKabataş Erkek Lisesi, …",
  "offset": 0,
  "totalChars": 31250,
  "truncated": true,
  "nextOffset": 8000
}
```

- `url`: the final URL after redirects.
- `title`, `siteName`, `byline`, `published` (ISO 8601), `lang` (from `<html lang>` or meta):
  optional, `null` or absent when unknown. `title` falls back to `<title>`, then the hostname.
- `markdown`: the window `[offset, offset + maxChars)` of the page's markdown (see §6.1, Content). Window
  ends are moved back to the nearest paragraph/line break when one is within the last 20 %.
- `totalChars`: length of the whole markdown. `truncated`: more follows the window.
  `nextOffset`: present iff `truncated`; pass it as `offset` to continue.
- `find`: case-insensitive, accent-sensitive search starting at `offset`. Found → the window
  starts up to 300 characters before the match (at a line start when possible), and the
  response adds `"found": true, "matchOffset": <n>`. Not found → `"found": false`,
  `"markdown": ""`, other fields as usual. Without `find`, `found` is absent.
- Extracted markdown is cached per (key, final URL) for 10 minutes, so paging and `find` don't
  re-download. Requests for the same original URL within that time reuse it.

---

## 5. Errors

Every failure of the three `/v1/tools` routes is:

```json
{ "error": { "code": "blocked_domain", "message": "example.com is blocked in Tollgate's settings" } }
```

`message` is English for logs/debugging; clients show their own text per `code`. Extra fields
may appear in `error` (`status`, `retry_after`).

| HTTP | `code`               | When                                                                      |
| ---- | -------------------- | ------------------------------------------------------------------------- |
| 400  | `invalid_request`    | Bad JSON, missing/out-of-range field, query too long                      |
| 400  | `provider_unavailable` | `provider` names a provider that can't search (Brave without a key, any provider without a key while keyless search is off) or an unknown one |
| 400  | `insecure_url`       | Not `https`, a port other than 443, userinfo in the URL                   |
| 403  | `scope_required`     | The key lacks `web`                                                       |
| 403  | `blocked_domain`     | Host (or a redirect's host) is denied by the domain list / not allowed in allowlist mode |
| 403  | `private_address`    | Host resolves to (or is) a private, loopback, link-local, … address, or is a local name |
| 422  | `too_many_redirects` | More than 3 redirects                                                     |
| 422  | `too_large`          | Body over 5 MB                                                            |
| 422  | `unsupported_type`   | Content type not HTML/XHTML/plain text/markdown (PDF if not supported yet) |
| 422  | `no_content`         | Nothing readable was extracted                                            |
| 502  | `fetch_failed`       | DNS failure, TLS error, connection reset, non-2xx from the site (`status` set) |
| 502  | `search_failed`      | The search provider failed or returned garbage. When it was a rate limit (§3): `status: 429` and `retry_after` |
| 503  | `disabled`           | Web tools switched off in Tollgate                                        |
| 503  | `not_configured`     | `web_search` while no provider can search (keyless search off and no API key) |
| 504  | `timeout`            | Site or provider took longer than 10 s in total                           |
| 429  | `rate_limited`       | Per-key limit hit; `Retry-After` header and `error.retry_after` (seconds) |

Admin mapping: each tool returns `{ ok: false, code, error }` where `error` is Turkish and tells
the model what to do next (e.g. `blocked_domain` → *Bu site Tollgate ayarlarında engellenmiş;
başka bir kaynak kullanın.*). Unknown codes fall back to the message with a generic Turkish
prefix.

---

## 6. Safeguards

### 6.1 Tollgate (server)

Network:

- `https` only, port 443 only, no userinfo. Redirects: at most 3, each target re-checked
  (scheme, port, domain list, address). Redirects to `http` are refused (`insecure_url`).
- Address check in the dialer (`net.Dialer.Control` or a custom `DialContext`), on the IP
  actually connected, so DNS rebinding can't slip past a pre-check. Blocked: loopback, private
  (RFC 1918, `fc00::/7`), link-local (`169.254/16`, `fe80::/10`), CGNAT `100.64/10`,
  `0.0.0.0/8`, `192.0.0/24`, benchmarking `198.18/15`, multicast, reserved `240/4`, broadcast,
  unspecified, IPv4-mapped/NAT64 forms of those, and documentation ranges. Names `localhost`,
  `*.localhost`, `*.local`, `*.internal`, `*.lan`, `*.home.arpa` and bare single-label names are
  refused before resolving. IP-literal hosts are checked the same way.
- Connect directly: ignore `proxy-url` and `HTTP(S)_PROXY` for web fetches, otherwise the
  address check is meaningless. (Search provider calls go only to the fixed provider API hosts,
  `api.search.brave.com`, `api.tavily.com`, `api.exa.ai`, `mcp.exa.ai`, `search.parallel.ai`,
  `mcp.firecrawl.dev` and `agent.tinyfish.ai`, and may use the configured proxy.)
- No cookies (no jar), no `Authorization`, no `Referer`. Request headers: `Accept:
  text/html,application/xhtml+xml,text/plain;q=0.9,*/*;q=0.1`, `Accept-Language` from `lang`
  hint or `tr,en;q=0.8`, and an honest `User-Agent`: `Mozilla/5.0 (compatible; CLI-Tollgate/<version>; +https://github.com/dylancetin/cli-tollgate)`.
- Limits: 10 s total per call (connect + redirects + body), 5 MB body (read through a limited
  reader; decompressed size counts), content types `text/html`, `application/xhtml+xml`,
  `text/plain`, `text/markdown` (PDF text extraction optional; until then `unsupported_type`).
  Charset from header/meta, converted to UTF-8.
- robots.txt is not consulted: each fetch is one page a person asked for through their own
  assistant, like a browser visit. No crawling, no following links server-side.

Domain list (Settings):

- Mode `denylist` (default, empty list) or `allowlist`. Entries are hostnames; an entry matches
  the host and its subdomains (`example.com` matches `a.example.com`, not `badexample.com`).
- Applied to fetch targets, every redirect hop, and search results (dropped silently).

Per-key limits (in memory, sliding one-minute windows): 20 searches/min, 40 fetches/min, and at
most 4 concurrent tool calls per key. Exceeded → `429 rate_limited` with `Retry-After`.

Content:

- Before extraction remove `script`, `style`, `noscript`, `template`, `iframe`, `object`,
  `embed`, `svg`, `canvas`, `form` controls, and elements with `hidden`, `aria-hidden="true"`
  or inline `display:none` / `visibility:hidden`. Then Readability-style main-content
  extraction (`codeberg.org/readeck/go-readability/v2`, the maintained successor of the archived
  go-shiori/go-readability), falling back to `<body>` when it finds nothing.
- HTML → Markdown (JohannesKaufmann/html-to-markdown v2 or equivalent): headings, paragraphs,
  lists, tables, blockquotes, emphasis, links. Links are made absolute against the final URL;
  only `http(s)` links are kept as links (others become plain text). Images become their alt
  text in brackets (`[Görsel: alt]`) or are dropped when alt is empty. No raw HTML in the output.
  Collapse runs of blank lines.
- Logging: method, host and outcome code only, never the full URL query, page text or search
  query. Usage counters may count calls per key.

Settings UI (macOS SettingsView and Windows web UI, same strings in TR):

- Section **Web access**: toggle *Let connected apps search and read the web* (default on; off →
  `disabled`); toggle *Search without an API key* (default on); one optional API key field per
  provider (Brave Search, Tavily, Exa, Parallel, Firecrawl, TinyFish; secure fields, each marked
  *Works without a key* or *Key required* and with a *Get a key* link); a *Default search* picker
  offering *Automatic* and the providers that can search; domain list mode (Block listed sites /
  Allow only listed sites) and a multi-line list.
- Stored in engine `config.yaml` under `web-tools:` and edited through the management API like
  other config (`GET/PUT /v8/management/config/web-tools`), hot-reloaded:

  ```yaml
  web-tools:
    enabled: true
    search:
      default: auto          # auto | brave | tavily | exa | parallel | firecrawl | tinyfish
      keyless: true          # let providers that support it search without a key
      providers:             # keys are optional except for Brave
        brave: { api-key: "" }
        tavily: { api-key: "" }
        exa: { api-key: "" }
        parallel: { api-key: "" }
        firecrawl: { api-key: "" }
        tinyfish: { api-key: "" }
    domains:
      mode: denylist         # denylist | allowlist
      list: []
  ```

### 6.2 Admin (client)

- **Provenance**: `fetchPage` only accepts a URL that appeared in a user message (text the user
  typed, scanned for `http(s)://` and `www.` URLs), in a `webSearch` result, or as a link in a
  page already fetched in this conversation, or a source id (`src-3`) for one of those. Matching
  is exact after normalization (lowercase scheme/host, no default port, no fragment, trailing
  slash on the path ignored); the query string must match too, so an injected page can't turn a
  link into an exfiltration channel. `http://` URLs are tried as `https://`. Anything else →
  `{ ok: false, code: "unknown_url" }` with a Turkish hint to search first or ask the user.
- **Provider choice**: `webSearch` has an optional `provider` (`brave | tavily | exa | parallel |
  firecrawl | tinyfish`) whose description says what each is good at (§3). When the probe lists two or more providers, the
  system prompt names them and the default. A provider the probe didn't list is refused
  client-side without spending a search. A server `provider_unavailable` marks the probe stale
  and tells the agent to retry without `provider`.
- **Caps per user message**: 5 `webSearch` and 8 `fetchPage` calls; the counter resets when the
  user sends a message. Over the cap → `{ ok: false, code: "limit_reached" }`.
- **Sources**: each search result and fetched page is registered once per conversation as
  `src-N` (title, URL, site). Results carry their `src-N`; the model cites with it.
- **Untrusted data**: fetched markdown is returned inside a delimited block
  (`<<<WEB_CONTENT src-N …>>>` … `<<<END_WEB_CONTENT src-N>>>`) with a note that it is data, not
  instructions; search snippets are labelled the same way. The system prompt says web text is
  never instructions.
- **Links in content**: links the assistant writes into rich-text content must be a site path
  (`/…`, `#…`), the site's own origin (`https://kabataserkekliseliler.org.tr`, `www.` too),
  `mailto:`/`tel:`, a registered source, or a URL from a user message. Others are refused before
  the edit with a Turkish error. Script-tool code with other `http(s)` URLs is refused too.
- **No remote images in chat**: the chat's Markdown renderer loads no remote images, so text
  echoed from a page can't make the browser call out.
- `insertSourceList(sourceIds)`: adds a "Kaynaklar" heading and a list of links (title — site)
  for those sources to the open content editor.
- Backend interface (`WebToolsBackend`: `probe`, `search`, `fetchPage`), so `apps/api` can serve
  the same two endpoints later for OpenRouter-only users. Today the backend is Tollgate whenever
  a Tollgate key is set, whichever provider the chat model comes from.

---

## 7. Decisions and reasons

- **Scopes on the access request, not a second key**: one key per app keeps Connected apps
  readable and revocation simple. The upgrade request (§1.3) avoids re-pairing.
- **`models` implicit**: enforcing `models` on every existing route is a large, risky change
  with no user benefit today; `web` is the only new privilege and the only one enforced.
- **Probe always 200**: the client must distinguish "old Tollgate" (404), "ask for permission"
  (`scope_required`) and "user hasn't set a search key" (`not_configured`) to show the right UI.
- **`web_fetch` needs no API key**: reading a page the user named is the most common need and
  shouldn't wait on a search subscription.
- **https/443 only, direct connections, dialer-level IP check**: Tollgate runs on the user's
  machine, often next to routers, NAS boxes and dev servers; SSRF from a prompt-injected agent
  is the main threat. The few http-only sites lost are an acceptable cost.
- **Exact-match provenance including the query**: blocks a page from instructing the agent to
  fetch `https://attacker/?q=<secret>`; the agent can only follow links that literally exist.
- **No headless rendering in v1**: big dependency, larger attack surface; Readability covers
  most article pages. `render` is reserved in the probe and request.
- **Search providers**: Brave, Tavily, Exa, Parallel, Firecrawl and TinyFish. All but Brave answer
  without a key, so search works on a fresh install with zero setup; a key lifts the keyless
  limits. The user picks a default or *Automatic*, and the agent may name a provider per search
  (product decision). The probe lists only providers that can answer, so the agent is never
  offered one that can't.
- **Failover only on rate limits, only when no provider was named** (v1.1, replaces v1's "no
  fallback"): keyless caps are small and hit often, and a `429` means the provider can't answer
  at all, so answering from another provider beats an error. The response's `provider` says who
  answered. Other failures are not retried elsewhere, so quality problems stay visible, and a
  named provider is never swapped. With a key set, keyed providers are tried before keyless ones.
- **Keyless can be switched off**: keyless calls send the query to the provider without an
  account. A user who wants only their own keys used turns *Search without an API key* off.
- **robots.txt ignored**: user-initiated single-page reads, same stance as a browser.

---

## Implementation notes (server, v1)

None of these change what the client sends or expects; they pin down details the contract left open.

- **Libraries**: `github.com/go-shiori/go-readability` was archived in December 2025, so extraction
  uses its maintained fork `codeberg.org/readeck/go-readability/v2` (v2.1.3, same algorithm and
  metadata: title, byline, site name, language, published time from meta tags and JSON-LD).
  HTML → Markdown uses `github.com/JohannesKaufmann/html-to-markdown/v2` (v2.5.2, MIT, maintained)
  with its CommonMark, table and strikethrough plugins.
- **`/access/*` codes**: besides the four codes in §1 (`unknown_scope`, `scopes_already_granted`,
  `key_owned_by_other_app`, `key_already_allowed`), the other refusals now carry a `code` too:
  `invalid_request` (400), `not_loopback` (403), `no_approver` (503), `cooldown` (429),
  `request_pending` (409, with `id`), `too_many_pending` (429). Approving an upgrade whose app was
  revoked meanwhile answers `409 {"code": "grant_revoked"}` on the management route and the request
  becomes `expired`.
- **Upgrade request `scopes`**: the 202 response and the listing's `scopes` are the grant's scopes
  after approval (existing ∪ requested); `added_scopes` are the new ones.
- **Key without a grant**: any authenticated key that has no grant (the user's own keys, keys from
  other access providers) has every scope.
- **`find` not found**: `markdown` is `""`, `offset` is the requested offset, `truncated` is `false`
  and `nextOffset` is absent.
- **`find` window start**: the window may start before the requested `offset` (up to 300 characters
  before the match, moved forward to a line start); `offset` in the response is where it actually
  starts.
- **`web_fetch` `lang`**: optional and not in the client contract; when given (`tr`, `en`, …) the
  `Accept-Language` sent to the site is `<lang>,en;q=0.8` instead of `tr,en;q=0.8`.
- **Rate limits** count calls that passed validation (bad requests cost nothing), cache hits
  included. The concurrency cap answers `429` with `retry_after: 1`.
- **Order of checks**: `scope_required`, then `disabled`, then (search) `not_configured`, then
  request validation, then `provider_unavailable`, then rate limits.
- **Search result hosts**: "private hosts" are IP literals in blocked ranges and local names; result
  host names are not resolved (the fetch re-checks the address when the page is read). Results
  outside `site` are dropped as well, even if the provider returned them. Brave's `search_lang`
  only accepts its own list; `ja` → `jp`, `zh` → `zh-hans`, `pt` → `pt-br`, `no` → `nb`, and
  unsupported languages are not sent.
- **Exa**: `contents.highlights.maxCharacters: 300` (the docs deprecate `numSentences`).
  **Tavily**: `include_published_date: true`, otherwise `published_date` is never returned.
- **Search provider calls** use the system proxy settings (`HTTP(S)_PROXY`), not `proxy-url`.
- **User-Agent version** is the engine's build version with the `tollgate-` prefix removed, which is
  the app version in release builds.
- **Logging**: `web tools: fetch GET <host> -> <code>` and `web tools: search via <provider> -> <code>`.
  The engine's optional request-body log (`request-log`) skips `/v1/tools` entirely.
- **IPv6**: only global unicast (`2000::/3`) is allowed, minus documentation, 6to4, Teredo and other
  `2001::/23` ranges; NAT64 (`64:ff9b::/96`) and IPv4-mapped addresses are checked as their IPv4
  address. If any resolved address is blocked the fetch is refused, even when another is public.
- **Keyless endpoints** (checked live on 2026-11-17 with one real query each, no key): Tavily
  (`X-Tavily-Access-Mode: keyless`, `X-Client-Name: cli-tollgate`; answers `429
  daily_cap_reached` with `Retry-After` once the per-IP cap is spent), Exa
  (`https://mcp.exa.ai/mcp`), Parallel (`https://search.parallel.ai/mcp`), Firecrawl
  (`https://mcp.firecrawl.dev/v2/mcp`) and TinyFish (`https://agent.tinyfish.ai/mcp`,
  `X-TinyFish-Access-Mode: keyless`) all returned results. `TestLiveKeylessSearch` in the engine
  repeats the check (`TOLLGATE_LIVE_SEARCH=1`).
- **MCP providers** are called with a single JSON-RPC `tools/call` POST (`Accept: application/json,
  text/event-stream`); the reply is read as JSON or from SSE `data:` lines, no MCP SDK. Keyed
  TinyFish calls first send `initialize` and `notifications/initialized` for an `Mcp-Session-Id`.
  A tool result with `isError` whose text reads like a rate limit counts as `429`, one that reads
  like a bad key as `401`. Markdown in their excerpts (links, headings, list markers, emphasis) is
  reduced to plain text.
- **Pauses** (rate-limit cooldowns) are in memory, per provider and per API key: setting a new key
  for a paused provider lifts its pause. `Retry-After` may be seconds or an HTTP date; it is
  capped at 24 h. The 10 s limit applies to each provider call, not to the whole failover chain
  (failover only follows a `429`, which comes back fast).
- **Auto mode stickiness** is per Tollgate API key (one per app), in memory, so it resets when the
  engine restarts.
- **Logging** with failover lists the providers tried in order: `web tools: search via
  tavily>exa -> ok`.
- **Rate limits** count a search once, however many providers it tries.

---

## Changes in v1.1

Server (all additive; a v1 client keeps working unchanged):

1. `GET /v1/tools`: `web_search.providers` now lists every provider that **can search** (key or
   keyless), not only those with a key, and may contain `parallel`, `firecrawl` and `tinyfish`.
   On a fresh install it is `["tavily", "exa", "parallel", "firecrawl", "tinyfish"]` and
   `available` is `true` with no setup, where v1 said `not_configured`.
2. `GET /v1/tools`: new `web_search.keyless` (providers answering without a key) and
   `web_search.mode` (`"auto"` | `"default"`).
3. `POST /v1/tools/web_search`: `provider` accepts `parallel`, `firecrawl` and `tinyfish`.
4. `POST /v1/tools/web_search` without `provider`: on a provider `429`/`402` Tollgate fails over
   to another provider; the response's `provider` names the one that answered.
5. `search_failed` caused by rate limits carries `status: 429` and `retry_after` (and a
   `Retry-After` header).
6. `not_configured` now means "keyless search is off and no key is set".

Client (`apps/admin`), to take advantage of it:

- Accept `parallel`, `firecrawl` and `tinyfish` in the `webSearch` tool's `provider` enum and in
  the client-side check against the probe's `providers` (until then they're ignored, which is
  safe: the agent just can't name them, and the default still reaches them).
- Add the three descriptions from §3 to the tool description.
- Show the provider that answered from the response's `provider`, not the probe's `provider`,
  since a failover can change it.
- On `search_failed` with `status: 429`: tell the agent (Turkish) that search providers are busy
  and to wait `retry_after` seconds or try another `provider` from the probe; if the call named a
  `provider`, suggest retrying without it so Tollgate can fail over.
- Optional: in the Connections view, say when search runs keyless (`keyless` non-empty) that the
  free limits are lower and an API key in Tollgate lifts them; update the `not_configured` text
  to "turn on search without a key or add a key in Tollgate".
- Optional: when `mode` is `"auto"`, don't name a default provider in the system prompt as if it
  were fixed.
