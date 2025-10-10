# PixelFox Storage & Bildverarbeitung – Architektur & Betrieb

Stand: 2025-09. Diese Datei beschreibt das komplette Storage‑System, den Upload‑Flow (Direct‑to‑Storage), die Bildverarbeitung (Varianten/Thumbnails), S3‑Backups, Replikation/Pool‑Moves sowie relevante Settings und Operatives. Sie ist als Nachschlagewerk für Entwicklung und Betrieb gedacht.

## Überblick

- Storage‑Pools mit Typen `local|nfs|s3`, Tiers `hot|warm|cold|archive`, Prioritäten und Node‑Awareness (Public‑Base‑URL, Upload‑API‑URL, Node‑ID).
- Uploads laufen Direct‑to‑Storage: App vergibt Upload‑Sessions + Token, Client lädt direkt zum gewählten Storage‑Knoten hoch.
- Varianten/Thumbnails werden asynchron per Job‑Queue aus dem Original erzeugt (WebP/AVIF + Größen small/medium). AVIF via ffmpeg/libsvtav1, WebP via go‑webp.
- S3‑Backup ist asynchron (sofort oder verzögert), mit Retry‑Mechanismus und Delete‑Jobs beim Entfernen.
- Replikation/Move: Dateien können zwischen Pools verschoben werden, auch node‑übergreifend via `/api/internal/replicate` mit Secret.

## Datenmodell (Kern)

- `images`: Originaldatei‑Infos inkl. `storage_pool_id`, Dateipfad/‑name, Hash, Dimensionen, Share‑Link.
  - Code: app/models/image.go:14
- `image_variants`: Einträge für erzeugte Varianten/Thumbnails (Typ, Pfad, Größe, Pool‑Bezug).
  - Code: app/models/image_variant.go:23
- `image_metadata`: EXIF/Metadaten als JSON plus ausgewählte Felder (Kamera, Zeit, GPS, etc.).
  - Code: app/models/image_metadata.go:65
- `storage_pools`: Pools mit `storage_type`, `storage_tier`, `priority`, `base_path`, `public_base_url`, `upload_api_url`, `node_id`, S3‑Feldern.
  - Code: app/models/storage_pool.go:27
- `image_backups`: S3‑Backup‑Status je Bild (pending/uploading/completed/failed/deleted) inkl. Bucket/Key/Size/RetryCount.
  - Code: app/models/image_backup.go:30

## Storage‑Pools & Pfad‑Layout

- Pooltypen: `local`, `nfs`, `s3`; Tiers priorisieren Auswahl bei Upload:
  - Hot → Warm → Fallback beliebig (siehe `SelectOptimalPoolForUpload`).
  - Code: app/models/storage_pool.go:453
- Node‑Awareness pro Pool: `public_base_url` (für Downloads), `upload_api_url` (für Direct‑Upload), `node_id` (Routing der Jobs).
  - Code: app/models/storage_pool.go:53
  - Defaults zur Laufzeit gesetzt (PUBLIC_DOMAIN → `public_base_url`; `…/api/internal/upload` → `upload_api_url`; `node_id=local`).
  - Code: internal/pkg/database/setup.go:104
- Pfad‑Layout im Dateisystem (relativ zum Pool‑`base_path`):
  - Original: `original/YYYY/MM/DD/UUID.ext`
  - Varianten: `variants/YYYY/MM/DD/UUID.webp|avif|UUID_small.webp|avif|origExt|UUID_medium.webp|avif|origExt`
  - Code (Erzeugung): internal/pkg/imageprocessor/imageprocessor.go:287

## Upload‑Flow (Direct‑to‑Storage)

1) Upload‑Session anfordern
   - Endpoint: `POST /api/v1/upload/sessions` (API‑Key oder Session)
   - App wählt geeigneten Hot‑Pool (Capacity/Health/Priority) und erstellt HMAC‑Token `{user_id, pool_id, max_bytes, exp}`.
   - Antwort: `{ upload_url, token, pool_id, expires_at, max_bytes }`.
   - Code: app/controllers/api_upload_controller.go:25,84,151,168
   - Token HMAC: internal/pkg/security/upload_token.go:21

