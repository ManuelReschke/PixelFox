# PixelFox Reverse Proxy (Caddy) – Production

This guide runs Caddy as a reverse proxy in a container on the App VPS. It assumes the app container listens on 127.0.0.1:4000 (see app.compose.yml).

## Why host networking?
We use `network_mode: host` so Caddy can:
- Bind to ports 80/443 directly
- Proxy to `127.0.0.1:4000`, which targets the app container bound on the host loopback

## Deploy
```
sudo mkdir -p /srv/caddy/{data,config}
cd /srv/caddy
cp /path/to/repo/docker/prod/Caddyfile.example ./Caddyfile
```
Edit `Caddyfile` and set your domain (e.g., pixelfox.cc). Minimal content:
```
pixelfox.cc {
  encode zstd gzip
  reverse_proxy 127.0.0.1:4000
}
```

Start Caddy via Compose:
```
cd /srv/caddy
cp /path/to/repo/docker/prod/proxy.compose.yml ./docker-compose.yml
export CADDY_EMAIL=admin@your-domain   # optional, used for ACME registration
docker compose up -d
```

## Verify
```
curl -I https://pixelfox.cc
```
You should see HTTP/2 200 with Caddy/auto TLS.

## DNS-01 challenge (Cloudflare) – optional
The official `caddy:2` image does not include DNS provider plugins by default. To use DNS-01, build a custom image with the Cloudflare plugin:

Dockerfile (example):
```
FROM caddy:2-builder AS builder
RUN xcaddy build \
  --with github.com/caddy-dns/cloudflare

FROM caddy:2
COPY --from=builder /usr/bin/caddy /usr/bin/caddy
```

Build and use this image in `proxy.compose.yml` instead of `caddy:2`, then set:
```
export CLOUDFLARE_API_TOKEN=...   # and reference it in your Caddyfile global block if needed
```
Refer to the Caddy DNS plugin docs for exact Caddyfile syntax.

## Logs & maintenance
```
docker logs -f pxlfox-proxy
docker compose restart
```

