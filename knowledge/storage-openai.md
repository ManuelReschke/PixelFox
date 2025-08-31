# PixelFox Storage Scaling – Hot Storage über mehrere VPS

Stand: 2025-08-31

Ziel: Mehrere kleine VPS als Hot‑Storage-Knoten betreiben, App/DB/Redis separat. Uploads sollen horizontal skaliert werden, Downloads effizient vom richtigen Storage‑Knoten bedient werden – angelehnt an Sibsoft/XFileSharing (Upload‑Server Auswahl + Weiterleitung).

## Kurzfazit (Empfehlung)

- Beste Option: Direct‑to‑Storage nach Sibsoft‑Muster.
  - Zentrale App wählt einen aktiven Hot‑Storage‑Knoten und gibt dem Client eine Upload‑URL + Token zurück.
  - Downloads verlinken direkt auf die Public‑Base‑URL des Storage‑Pools (oder via CDN), nicht über den App‑Knoten.
- Alternativ (nicht empfohlen für Wachstum): App als Proxy für Upload/Download – einfach, aber Bandbreite/CPU der App werden zum Bottleneck.

## Ausgangslage im Code

- `internal/pkg/storage/manager.go`: schreibt Dateien in `StoragePool.BasePath` (lokales/NFS FS).
- `app/models/storage_pool.go`: Pools mit Typ (`local|nfs|s3`), Tier (`hot|warm|…`), Priorität. S3‑Backup ist bereits integriert.
- `app/controllers/image_controller.go#HandleUpload`: wählt Hot‑Pool und schreibt direkt unter `BasePath/original/YYYY/MM/DD/UUID.ext`.
- `internal/pkg/imageprocessor`: verarbeitet auf Basis lokaler Pfade aus dem Pool. Web‑Pfad wird aktuell zu `/uploads/...` normalisiert (gleiches Host‑Origin).

Konsequenz: In Multi‑VPS‑Setups müssen Upload/Processing/Serving „knotenbewusst“ werden (Pfad gehört zu einem bestimmten Pool/Host).

## Architekturvorschlag

1) Storage‑Knoten als erste Klasse
- Jeder VPS kann optional „APP“ und/oder „STORAGE“ Rolle ausführen. Logisch trennen, auch wenn beides auf einer VM läuft.
- Für jeden Storage‑Knoten existiert mind. ein Hot‑Storage‑Pool.

2) Neue Felder im `StoragePool` (DB)
- `public_base_url` (string): Öffentliche Basis‑URL des Pools, z. B. `https://s01.pixelfox.cc`.
- `upload_api_url` (string): Interne/öffentliche Upload‑API des Storage‑Knotens, z. B. `https://s01.pixelfox.cc/api/internal/upload`.
- `node_id` (string): Logischer Knotenname/ID, z. B. `s01` (für Scheduling, Health, Logs).
- Optional: `region` (string), `weight` (int), `capacity_mb` (int), `is_upload_target` (bool), `health_status`.

3) Upload‑Flow (Direct‑to‑Storage)
- Client → `POST /api/v1/upload/sessions` (App): App prüft User, Dateigröße, wählt Hot‑Pool per Strategie (least‑used/priority/weight).
- App erstellt Upload‑Session (DB/Redis) und signiert ein Token: `{user_id, pool_id, expires_at, max_size}` (HMAC/EdDSA).
- Response: `{upload_url, token, pool_id, session_id, expires_at}`; `upload_url = pool.upload_api_url`.
- Client lädt direkt zum Storage‑Knoten hoch: `POST upload_url` mit `token` + Daten (Multipart/Chunked/Resumable).
- Storage‑Knoten verifiziert Token, schreibt Datei unter seinen `BasePath`, erzeugt Callback (App‑intern) oder sendet Metadaten zurück; App legt Image‑Datensatz an (inkl. `storage_pool_id`).
- App enqueued Job in Redis‑Queue (siehe Scheduling unten).

4) Download‑Flow
- App/Views generieren absolute URLs basierend auf `pool.public_base_url`:
  - Original: `https://sNN.pixelfox.cc/uploads/original/YYYY/MM/DD/UUID.ext`
  - Varianten: `https://sNN.pixelfox.cc/uploads/variants/YYYY/MM/DD/UUID.webp|avif|…`
