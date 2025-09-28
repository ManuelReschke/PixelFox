# PixelFox Go‑Live Plan (App, DB, Cache on separate VPS)

This document describes a pragmatic production rollout of PixelFox on three VPS hosts: one for the app, one for the database (MySQL 8), and one for the cache (Dragonfly/Redis‑compatible). It covers network layout, secrets, image build/publish, host provisioning, migrations, reverse proxy/TLS, and operations.

## Goals
- Public HTTPS site at your domain (e.g., https://pixelfox.cc)
- Separate VPS per role: App, DB, Cache
- Private networking or firewall isolation between nodes
- Reproducible deploy using container images
- Persisted data and backups

## Reference Architecture
- App VPS
  - Runs the PixelFox container (built from `docker/golang/Dockerfile`)
  - Reverse proxy (Nginx/Caddy/Traefik) terminates TLS and forwards to app on port 4000
  - Persistent directories: `uploads/`, `tmp/` (mounted as host volumes)
  - Outbound access to DB:3306 and Cache:6379

- DB VPS (MySQL 8.4)
  - Port 3306 only reachable from App VPS
  - Persistent data volume, regular backups

- Cache VPS (Dragonfly or Redis)
  - Port 6379 only reachable from App VPS
  - Ephemeral data is fine; can persist if desired

Recommended private addresses (example):
- App: 10.0.0.30
- DB: 10.0.0.10 (MySQL 3306)
- Cache: 10.0.0.20 (Redis/Dragonfly 6379)

## DNS and Certificates
- `A` for apex or `www` -> App VPS public IP
- Optional storage subdomain(s) if using separate storage nodes later (e.g., `s01.pixelfox.cc`)
- Use Let’s Encrypt via your reverse proxy (Nginx + certbot, Caddy automatic TLS, or Traefik)

## Security Baseline
- Create a non‑root user, disable password SSH, enable UFW/iptables
- App VPS: allow 22/tcp, 80/tcp, 443/tcp; block 4000 externally
- DB VPS: allow 22/tcp; allow 3306/tcp only from App VPS
- Cache VPS: allow 22/tcp; allow 6379/tcp only from App VPS
- Cache authentication: Set a password on Dragonfly (`--requirepass`) and provide it to the app via `CACHE_PASSWORD`. Restrict port 6379 via firewall to App VPS only.

## Environment and Secrets
Start from `.env.prod` and adjust for multi‑host:

```
PUBLIC_DOMAIN=https://pixelfox.cc
APP_ENV=prod
APP_HOST=0.0.0.0
APP_PORT=4000

# Point to remote DB + Cache
DB_HOST=10.0.0.10
DB_NAME=pixelfox_db
DB_USER=pixelfox
DB_PASSWORD=strong_db_password

CACHE_HOST=10.0.0.20
CACHE_PORT=6379

# Critical secrets – set strong random values
UPLOAD_TOKEN_SECRET=change_this_secret
REPLICATION_SECRET=change_this_replication_secret

# hCaptcha / SMTP (production values)
HCAPTCHA_SECRET=...
HCAPTCHA_SITEKEY=...
SMTP_HOST=smtp.mailgun.org
SMTP_PORT=587
SMTP_USERNAME=...
SMTP_PASSWORD=...
SMTP_SENDER=postmaster@your-domain
```

Notes
- MySQL port is currently hard‑coded to 3306 in code; keep default port.
- `PUBLIC_DOMAIN` is used to initialize storage pool defaults (public base and internal upload URL) on first run.

## Build and Publish Image
Build the production image once, then push to your registry (GitLab/GHCR/ACR/ECR):

```
export IMAGE=registry.example.com/pixelfox/app
export TAG=$(date +%Y%m%d-%H%M)
docker buildx build \
  -f docker/golang/Dockerfile \
  -t $IMAGE:$TAG -t $IMAGE:latest \
  --platform linux/amd64 \
  --push .
```

## Provision Hosts
Templates are included in the repo for production Compose per host (each with a README for step-by-step deployment):
- App: `docker/prod/app.compose.yml`
- DB: `docker/prod/db.compose.yml`
- Cache: `docker/prod/cache.compose.yml`
- Proxy (Caddy): `docker/prod/proxy.compose.yml`

Example .env templates per host:
- App: `docker/prod/.env.app.example`
- DB: `docker/prod/.env.db.example`
- Cache: `docker/prod/.env.cache.example`
Proxy uses `Caddyfile` (see `docker/prod/Caddyfile.example`).

Place a suitable `.env` next to each file on its host (or use `--env-file`).

### DB VPS (MySQL 8.4)
Use `docker/prod/db.compose.yml` on the DB host:

```
cd /srv/mysql
cp /path/to/repo/docker/prod/db.compose.yml ./docker-compose.yml
cp /path/to/repo/.env.prod ./.env   # or create a minimal .env with DB_* vars
docker compose up -d
```

Backups: nightly `mysqldump` to encrypted storage and/or volume snapshots.

### Cache VPS (Dragonfly)
Use `docker/prod/cache.compose.yml` on the cache host:

```
cd /srv/dragonfly
cp /path/to/repo/docker/prod/cache.compose.yml ./docker-compose.yml
touch .env  # optional; not strictly required for cache
docker compose up -d
```

To enable a password, modify the cache compose to add `--requirepass` and set `CACHE_PASSWORD` in its `.env`:
```
services:
  cache:
    image: docker.dragonflydb.io/dragonflydb/dragonfly
    command: ["dragonfly", "--cache_mode=true", "--requirepass", "${CACHE_PASSWORD}"]
    env_file: [".env"]
```
Then add `CACHE_PASSWORD` to the App `.env` as well.

If you prefer Redis, deploy `redis:7` similarly. Given the app does not set a cache password, rely on network isolation.

### App VPS
Use `docker/prod/app.compose.yml` on the app host:

```
mkdir -p /srv/pixelfox/{uploads,tmp}
cd /srv/pixelfox
cp /path/to/repo/docker/prod/app.compose.yml ./docker-compose.yml
cp /path/to/repo/.env.prod ./.env
export APP_IMAGE=registry.example.com/pixelfox/app:<TAG>
docker compose up -d
```

3) Reverse Proxy + TLS
- Nginx example (`/etc/nginx/sites-available/pixelfox`):

