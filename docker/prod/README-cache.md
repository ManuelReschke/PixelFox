# PixelFox Cache (Dragonfly) – Production

This guide explains how to run Dragonfly (Redis-compatible) as the cache/session backend on a dedicated VPS.

## Prerequisites
- Docker and Docker Compose plugin installed
- A separate App VPS that will connect on port 6379
- Firewall to restrict 6379/tcp to the App VPS only

## Deploy
```
sudo mkdir -p /srv/dragonfly/data
cd /srv/dragonfly
cp /path/to/repo/docker/prod/cache.compose.yml ./docker-compose.yml
cp /path/to/repo/docker/prod/.env.cache.example ./.env   # optional; not strictly required

docker compose up -d
```

## Enable password authentication (recommended)
Dragonfly supports a Redis-compatible password via `--requirepass`. Edit `docker-compose.yml` and extend the `command`:
```
services:
  cache:
    image: docker.dragonflydb.io/dragonflydb/dragonfly
    command: ["dragonfly", "--cache_mode=true", "--requirepass", "${CACHE_PASSWORD}"]
    env_file:
      - .env
    # ...
```
Then set `CACHE_PASSWORD` in `.env` to a strong random value. The app uses this value via `CACHE_PASSWORD` to authenticate.

## Firewall (UFW example)
```
sudo ufw allow ssh
sudo ufw deny 6379/tcp
sudo ufw allow from <APP_VPS_IP> to any port 6379 proto tcp
sudo ufw enable
```

## Verify
From the App VPS (or allowed host):
```
redis-cli -h <CACHE_HOST> -p 6379 ping
# Expect: PONG
```

## Notes
- Current app configuration does not set a cache password; rely on network isolation/firewall.
- Data persistence for cache is optional; sessions use DB 1, general cache DB 0.
