# Go-Live Plan: App + Storage `s1` (+ optional DB VPS)

Stand: 2026-02-27  
Status: Entwurf fuer ersten produktiven Rollout

## Zielbild

### Empfohlen (3 VPS)
- VPS `app`: PixelFox App + Reverse Proxy (Caddy/Nginx)
- VPS `s1`: Storage-Node (eigene App-Instanz fuer Upload/Replication + Datei-Auslieferung)
- VPS `data`: MySQL + Dragonfly (Cache)

### Minimal (2 VPS)
- VPS `app`: App + Proxy + MySQL + Dragonfly
- VPS `s1`: Storage-Node
- Nachteil: DB/Cache teilen sich Ressourcen mit der App.

## Wichtige Regeln vorab

- `CACHE_HOST`/Redis-kompatibler Cache ist Pflicht fuer Sessions/Queue.
- `UPLOAD_TOKEN_SECRET` muss auf allen Nodes gleich sein, die `/api/internal/upload` annehmen.
- `REPLICATION_SECRET` muss auf App und `s1` identisch sein.
- Jeder Storage-Pool braucht korrekte `public_base_url` und `upload_api_url`.
- Fuer produktive Stabilitaet zuerst Uploads auf App-Hot-Storage lassen; `s1` als Warm/Cold-Ziel nutzen.

## Netzwerk- und DNS-Plan

Beispiel:
- `pixelfox.cc` -> Proxy auf VPS `app`
- `images-s1.pixelfox.cc` -> Proxy auf VPS `s1` (Serving aus Pool `s1`)

Firewall-Minimum:
- App VPS: `22`, `80`, `443` offen; App-Container bleibt auf `127.0.0.1:4000`
- Data VPS: `3306` nur von App- und Storage-IP, `6379` nur von App- und Storage-IP
- Storage VPS: `22`, `80`, `443` offen

## Option 1 (empfohlen): Caddy + Cloudflare (im Detail)

Ja, in diesem Setup braucht jeder oeffentlich erreichbare Host einen Reverse Proxy:
- VPS `app` fuer `pixelfox.cc`
- VPS `s1` fuer `images-s1.pixelfox.cc`

Grund:
- `docker/prod/app.compose.yml` bindet die App auf `127.0.0.1:4000`.
- Cloudflare alleine kann nicht auf `127.0.0.1` zugreifen.
- Caddy terminiert TLS auf `:443` und proxyt intern auf `127.0.0.1:4000`.

### Cloudflare-Konfiguration

1. DNS-Records anlegen:
   - `A`/`AAAA` `pixelfox.cc` -> IP von VPS `app` (Proxy aktiviert, orange cloud)
   - `A`/`AAAA` `images-s1` -> IP von VPS `s1` (Proxy aktiviert, orange cloud)
2. SSL/TLS Mode auf `Full (strict)` setzen.
3. Optional:
   - `Always Use HTTPS = on`
   - Caching-Bypass-Regeln fuer dynamische Endpunkte (`/api/*`, `/admin/*`).

### Caddy auf beiden Hosts

1. Auf VPS `app`:
   - App deployen (`scripts/deploy_app_stack_interactive.sh`)
   - Proxy deployen (`scripts/deploy_proxy_stack_interactive.sh`) mit:
     - `domain=pixelfox.cc`
     - `backend=127.0.0.1:4000`
2. Auf VPS `s1`:
   - Storage deployen (`scripts/deploy_storage_stack_interactive.sh`)
   - Proxy deployen (`scripts/deploy_proxy_stack_interactive.sh`) mit:
     - `domain=images-s1.pixelfox.cc`
     - `backend=127.0.0.1:4000`

### Verifikation (Pflicht)

Von extern:
- `curl -I https://pixelfox.cc`
- `curl -I https://images-s1.pixelfox.cc/api/internal/upload`

Auf den Hosts:
- `docker logs -f pxlfox-proxy`
- `docker logs -f pxlfox-app`