- Optional: CDN vor alle `public_base_url`s schalten (ein Origin per Pool oder ein zentrales CDN mit Host‑Header‑Weiterleitung).
- Für private/temporäre Inhalte: signierte, kurzlebige URLs (HMAC + Expires) oder 302‑Redirects von der App.

5) Verarbeitung/Scheduling (Job Queue)
- Jobs enthalten `image_id`, `pool_id`, optional `node_id`.
- Worker starten nur auf Storage‑Knoten und verarbeiten Jobs, deren `pool_id` zu „ihrem“ Knoten gehört.
  - Variante A (einfach): Jeder Worker filtert auf `node_id`/`pool_id` und holt nur „eigene“ Jobs (Queue‑Key pro Node oder Claim‑Check vor Start).
  - Variante B (sauber): Separate Redis‑Queues pro Node, App enqueued direkt in die passende Node‑Queue.
- Verarbeitung liest Original aus lokalem `BasePath` des Pools und schreibt Varianten ebenfalls dort. S3‑Backup bleibt unverändert asynchron.

6) Health/Discovery
- Jeder Storage‑Knoten meldet sich per Heartbeat (DB/Redis) mit `free_space`, `used_pct`, `iops`, `active_uploads`, `health_status`.
- App berücksichtigt Health/Capacity bei der Pool‑Auswahl (Backoff, Weight, Region‑Affinity).

7) Replikation/HA (optional, schrittweise)
- Replikationsfaktor (RF=2): Zweitschreibungen auf einen zweiten Hot‑Pool; „best‑effort“ asynchron via Queue.
- Rebalancing: Bei Knoten‑Zuwachs/Entfall Jobs erzeugen, die Dateien verschieben und DB‑Referenzen aktualisieren.
- S3 bleibt als kaltes Sicherheitsnetz (Backup/Restore, Cross‑Region).

## Vergleich der Optionen

- App als Proxy (Upload/Download durch App):
  - + Einfach zu implementieren
  - – App wird BW/CPU‑Bottleneck, schlechtere Latenz, höherer Traffic‑Preis

- Direct‑to‑Storage (empfohlen):
  - + Lineare Skalierung pro Storage‑VPS, App bleibt Control‑Plane
  - + Bessere Latenz, CDN‑freundlich, weniger Doppel‑Traffic
  - – Benötigt Token‑Flows/Health/Scheduling und URL‑Generierung pro Pool

## Minimale Code‑Änderungen (Roadmap)

1) Datenmodell
- `StoragePool` um Felder erweitern: `PublicBaseURL`, `UploadAPIURL`, `NodeID` (+ Migration).

2) API
- `GET /api/v1/upload/server` oder `POST /api/v1/upload/sessions` → gibt `upload_url`, `token`, `pool_id`, `expires_at` zurück (Sibsoft‑Pattern; siehe `knowledge/sibsoft.md`).
- `POST /api/internal/upload` (auf Storage‑Knoten): validiert Token, speichert Datei, antwortet mit Metadaten, optional Callback zur App.

3) Controller/Views
- `image_controller.HandleUpload`: wahlweise alten Pfad (App‑Proxy) oder neuen Direct‑Upload aktivieren (Feature‑Flag). Im neuen Pfad nur Session ausstellen, nicht mehr schreiben.
- `imageprocessor.GetImageURL`/`BuildImagePaths`: statt hartem `/uploads/...` Host aus `pool.public_base_url` prefixen; Fallback auf `PUBLIC_DOMAIN` bleibt erhalten.

4) Job Queue
- Payload um `pool_id`/`node_id` erweitern. Worker ziehen nur passende Jobs (Node‑Queue oder Filter).

5) Ops/Infra
- DNS: `s01, s02, ...` Subdomains auf jeweilige VPS zeigen lassen.
- Nginx auf Storage‑Knoten: `location /uploads/ { alias <BasePath>/; }` + CORS/Cache‑Header; `location /api/internal/upload` für Upload‑Endpoint.
- Optional CDN vor `sNN.*` schalten.

## Schrittweise Einführung

