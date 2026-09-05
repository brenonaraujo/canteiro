# Canteiro Swarm stack

Public product host: `https://canteiro.brenon.cloud`

## Processes

| Service | Image | Listen | Role |
|---------|-------|--------|------|
| `canteiro` | `ghcr.io/brenonaraujo/canteiro:${SERVICE_TAG:-latest}` | :8080 | Gin API |
| `web` | `ghcr.io/brenonaraujo/canteiro-web:${SERVICE_TAG:-latest}` | :3000 | Nuxt SPA |
| `edge` | `nginx:1.27-alpine` | :80 published **18083** | same-origin router |
| `postgres` | `postgres:18.4-alpine` | :5432 | DB |
| `migrate` | `migrate/migrate:v4.19.1` | one-shot | SQL up |

Placement: `canteiro` / `web` / `edge` / `migrate` on `node.labels.vserver == true` (fedora). `postgres` stays on the manager (volume). Tunnel origin remains `http://192.168.1.101:18083` via Swarm ingress mesh.

Restart: `condition: any` on long-running processes (not `on-failure`). A clean SIGTERM / node bounce with `on-failure` never reschedules — public host then returns Cloudflare **502 / 530 / error 1033**. `migrate` stays `none`.

Kong path `https://api.brenon.cloud/canteiro` (key-auth) stays LAN/server-to-server. This stack does **not** change that plugin.

## Tunnel (already published)

Explicit hostname **above** the wildcard:

```
canteiro.brenon.cloud  →  http://192.168.1.101:18083
*.brenon.cloud         →  http://192.168.1.101:19080   (haas-edge)
catch-all              →  http_status:404
```

DNS CNAME `canteiro` → `5ea9935b-fac5-4161-a6b0-6c1afaf4bce3.cfargotunnel.com` (proxied).

Do **not** CNAME this host to `api.brenon.cloud`. After this stack update the tunnel origin stays `:18083`; the process bound there becomes the edge (was API-only).

## Edge routing

| Path | Upstream |
|------|----------|
| `/` and other SPA routes (`/_nuxt`, `/auth/login`, `/account/listings`, …) | `web:3000` |
| `/healthz` `/readyz` `/metrics` | `canteiro:8080` |
| `/catalog` `/owner` `/rentals` `/payments` `/damage` `/debt` `/users` | `canteiro:8080` |
| `/auth/google*` `/auth/logout` | `canteiro:8080` |
| `/account` `/account/deactivate` (exact) | `canteiro:8080` |
| `/listings*` browser navigation (`Sec-Fetch-Dest: document`) | `web:3000` (catalog HTML) |
| `/listings*` curl / `fetch` (no document dest) | `canteiro:8080` (JSON) |

`NUXT_PUBLIC_API_BASE=""` — the SPA talks same-origin. Catalog data uses `/catalog/listings` (always API). Owner list uses `/listings` via `fetch`.

## Google OAuth + session (runtime only)

Google is a backing service (F3/F4). The API process does **not** require it to start.

| Env | Where | Notes |
|-----|--------|-------|
| `GOOGLE_CLIENT_ID` | Portainer stack env | empty in git |
| `GOOGLE_CLIENT_SECRET` | Portainer stack env | empty in git; never paste in issues |
| `GOOGLE_REDIRECT_URL` | stack default | **must** match the OAuth client: `https://canteiro.brenon.cloud/auth/google/callback` |
| `SESSION_SECRET` | Portainer stack env | empty in git; **≥ 16 bytes** (HMAC CSRF state) |
| `WEB_APP_URL` | stack | `https://canteiro.brenon.cloud` |
| `SESSION_COOKIE_SECURE` | stack | `true` on the public host |

Copy `deploy/swarm/.env.example` → `deploy/swarm/.env` (gitignored) for a local `docker stack deploy`. Leave Google/session empty to boot without the provider.

OAuth client (Google Cloud → Web application):

- Authorized JavaScript origin: `https://canteiro.brenon.cloud`
- Authorized redirect URI: `https://canteiro.brenon.cloud/auth/google/callback`

### Smoke

Use **GET**, not `curl -I` / HEAD. Smoke only counts on the public HTTPS host.

```bash
curl -sS -A 'Mozilla/5.0' -o /dev/null -w '%{http_code} %{content_type}\n' https://canteiro.brenon.cloud/
# 200 text/html — title Canteiro (not CF 502/530/1033)

curl -sS -A 'Mozilla/5.0' https://canteiro.brenon.cloud/healthz
# 200 {"service":"canteiro","status":"ok"}

curl -sS -A 'Mozilla/5.0' https://canteiro.brenon.cloud/catalog/listings
# 200 application/json (same-origin API, not CF HTML)

curl -sSI --max-redirs 0 https://canteiro.brenon.cloud/auth/google
# with backing: 302 Location https://accounts.google.com/o/oauth2/v2/auth?...
# without:      503 JSON not_configured (SPA must show auth.not_configured)
```

Cloudflare **1033** = tunnel connector down (`cloudflared-tunnel` Swarm service), not a missing DNS CNAME. **502/530** with the connector up = origin `:18083` unreachable (dead edge task or broken ingress mesh). Do not CNAME this host to `api.brenon.cloud`.

Rollback of the backing: remove `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET` from Portainer stack env and update the stack. `/healthz` stays 200; login start returns 503 again.

## Rollback

1. Remove `web` + `edge` from the stack.
2. Publish `canteiro` `:8080` → host **18083** again.
3. Leave tunnel/DNS/Kong as they are.
4. Public `/healthz` stays 200; `/` returns Gin `404 page not found` (status quo of #21).