Erwartung:
- HTTPS antwortet mit 200/204 (kein TLS-Fehler).
- Keine Exponierung von Port `4000` nach extern.

### Typische Fehler vermeiden

- Nicht nur Cloudflare einschalten und Caddy weglassen (fuehrt bei diesem Compose zu Origin-Fehlern).
- `REPLICATION_SECRET` auf `app` und `s1` exakt gleich halten.
- Im Admin fuer Pool `s1` exakt setzen:
  - `public_base_url=https://images-s1.pixelfox.cc`
  - `upload_api_url=https://images-s1.pixelfox.cc/api/internal/upload`

## Phase 1: Data VPS (MySQL + Dragonfly)

Siehe auch:
- `docker/prod/README-db.md`
- `docker/prod/README-cache.md`

Schritte:
1. MySQL deployen (`docker/prod/db.compose.yml` + `.env.db.example`).
2. Dragonfly deployen (`docker/prod/cache.compose.yml`).
3. Firewall auf Source-IP-Whitelist setzen (nur App/Storage).
4. Verbindung testen:
   - `mysql -h <DB_HOST> -u pixelfox -p pixelfox_db -e 'SELECT 1;'`
   - `redis-cli -h <CACHE_HOST> -p 6379 ping`

Alternative bei nur 2 VPS:
- DB + Cache auf VPS `app` mit denselben Compose-Dateien deployen.
- In der App-`.env` dann `DB_HOST` und `CACHE_HOST` auf die private IP von VPS `app` setzen.
- Firewall so setzen, dass `3306` und `6379` nur von `app` selbst und VPS `s1` erreichbar sind.

## Phase 2: App VPS deployen

Siehe auch:
- `docker/prod/README-app.md`
- `docker/prod/README-proxy.md`

Schritte:
1. Verzeichnisse anlegen:
   - `/srv/pixelfox/uploads`
   - `/srv/pixelfox/tmp`
2. `docker/prod/app.compose.yml` nach `/srv/pixelfox/docker-compose.yml` kopieren.
3. `.env` aus `docker/prod/.env.app.example` erstellen und anpassen:
   - `PUBLIC_DOMAIN=https://pixelfox.cc`
   - `DB_HOST=<data-vps-ip>`
   - `CACHE_HOST=<data-vps-ip>`
   - starke Secrets fuer `UPLOAD_TOKEN_SECRET`, `REPLICATION_SECRET`
4. App starten:
   - `docker compose up -d`
5. Proxy (Caddy/Nginx) deployen und TLS aktivieren.
6. Smoke-Test:
   - `curl -f https://pixelfox.cc/api/v1/ping`

## Phase 3: Storage VPS `s1` deployen

`s1` nutzt dieselbe App-Compose-Datei, aber mit eigener `.env` und eigenem Upload-Volume.

Schritte:
1. Verzeichnisse anlegen:
   - `/srv/pixelfox-s1/uploads`
   - `/srv/pixelfox-s1/tmp`
2. `docker/prod/app.compose.yml` nach `/srv/pixelfox-s1/docker-compose.yml` kopieren.
3. `.env` auf `s1` erstellen (aus `docker/prod/.env.app.example`) und setzen:
   - `PUBLIC_DOMAIN=https://images-s1.pixelfox.cc`
   - `DB_HOST=<data-vps-ip>`
   - `CACHE_HOST=<data-vps-ip>`
   - `UPLOAD_TOKEN_SECRET=<gleich wie app>`
   - `REPLICATION_SECRET=<gleich wie app>`
   - `NODE_ID=s1`
   - `DISABLE_JOB_WORKERS=1` (Startempfehlung fuer reinen Storage-Node)
4. Optional in `.env`:
   - `UPLOADS_DIR=/srv/pixelfox-s1/uploads`
   - `TMP_DIR=/srv/pixelfox-s1/tmp`
5. Starten:
   - `docker compose up -d`