Phase 1 (1–2 Wochen)
- Felder + Migration, Health‑Heartbeat, `public_base_url` in URL‑Generierung, Downloads direkt vom Pool. Upload weiterhin via App (Kompatibilität).

Phase 2 (2–4 Wochen)
- Upload‑Session API + Storage‑Upload‑Endpoint, Direct‑to‑Storage aktivieren. Worker an `pool_id` binden, einfache Node‑Queue.

Phase 3 (4–8 Wochen)
- RF>1 Replikation, Rebalancing‑Jobs, Region/Weight‑Aware Selection, signierte Download‑URLs, CDN‑Integration.

## Warum diese Lösung gut zu PixelFox passt

- Nutzt das bestehende Storage‑Pool‑Konzept (Hot/Warm/Cold) und erweitert es um Host‑/URL‑Bewusstsein.
- Redis‑Queue existiert bereits – nur Job‑Routing pro Node/Poll nötig.
- S3‑Backup bleibt unverändert und erhöht die Ausfallsicherheit.
- Jeder zusätzliche VPS erhöht Upload‑Durchsatz, Storage und Verarbeitungsleistung linear.

## Hinweise

- Für kleine Setups kann ein VPS gleichzeitig App+Storage laufen – die Logik bleibt identisch (Direct‑to‑Storage zeigt dann auf denselben Host).
- NFS/Gluster als „Shared FS“ sind möglich, aber auf kleinen VPS oft fragil/aufwendig. Direct‑to‑Storage mit URL‑Routing ist robuster und günstiger.

## TODO Checklist (Implementierung)

- [x] StoragePool um Felder erweitern (`public_base_url`, `upload_api_url`, `node_id`).
- [x] Helper im Imageprocessor für Base‑URL/Absolute‑URLs (`GetPublicBaseURLForImage`, `MakeAbsoluteURL`, `GetImageAbsoluteURL`).
- [x] Controller auf absolute Storage‑URLs umstellen (erste Seiten):
  - [x] `image_controller` (Show + AJAX Status: Domain/OG/Preview/Original absolut)
  - [x] `album_controller` (Galerie‑Previews/Original absolut)
  - [x] `user_controller` (Meine Bilder + weitere Seiten: Previews/Original absolut)
- [x] Health‑Heartbeat der Storage‑Nodes (Cache/Redis: used, max, used_pct, healthy, timestamp).
- [x] Admin UI Felder für `public_base_url`/`upload_api_url`/`node_id` bearbeiten (Form + Controller Save/Edit).
- [ ] Download‑URL‑Signierung (optional) für private/temporäre Inhalte.
- [x] Upload‑Session API (`/api/v1/upload/sessions`) + Storage‑Upload‑Endpoint (`/api/internal/upload`).
- [x] Worker‑Routing pro Node (Filter via `NODE_ID` + Pool.NodeID; Requeue bei Mismatch).

Phase 2 Hinweise (Stand jetzt)
- Storage‑Knoten brauchen `UPLOAD_TOKEN_SECRET` und sollten `DISABLE_JOB_WORKERS=1` setzen, bis Worker‑Routing implementiert ist.
- Admin → Storage Pool: `upload_api_url` muss gesetzt sein (z. B. https://s01.pixelfox.cc/api/internal/upload).
- Client Flow: `POST /api/v1/upload/sessions` (mit Session) → `upload_url` + `token`; danach `POST upload_url` (Multipart `file` + `Authorization: Bearer <token>`).

Beispiel (curl)
1) Upload‑Session vom App‑Server anfordern (eingeloggt, Cookie/Session senden):
```
curl -s -X POST \
  -H "Content-Type: application/json" \
  -H "Cookie: session=YOUR_SESSION_COOKIE" \
  -d '{"file_size": 1234567}' \
  https://app.pixelfox.cc/api/v1/upload/sessions

# Antwort (Beispiel)
{
  "upload_url": "https://s01.pixelfox.cc/api/internal/upload",
  "token": "<SIGNED_TOKEN>",
  "pool_id": 3,
  "expires_at": 1724940000
}
```