```
server {
  listen 80;
  server_name pixelfox.cc;
  location / { return 301 https://$host$request_uri; }
}

server {
  listen 443 ssl http2;
  server_name pixelfox.cc;
  ssl_certificate /etc/letsencrypt/live/pixelfox.cc/fullchain.pem;
  ssl_certificate_key /etc/letsencrypt/live/pixelfox.cc/privkey.pem;
  location / {
    proxy_pass http://127.0.0.1:4000;
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
  }
}
```

Enable site, reload Nginx, obtain/renew certificates with certbot or use Caddy/Traefik for simpler TLS.

## Database Migrations
There are SQL migrations (golang‑migrate) and also GORM AutoMigrate. For production, prefer the SQL migrations to control schema.

Options:
- From your workstation (or CI) with network to DB:
  ```
  docker run --rm -it \
    -v "$PWD":/app -w /app \
    -e DB_HOST=10.0.0.10 -e DB_NAME=pixelfox_db \
    -e DB_USER=pixelfox -e DB_PASSWORD=strong_db_password \
    golang:1.25-alpine sh -lc 'apk add --no-cache build-base && go run cmd/migrate/main.go up'
  ```
- Or temporarily run the app in dev mode container (has Go toolchain) on the App VPS and execute `make migrate-up` pointing to the remote DB.

After first app start, defaults for the storage pool are applied from `PUBLIC_DOMAIN` (public base + internal upload url), see `internal/pkg/database/setup.go`.

## Smoke Tests
- `curl -f https://pixelfox.cc/api/v1/ping` → `{ "ping": "pong" }`
- Web register/login flow works
- Upload a small image in the UI; verify it appears and thumbnails render
- Check `/api/internal/upload` via the UI direct‑to‑storage flow (session issued, upload accepted)

## Backups and Disaster Recovery
- DB: nightly `mysqldump` + weekly volume snapshots; test restore
- App: stateless; keep image tags and `.env` in a secure repo
- Files: back up `/srv/pixelfox/uploads` (or use S3 via the built‑in S3 backup client – configure in `.env.prod`)

## Monitoring and Ops
- System: CPU, memory, disk usage, inode usage, IO wait
- App: `docker logs -f pixelfox`; reverse proxy logs; optional Prometheus/Grafana
- MySQL: slow query log; exporter (optional)
- Alerts: uptime checks for `GET /api/v1/ping` and main site

## Scaling Later
- App: run multiple app containers behind the same reverse proxy; sessions already use Redis (DB 1)
- Storage: add dedicated storage nodes; configure storage pools (public base URL + upload API); use replication secret for server‑to‑server copy
- Cache: scale vertically; Redis/Dragonfly handles session and cache
- DB: use managed MySQL or add read replicas (app is mostly write‑light)

## Deployment Workflow (CI/CD Suggestion)
GitHub Actions and GitLab CI templates are included. Use the one matching your host.

- GitHub Actions: `.github/workflows/docker-build.yml` builds and pushes `docker/golang/Dockerfile` to GHCR as `ghcr.io/<owner>/<repo>:<SHA>` and `:latest`.
- GitLab CI: `.gitlab-ci.yml` builds and pushes to GitLab's Container Registry as `$CI_REGISTRY_IMAGE/app:<SHA>` and `:latest`.

Deploy (manual on App VPS):
- With GHCR tag:
  - `bash scripts/deploy_app.sh -f /srv/pixelfox/docker-compose.yml -e /srv/pixelfox/.env -i ghcr.io/<owner>/<repo>:<SHA>`
- With GitLab tag:
  - `bash scripts/deploy_app.sh -f /srv/pixelfox/docker-compose.yml -e /srv/pixelfox/.env -i $CI_REGISTRY_IMAGE/app:<SHA>`

Verify `GET /api/v1/ping` and a test login/upload.

Rollback: rerun deploy with previous tag.

Optional: proxy updates via helper script:
- `bash scripts/deploy_proxy.sh -f /srv/caddy/docker-compose.yml -E CADDY_EMAIL=admin@your-domain`

## Notes and Caveats
- Cache auth: current code doesn’t read a cache password; isolate the cache by firewall/VPC
- MySQL port is fixed to 3306 in code; keep default or adjust code to honor `DB_PORT`
- Ensure `UPLOAD_TOKEN_SECRET` and `REPLICATION_SECRET` are strong and different
- Set `CookieSecure` in session middleware if you terminate TLS directly on the app in the future