2) Direkt hochladen zum Storage‑Knoten
   - Ziel: `POST <upload_url>` (typisch `https://<pool.public_base_url>/api/internal/upload`)
   - Auth: `Authorization: Bearer <token>` oder Feld `token`.
   - Validierungen: IP/User Rate‑Limits (Redis), Plan‑Quota, MIME‑Sniffing+Extension‑Whitelist (JPG, PNG, GIF, WEBP, AVIF, BMP), SHA‑256 für Dedupe‑Lock.
   - Speicherung: `StorageManager.SaveFile(reader, "original/YYYY/MM/DD/UUID.ext", pool.ID)` schreibt direkt unter `base_path` des Pools; Pool‑Usage wird aktualisiert.
   - DB: `images`‑Datensatz mit `storage_pool_id`, Hash, Originalpfad. Duplicate wird früh erkannt und als `duplicate=true` zurückgegeben.
   - Asynchron: `jobqueue.ProcessImageUnified(image)` enqueued Bildverarbeitung; Status landet in Cache.
   - Antwort enthält `image_uuid`, `view_url`, Direkt‑URL auf Original und verfügbare Varianten.
   - Code: app/controllers/storage_upload_controller.go:23,61,120,186,260,406
   - MIME/Whitelist: internal/pkg/upload/validate.go:9

3) Interne Reachability/Health
   - `HEAD /api/internal/upload` antwortet `204` (Health‑Probe).
   - Health‑Monitor cached Pool‑Health/Reachability in Redis.
   - Code: app/controllers/storage_upload_controller.go:15, internal/pkg/storage/health.go:20,96

## Bildverarbeitung (Varianten/Thumbnails)

- Job‑Queue (Redis‑basiert)
  - Payload enthält `image_id`, `image_uuid`, `pool_id`, optional `node_id`. Workeranzahl per Setting konfigurierbar.
  - Node‑Routing: Nur Worker auf passendem Node (`NODE_ID`) verarbeiten Jobs des Pools, sonst Requeue.
  - Sweeper rettet hängende Jobs aus „processing“ zurück nach „pending“.
  - Code: internal/pkg/jobqueue/manager.go:21,47,77; internal/pkg/jobqueue/image_processor.go:22,41

- Status/Progress im Cache
  - Cache‑Keys `image:status:<uuid>` mit TTL je Status; Timestamp separat.
  - Status: `pending|processing|completed|failed`. Abfrage `IsImageProcessingComplete(uuid)`.
  - Code: internal/pkg/imageprocessor/status.go:18,35,84

- Verarbeitungspipeline (Kern)
  - Originalpfad wird aus `image.FilePath/FileName` + Pool‑`base_path` aufgelöst.
  - Metadaten: EXIF auslesen (rwcarlsen/goexif) → `image_metadata` JSON + Felder.
  - Formate/Größen:
    - Optimierte Vollformate: WebP (go‑webp), AVIF (ffmpeg/libsvtav1, nur wenn `ffmpeg` verfügbar und >64×64).
    - Thumbnails: small=200px, medium=500px in WebP/AVIF/Original (je nach Admin‑Toggles, Plan und User‑Prefs via Entitlements).
    - GIF: nur Thumbnails erzeugen, keine „optimierten“ Vollformate.
    - AVIF‑Original‑Input: Dimensions via ffprobe, keine Umkodierung.
  - Varianten‑Records: Für jede erzeugte Datei wird ein Eintrag in `image_variants` angelegt (inkl. `storage_pool_id`).
  - Image‑Record: `width/height` aktualisiert; Metadaten gespeichert.
  - Code: internal/pkg/imageprocessor/imageprocessor.go:214,300,318,491,539,579,652,1107
  - Entitlements: internal/pkg/entitlements/entitlements.go:46

- URL‑Auflösung (Serving)
  - Relativer Web‑Pfad lautet stets `/uploads/...`.
  - Absolute URLs bauen sich aus `image.StoragePool.PublicBaseURL` + Web‑Pfad (sonst Fallback `PUBLIC_DOMAIN`).
  - Konvertierung von absoluten Storage‑Pfaden der Varianten → web‑geeignete Pfade.
  - Code: internal/pkg/imageprocessor/variant_helpers.go:67,121,164
  - Statisches Serving im App‑Process (Dev/Single‑Node): `/uploads` → `./uploads`.
  - Code: cmd/pixelfox/main.go:143

## S3‑Backup (asynchron)