2) Direkt zum Storage‑Server hochladen:
```
curl -s -X POST \
  -H "Authorization: Bearer <SIGNED_TOKEN>" \
  -F "file=@/path/to/image.jpg" \
  https://s01.pixelfox.cc/api/internal/upload

# Antwort (Beispiel)
{
  "image_uuid": "f2c2c0e1-...",
  "view_url": "/i/ABCDEFG"
}
```

Notizen (Phase 1 Defaults)
- [x] Default‑Pool: Runtime‑Defaults anwenden, falls Felder leer (`PUBLIC_DOMAIN` → `public_base_url`, `…/api/internal/upload` → `upload_api_url`, `local` → `node_id`).

## Phase 2 Roadmap – Nächste Schritte

- Sicherheit/Validierung
  - Einheitliche MIME/Extension‑Checks im Storage‑Upload (serverseitig) – implementiert inkl. MIME‑Sniffing; optional strengere Erkennung via Lib (z. B. mimetype) oder Image‑Decoding.
  - Rate‑Limit pro User zusätzlich zum IP‑Limit (Einstellung in Admin). Optional Burst/Leaky‑Bucket.
- UX und Feedback
  - Dezente “Verarbeitung läuft …” Anzeige (Poll auf JSON‑Status) – umgesetzt; Feinschliff möglich.
  - Bessere Fehlertexte aus Storage‑Upload (z. B. Dateityp/Größe), ggf. Mapping in Flash‑Routen.
- Admin/Config
  - Kurzer Hinweis in Admin‑Einstellungen, dass `UPLOAD_TOKEN_SECRET` gesetzt sein muss (Validator/Warning).
  - Übersicht der aktiven Knoten (NODE_ID, Erreichbarkeit des `upload_api_url`) mit Heartbeat.
- Technik/Optimierung
  - Optional: Resumable/Chunked Upload (große Dateien, Wiederaufnahme).
  - Optional: Signierte Download‑URLs (private/temporäre Links).
  - Optional: CDN‑Konfiguration/Auto‑Rewrite in Templates für statische Auslieferung.

---

## Phase 2 – Implementierungsstand (2025‑08‑31)

Erreicht (fertig):
- Direct‑to‑Storage Upload end‑to‑end
  - `POST /api/v1/upload/sessions` (App) gibt `upload_url` + signiertes Token aus.
  - `POST /api/internal/upload` (Storage) validiert Token und speichert Datei im gewählten Pool.
  - Frontend: Direct‑Upload mit XHR‑Progress, Polling auf `/api/v1/image/status/:uuid` bis fertig.
- Worker‑Routing pro Node (bereit)
  - Jobs werden anhand von `NODE_ID`/`pool.node_id` gefiltert; bei Mismatch Requeue.
- URL‑Generierung (Downloads) node‑bewusst
  - `imageprocessor` prefixed URLs mit `pool.public_base_url`, Fallback `PUBLIC_DOMAIN`.
- Health/Heartbeat
  - Pools werden alle 60s in Redis gecached (Used/Max/Usage%, Healthy, Timestamp).
  - Reachability‑Check für `upload_api_url` (OPTIONS/HEAD, 2s Timeout) wird als `upload_api_reachable` im Cache gespeichert.
  - Dev‑Fallback: Für `node_id=local` wird zusätzlich `http://localhost:<APP_PORT>/api/internal/upload` geprüft (0.0.0.0 ist in Dev sonst nicht erreichbar).
