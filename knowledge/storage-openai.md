# PixelFox Storage Scaling – Hot Storage über mehrere VPS

Stand: 2025-08-29

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
  - Einheitliche MIME/Extension‑Checks im Storage‑Upload (serverseitig) – Basis erledigt; optional echte MIME‑Sniffing.
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
