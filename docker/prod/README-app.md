# PixelFox App (Production)

This guide explains how to deploy the PixelFox application container on a dedicated VPS, fronted by a reverse proxy with TLS. Development docker-compose.yml remains unchanged.

## Prerequisites
- Docker and Docker Compose plugin installed
- Domain points to this VPS (A/AAAA record)
- Reverse proxy (Nginx/Caddy/Traefik) for HTTPS
- Separate DB and Cache hosts reachable from this VPS (firewalled)

## Prepare environment
1) Create app directories
```
sudo mkdir -p /srv/pixelfox/{uploads,tmp}
sudo chown -R $USER:$USER /srv/pixelfox
```

2) Place Compose file and env next to it
```
cd /srv/pixelfox
cp /path/to/repo/docker/prod/app.compose.yml ./docker-compose.yml
cp /path/to/repo/docker/prod/.env.app.example ./.env
# Edit .env to set DB_HOST (DB VPS IP), CACHE_HOST (Cache VPS IP), secrets, SMTP, hCaptcha, etc.
```
Required .env keys (non-exhaustive): `PUBLIC_DOMAIN, APP_ENV, DB_HOST, DB_NAME, DB_USER, DB_PASSWORD, CACHE_HOST, CACHE_PORT, UPLOAD_TOKEN_SECRET, REPLICATION_SECRET, SMTP_*`.
If your cache is password-protected, also set: `CACHE_PASSWORD`.

3) Choose image tag
```
# Option A: put APP_IMAGE inside .env
# Option B: export it in your shell (example shown):
export APP_IMAGE=registry.example.com/pixelfox/app:<TAG>
```

## Start the app
```
docker compose up -d
```
The app listens on 127.0.0.1:4000 (local-only). Use a reverse proxy for public HTTPS.

## Reverse proxy (example: Caddy)

Minimal Caddyfile:
```
pixelfox.cc {
  encode zstd gzip
  reverse_proxy 127.0.0.1:4000
}

www.pixelfox.cc {
  redir https://pixelfox.cc{uri} permanent
}
```
Caddy provisions TLS automatically with Let's Encrypt. Place this file at `/etc/caddy/Caddyfile` (or use a custom path) and reload Caddy.

Global options (optional) and hardened headers example:
```
{
  email admin@your-domain
}

pixelfox.cc {
  encode zstd gzip
  header {
    Strict-Transport-Security "max-age=31536000; includeSubDomains; preload"
    X-Frame-Options "DENY"
    X-Content-Type-Options "nosniff"
    Referrer-Policy "strict-origin-when-cross-origin"
    Permissions-Policy "accelerometer=(), autoplay=(), camera=(), display-capture=(), document-domain=(), encrypted-media=(), fullscreen=*, geolocation=(), gyroscope=(), magnetometer=(), microphone=(), midi=(), payment=(), usb=(), interest-cohort=()"
  }
  reverse_proxy 127.0.0.1:4000
  log {
    output file /var/log/caddy/pixelfox.access.log
    format console
    level INFO
  }
}

www.pixelfox.cc {
  redir https://pixelfox.cc{uri} permanent
}
```

DNS challenge with Cloudflare (optional; useful behind restrictive firewalls):
```
{
  email admin@your-domain
  acme_dns cloudflare {env.CLOUDFLARE_API_TOKEN}
}

pixelfox.cc {
  reverse_proxy 127.0.0.1:4000
}
```
Run Caddy with `CLOUDFLARE_API_TOKEN` in the environment. See Caddy docs for the Cloudflare DNS module.

## Reverse proxy (example: Nginx)
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
Use certbot (or Caddy/Traefik) to provision TLS certificates.

## Database migrations
Prefer SQL migrations or run from a dev machine as documented in `knowledge/go-live.md` (section: Database Migrations). The app also performs GORM AutoMigrate on startup.

## Health check
- `curl -f https://pixelfox.cc/api/v1/ping` should return `{ "ping": "pong" }`

## Update / Rollback
- Update:
```
export APP_IMAGE=registry.example.com/pixelfox/app:<NEW_TAG>
docker compose pull || true
docker compose up -d
```
- Rollback:
```
export APP_IMAGE=registry.example.com/pixelfox/app:<PREV_TAG>
docker compose up -d
```

## Logs
```
docker logs -f pxlfox-app
```

## Firewall (UFW suggestion)
- Baseline inbound policy (keep 4000 local-only):
```
sudo ufw default deny incoming
sudo ufw default allow outgoing
sudo ufw allow OpenSSH    # 22/tcp
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
# 4000/tcp is bound to 127.0.0.1 by compose and not exposed externally
sudo ufw enable
```

- Optional: tighten egress so the app host only reaches DB + Cache and essentials:
```
sudo ufw default deny outgoing
# Allow DB and Cache communications
sudo ufw allow out to <DB_VPS_IP> port 3306 proto tcp
sudo ufw allow out to <CACHE_VPS_IP> port 6379 proto tcp
# Allow DNS + NTP + HTTPS (package updates, certbot) + HTTP if needed
sudo ufw allow out 53/tcp
sudo ufw allow out 53/udp
sudo ufw allow out 123/udp
sudo ufw allow out 443/tcp
sudo ufw allow out 80/tcp
```
Adjust the IPs to your DB/Cache hosts. If you use additional services (e.g., object storage, SMTP), add corresponding egress rules.