- Admin UI
  - Storage‑Pools: Felder `public_base_url` / `upload_api_url` / `node_id` verwaltbar; Tabelle zeigt Node, Public/Upload‑URL und Badge “API OK/Fehler”.
  - Hinweis auf fehlendes `UPLOAD_TOKEN_SECRET` in den Einstellungen.
  - Rate‑Limits konfigurierbar (siehe unten).
 - Rebalancing/Move to Pool (v0):
   - Batch‑Enqueuer `pool_move_enqueue` plant je Bild einen `move_image` Job (200er Batches).
   - Node‑Routing: Ausführung auf dem Quell‑Node; bei Mismatch Requeue.
   - Pfad‑Normalisierung für Varianten (verhindert doppelte Prefixe, z. B. `/app/uploads/...`).
   - Same‑Path‑Guard: Falls Quell‑ und Zielpfad identisch auflösen (z. B. lokaler Ein‑Ordner‑Test), wird IO übersprungen (kein Truncate/Copy/Delete).
   - Fehlende Quelle: Non‑fatal Skip ohne Retries; Varianten einzeln skip‑bar.
   - Stuck‑Job‑Recovery: Sweeper requeued Jobs, die >10 Min in `processing` hängen (Intervall 1 Min).
 - Cross‑VPS Replikation (HTTP Push):
   - Ziel‑Endpoint: `PUT /api/internal/replicate` (Secret‑geschützt via `REPLICATION_SECRET`).
   - Move‑Job erkennt Remote‑Ziele (abweichende `node_id`) und streamt Dateien per Multipart an `…/replicate` (Basis: `upload_api_url`).
   - Idempotenz: Ziel prüft Existenz+Größe; bei Match → Skip.
   - Integrität: SHA‑256 wird vom Sender mitgeschickt und am Ziel gestreamt validiert; Mismatch → Datei löschen + 422.
   - Checksum‑Pflicht ist über Admin‑Setting steuerbar (Default: EIN).
   - Logging am Ziel: Unauthorized/Traversal/Capacity/Skip/Mismatch/Success mit Kontext (pool_id, path, size, IP).
- Rate‑Limits
  - IP‑basiert: bestand bereits, bleibt aktiv.
  - Neu: Per‑User‑Limit (Redis‑Zähler `rate:upload:user:<user_id>`, 60s TTL) – konfigurierbar in Admin Settings.
- Fehlermeldungen (Direct Upload)
  - Flash‑Routen für Rate‑Limit (`/flash/upload-rate-limit`), zu große Dateien (`/flash/upload-too-large`), nicht unterstützte Typen (`/flash/upload-unsupported-type`) und generische Upload‑Fehler (`/flash/upload-error?msg=...`).
  - Frontend mappt HTTP‑Fehler (429/413/415/sonstige) auf diese Routen, damit Benutzer sofort eine sichtbare Meldung sehen.
- Sicherheit/Validierung (neu)
  - MIME‑Sniffing serverseitig mit `http.DetectContentType` plus Extension‑Whitelist.
  - SVG/XML werden blockiert (XSS‑Risiko) bis Sanitizer verfügbar ist.
  - Global `X-Content-Type-Options: nosniff` gesetzt.
- Stabilität/Regression Fix
  - Der Stats‑Job aktualisiert nur noch `used_size` per `UpdateColumn`, überschreibt keine anderen Pool‑Felder (z. B. `is_active`) mehr.

Offen (optional / Feinschliff):
- Strengere MIME‑Erkennung (z. B. Lib mimetype, Image‑Decoding) und ggf. SVG‑Sanitizer falls SVG erlaubt werden soll.
- Resumable/Chunked Uploads für sehr große Dateien und Wiederaufnahme.
- Signierte Download‑URLs für private/temporäre Inhalte.
- CDN‑Integration und ggf. automatisches URL‑Rewrite in Templates.

Dev/ENV Hinweise (lokal):
- Pflicht für Direct‑Upload: `UPLOAD_TOKEN_SECRET` (App/Storage verwenden denselben Secret). In `.env.dev` ist es gesetzt.
- Replikation (HTTP Push): `REPLICATION_SECRET` muss auf allen Knoten identisch gesetzt sein (App + Storage‑Nodes).
- `PUBLIC_DOMAIN` (Dev default: `http://0.0.0.0:8080`). Für korrekten Reachability‑Check alternativ `http://localhost:8080` setzen, oder den implementierten Dev‑Fallback nutzen.
- Optional: `NODE_ID` (Single‑Node nicht nötig) und `DISABLE_JOB_WORKERS` (nur für reine Storage‑Nodes).
- Admin Settings:
  - `upload_rate_limit_per_minute` (pro IP, Default 60)
  - `upload_user_rate_limit_per_minute` (pro Benutzer, Default 60)
  - `replication_require_checksum` (Default TRUE) – SHA‑256 Validierung am Ziel verpflichtend