- Aktivierung & Auswahl
  - Globaler Toggle via Settings (`s3_backup_enabled`).
  - Zielpool: bevorzugt `is_backup_target`‑markierter S3‑Pool, sonst S3‑Pool mit höchster Priorität.
  - Code: internal/pkg/jobqueue/unified_processor.go:39, app/models/storage_pool.go:520

- Zeitliche Steuerung
  - Sofort‑Backup, wenn `s3_backup_delay_minutes <= 0`.
  - Sonst nur `image_backups` Record (pending); `delayedBackupWorker` enqueued später Jobs (konfigurierbares Intervall).
  - Retry‑Worker queued fehlgeschlagene Backups erneut (max. RetryCount < 3).
  - Code: internal/pkg/jobqueue/manager.go:94,120; internal/pkg/jobqueue/s3_processor.go:120,252

- Upload/Client
  - Konfig kommt aus Storage‑Pools (AccessKey/Secret/Bucket/Region/Endpoint). Optional Endpoint für S3‑kompatible Anbieter (B2/MinIO), Path‑Style aktiviert.
  - Objektkey‑Schema: `images/YYYY/MM/DD/UUID.ext`.
  - Upload setzt Content‑Type nach Dateiendung und schreibt Metadaten (original‑path, upload‑source).
  - Code: internal/pkg/s3backup/config.go:61, internal/pkg/s3backup/client.go:171

- Tracking
  - `image_backups`: Statusverlauf, Bucket, Objektkey, Size, RetryCount, Fehlermeldung. State‑Übergänge via Methoden.
  - Code: app/models/image_backup.go:62,77,88,95

- Delete‑Flow
  - Beim Löschen eines Bildes enqueued die App S3‑Delete‑Jobs für alle Completed‑Backups.
  - Nach erfolgreichem Löschen aus S3: Backup als `deleted` markieren; falls keine Completed‑Backups mehr existieren → harte Löschung von Variants/Metadata/Image in der DB (Aufräumen, idempotent).
  - Code: internal/pkg/jobqueue/delete_processor.go:26, internal/pkg/jobqueue/s3_processor.go:163,220

Hinweis: `.env` enthält Beispiel‑Variablen für S3‑Backup, wird aber primär über Storage‑Pools konfiguriert.
  - Beispiel: .env.prod:24

## Replikation & Pool‑Moves

- Server‑to‑Server Replikation
  - Endpoint: `PUT /api/internal/replicate` mit `Authorization: Bearer <REPLICATION_SECRET>`.
  - Felder: `pool_id` (Zielpool), `stored_path` (relativer Zielpfad, muss mit `original/` oder `variants/` beginnen), optional `size`, optional `sha256`.
  - Ablauf: Idempotenz (existierende Datei + gleiche Größe → skip), optionaler/erforderlicher Checksum‑Abgleich (Admin‑Setting), danach persistiert unter Ziel‑`base_path`.
  - Code: app/controllers/storage_upload_controller.go:406,520

- Move‑Jobs (Rebalancing)
  - Batch‑Enqueuer scannt Quell‑Pool und enqueued pro Bild einen `move_image` Job.
  - Lokaler Move: via `StorageManager.SaveFile/DeleteFile` (Usage‑Updates inklusive).
  - Remote Move: HTTP‑Push an `/api/internal/replicate` des Ziel‑Pools (Basis `upload_api_url`), bei Erfolg Löschung im Quell‑Pool.
  - DB‑Update atomar: `images.storage_pool_id` + `image_variants.storage_pool_id` → Zielpool.
  - Code: internal/pkg/jobqueue/move_processor.go:1

## Sicherheit & Limits

- Upload‑Token (HMAC, kein JWT): signierte Claims mit TTL (30 min). Verifikation serverseitig; fehlende/ungültige Tokens → 401.
  - Code: internal/pkg/security/upload_token.go:14
- Rate‑Limits: pro IP und zusätzlich pro User (Redis‑Counter, Antwort‑Header zur Diagnose). Limits per Setting konfigurierbar.
  - Code: app/controllers/storage_upload_controller.go:61,180
- MIME‑Sniffing/Whitelist: Nur Bildformate JPG/JPEG, PNG, GIF, WEBP, AVIF, BMP. SVG/XML geblockt (XSS‑Risiko) bis Sanitizer vorhanden.
  - Code: internal/pkg/upload/validate.go:9,19