6. Proxy auf `s1` deployen (Domain `images-s1.pixelfox.cc` -> `127.0.0.1:4000`).
7. Erreichbarkeit pruefen:
   - `curl -I https://images-s1.pixelfox.cc/api/internal/upload`

## Phase 4: Storage-Pools im Admin konfigurieren

Im Admin-Bereich:

1. Bestehenden Hot-Pool auf App-Node als Default behalten.
2. Neuen Pool `s1` anlegen:
   - `storage_type=local`
   - `storage_tier=warm` (oder `cold`, je nach Ziel)
   - `base_path=/app/uploads`
   - `public_base_url=https://images-s1.pixelfox.cc`
   - `upload_api_url=https://images-s1.pixelfox.cc/api/internal/upload`
   - `node_id=s1`
   - `is_default=false`
3. In Settings:
   - `replication_require_checksum=true`
   - Tiering erst aktivieren, wenn End-to-End Tests erfolgreich sind.

## Database Migrations

Empfohlen: aus separatem Admin-/CI-Checkout (nicht im lokalen Dev-Stack) ausfuehren.

1. Produktions-`.env` im Repo-Root bereitstellen (mit `DB_*` auf Produktiv-DB).
2. Migrationsstatus pruefen:
   - `go run ./cmd/migrate/main.go status`
3. Migrationen ausfuehren:
   - `go run ./cmd/migrate/main.go up`
4. Status erneut pruefen:
   - `go run ./cmd/migrate/main.go status`

Hinweise:
- Vor Migrationen ein DB-Backup erstellen.
- App macht beim Start auch AutoMigrate, aber SQL-Migrationen bleiben der primäre Weg.

## Go-Live Tag (Runbook)

1. Wartungsfenster festlegen, DNS TTL vorab auf z. B. 300s setzen.
2. Finales DB-Backup ziehen.
3. App + `s1` mit finalem `APP_IMAGE` starten.
4. Migrationen laufen lassen (siehe oben).
5. Smoke-Tests:
   - Login/Register
   - Upload einer Testdatei
   - `GET /api/v1/images/:uuid/status` bis `complete=true`
   - Testbild via `public_base_url` abrufen
6. Erst danach Tiering/Move-Jobs aktivieren.
7. Monitoring der ersten 24h:
   - App-Logs, `s1`-Logs, DB-Errors, Queue-Backlog, 4xx/5xx-Raten

## Rollback-Plan

Wenn etwas schieflaeuft:
1. Uploads kurzfristig deaktivieren (Admin-Setting `image_upload_enabled=false`).
2. Auf vorheriges `APP_IMAGE` zurueckrollen.
3. `s1`-Pool voruebergehend deaktivieren (`is_active=false`), falls Storage-Replikation Fehler wirft.
4. Bei DB-Problemen auf letztes Backup wiederherstellen.
5. Nach Stabilisierung Uploads wieder aktivieren.

## Betrieb nach Go-Live

- Updates:
  - `scripts/deploy_app.sh` fuer App-Rollouts nutzen.
- Regelmaessig:
  - DB-Backups + Restore-Tests
  - Logrotation und Error-Review
  - Security-Updates fuer Docker-Images (App, MySQL, Dragonfly, Proxy)

## Kurz-Checkliste

- [ ] DNS + TLS fuer `pixelfox.cc` und `images-s1.pixelfox.cc` stehen
- [ ] Cloudflare SSL/TLS steht auf `Full (strict)`
- [ ] DB + Cache laufen und sind per Firewall begrenzt
- [ ] App laeuft stabil auf VPS `app`
- [ ] Storage-Node `s1` erreichbar (`/api/internal/upload`)
- [ ] `UPLOAD_TOKEN_SECRET` und `REPLICATION_SECRET` sind auf allen Nodes identisch
- [ ] Pool `s1` korrekt im Admin angelegt
- [ ] Migrationen erfolgreich (`status` ohne Fehler)
- [ ] Upload/Processing/Serving End-to-End erfolgreich getestet
