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

## Rollback

1. Remove `web` + `edge` from the stack.
2. Publish `canteiro` `:8080` → host **18083** again.
3. Leave tunnel/DNS/Kong as they are.
4. Public `/healthz` stays 200; `/` returns Gin `404 page not found` (status quo of #21).