- Pfad‑Härtung bei Replikation: `stored_path` wird normalisiert, Traversal (`..`) blockiert, gültige Roots nur `original/` oder `variants/`.
  - Code: app/controllers/storage_upload_controller.go:560
- Global: `X-Content-Type-Options: nosniff` auf allen Antworten.
  - Code: cmd/pixelfox/main.go:87

## Health & Monitoring

- Pool‑Health in Redis: `storage_health:<pool_id>` enthält Healthy/Reachable/Usage etc.
- Reachability‑Checks: `OPTIONS`/`HEAD` gegen `upload_api_url` (Prod), Dev‑Fallback auf `http://localhost:<APP_PORT>/api/internal/upload` für `node_id=local`.
- Code: internal/pkg/storage/health.go:20,96,129

## Konfiguration (wichtige Settings/ENV)

- Admin‑Settings (DB‑gestützt):
  - `image_upload_enabled`, `direct_upload_enabled` (Schalter für UI/API)
  - Thumbnail‑Toggles: `thumbnail_original/webp/avif` (global)
  - S3: `s3_backup_enabled`, `s3_backup_delay_minutes`, `s3_backup_check_interval`, `s3_retry_interval`
  - Queue: `job_queue_worker_count`
  - API: `api_rate_limit_per_minute`, Upload: `upload_rate_limit_per_minute`, `upload_user_rate_limit_per_minute`
  - Replikation: `replication_require_checksum`
  - Code: app/models/setting.go:20

- ENV (Beispiele):
  - `PUBLIC_DOMAIN`, `APP_HOST/APP_PORT`
  - `UPLOAD_TOKEN_SECRET` (erforderlich für Direct‑Upload), `REPLICATION_SECRET` (Server‑to‑Server)
  - Optional S3‑ENV (Fallback); primär über Storage‑Pools pflegen.
  - Beispiel: .env.prod:24,33

## API‑Referenz (Storage‑relevant)

- Public v1:
  - `POST /api/v1/upload/sessions` → Upload‑Session (Token, Upload‑URL, Limits)
  - `GET /api/v1/images/:uuid` → Bild‑Ressource
  - `GET /api/v1/images/:uuid/status` → Polling ob Verarbeitung abgeschlossen
  - Code: internal/api/v1/generated.go:402

- Internal:
  - `POST /api/internal/upload` → Direct‑Upload (mit Token)
  - `HEAD /api/internal/upload` → Healthcheck
  - `PUT  /api/internal/replicate` → Replikationsziel (Secret‑geschützt)
  - Router: internal/pkg/router/api_router.go:73

## Operative Hinweise & „Gotchas“

- ffmpeg muss im PATH sein, sonst kein AVIF (wird automatisch erkannt, Logs warnen).
  - Code: internal/pkg/imageprocessor/imageprocessor.go:60,720
- go‑webp nutzt CGO; sicherstellen, dass Build‑Umgebung CGO für WebP aktiviert (siehe Commit‑Historie).
- Bei Multi‑Node: `NODE_ID` auf Storage‑Nodes setzen und optional `DISABLE_JOB_WORKERS=1` auf reinen Storage‑Nodes, bis Routing/Workers klar sind.
- `public_base_url` pro Pool korrekt pflegen. CDN davor möglich; URL‑Aufbau bleibt stabil.
- Varianten‑Pfad in DB ist ein Speicherpfad (teilweise absolut). Für Web‑URLs immer Helper (`GetImageURL/GetImageAbsoluteURL`) nutzen.
- Löschpfad: Delete‑Jobs räumen Dateien+DB auf; bei vorhandenen Backups bleiben DB‑Records „soft‑deleted“, bis S3‑Deletes durch sind.

## Schnelltest (lokal)

1. `.env.dev` prüfen: `UPLOAD_TOKEN_SECRET`/`REPLICATION_SECRET` gesetzt. `make start`.
2. Admin → Storage: Standard‑Pool prüfen (Defaults kommen aus `PUBLIC_DOMAIN`).
3. `POST /api/v1/upload/sessions` → `upload_url` + `token` → `POST /api/internal/upload` (Multipart `file`).
4. `GET /api/internal/images/:uuid/status` oder `GET /api/v1/images/:uuid/status` → warten bis `complete=true`.
5. URLs aus Response testen (Original/Varianten), AVIF nur bei ffmpeg vorhanden.