Kurzanleitung – Direct‑to‑Storage Test (lokal):
1. `.env.dev` → `UPLOAD_TOKEN_SECRET` gesetzt, `make start`.
2. Admin → Einstellungen → “Direct‑to‑Storage Upload aktivieren”.
3. Upload auf der Startseite testen; bei Fehlern erscheinen Flash‑Meldungen.

---

## Admin Pool Move (Rebalancing v0) – Stand

Ziel: Einen Pool manuell leeren, indem alle Bilder (inkl. Varianten) in einen anderen Pool verschoben werden.

UI/Bedienung
- Button “Move to” in Admin → Speicherverwaltung pro Pool.
- Formular: Quelle wird angezeigt, Zielpool auswählen (nur aktive, nicht identisch mit Quelle).
- Empfehlung: Quell‑Pool vorher deaktivieren (keine neuen Uploads/Verarbeitungen), dann Move starten.

Ablauf/Jobs
- Start POST `/admin/storage/move/:id` enqueued einen Batch‑Enqueuer `pool_move_enqueue`.
- `pool_move_enqueue`:
  - Liest Bilder des Quell‑Pools in Batches (200), `id`‑aufsteigend, ab Cursor.
  - Für jedes Bild enqueued ein `move_image`.
  - Requeued sich selbst mit neuem Cursor, bis alle Bilder geplant sind.
- `move_image` (je Bild):
  - Node‑Routing: Job läuft auf dem Quell‑Node (`NODE_ID` vs. `sourcePool.NodeID`); bei Mismatch Requeue.
  - Kopiert Original und alle Varianten:
    - Lokal/NFS: via StorageManager (`SaveFile` → Zielpool, danach `DeleteFile` im Quellpool).
    - Remote (anderer Node): HTTP‑Push an `PUT /api/internal/replicate` des Ziel‑Pools (Basis `upload_api_url`), bei Erfolg Löschen im Quell‑Pool.
  - Same‑Path‑Guard: Identische Quell/Ziel‑Pfadauflösung → IO wird übersprungen.
  - Fehlende Quelle/Varianten: Non‑fatal Skip ohne Retries.
  - Aktualisiert DB‑Referenzen atomar: `images.storage_pool_id` und `image_variants.storage_pool_id` → Zielpool.
  - Kapazität/`used_size` wird durch `SaveFile/DeleteFile` mitgepflegt.

Pfad‑Normalisierung (Fix)
- Varianten speicherten `FilePath` als absoluten Pfad (inkl. Storage‑Base). Beim Verschieben wird der Pfad vor dem Öffnen normalisiert:
  - Primär ab `variants/...` extrahiert, sonst Storage‑Base‑Prefix entfernt → relativer Pfad für `SaveFile/DeleteFile`.
- Damit wird ein doppeltes Präfix wie `/app/uploads/app/uploads/variants/...` vermieden (Fehler “no such file”).

Health/Reachability (Ergänzung)
- HEAD `/api/internal/upload` antwortet jetzt 204; der Healthcheck nutzt dies, sodass keine 405‑Logs mehr entstehen.
- Storage‑Pool Formular: Hinweis, dass Replikation auf Basis von `upload_api_url` über `/api/internal/replicate` läuft (Secret‑geschützt).

Monitoring/Operative Hinweise
- Fortschritt: Admin → Queues zeigt anstehende/verarbeitete Jobs. (Eigene Fortschritts‑UI kann bei Bedarf ergänzt werden.)
- Throttling/Last: Standard‑Queue‑Workeranzahl über Settings steuerbar.
- Fehlerfälle: Jobs besitzen Retry‑Mechanismus; dauerhafte Fehler erscheinen im Queue‑Monitor.

Nächste Schritte (Rebalancing v1)
- Fortschritts‑UI mit Zähler (geplant/abgearbeitet/fehlerhaft) und Pause/Resume.
- Pre‑Flight‑Checks (Ziel‑Kapazität, Reachability), optional Dry‑Run/Plan.
- Datenintegrität: Checksums (SHA‑256) nach Copy auch für lokale Moves; optional Audit/Verifier.
- Throttling/Rate‑Limit speziell für Move‑Jobs; Tageszeitfenster.
