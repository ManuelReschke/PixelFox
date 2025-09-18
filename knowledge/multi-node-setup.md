# PixelFox – Lokales Multi‑Node Setup (App + Storage‑Node s01)

Ziel: Das Cross‑VPS/Direct‑to‑Storage Verhalten lokal nachstellen, indem neben der Standard‑App ein zweiter Node (s01) als separater Storage‑Knoten läuft. Datenströme: Direct‑Upload zum jeweiligen Knoten, Rebalancing via HTTP‑Push (`/api/internal/replicate`).

## Voraussetzungen
- Docker + docker‑compose installiert.
- `.env` aus `.env.dev` vorbereitet (`make prepare-env-dev` oder `make start`).
- Secrets gesetzt und identisch für alle Knoten:
  - `UPLOAD_TOKEN_SECRET` (Direct‑Upload)
  - `REPLICATION_SECRET` (HTTP‑Replikation)

## Hosts‑Einträge (für Browser‑Zugriff)
- Linux/macOS: `/etc/hosts`, Windows: `C:\\Windows\\System32\\drivers\\etc\\hosts`
- Ergänzen:
  - `127.0.0.1 app.local`
  - `127.0.0.1 s01.local`

Diese Names sind für die `public_base_url` gedacht (Browser), nicht für interne Container‑Aufrufe.

## Compose‑Override (zweiter Node)
- Im Repo liegt `docker-compose.override.yml`. Es startet einen zweiten App‑Container als Storage‑Node `app_s01` auf Port 8082 und bindet ein separates Uploads‑Verzeichnis.
- Wichtige Details:
  - Service: `app_s01` (Containername `pxlfox-app-s01`)
  - Port: Host `8082` → Container `4000`
  - Env: `NODE_ID=s01`, `DISABLE_JOB_WORKERS=0`
  - Volumes: `./uploads_s01` wird in den Container gemountet (separat zu `./uploads` der Haupt‑App)
- Compose lädt Overrides automatisch. Alternativ gezielt starten: `docker-compose up -d app app_s01`

## Start/Stop
- Start (mit Override): `make start` oder `docker-compose up -d`
- Nur Storage‑Node starten: `docker-compose up -d app_s01`
- Stoppen: `make docker-down` oder `docker-compose down`

## Storage‑Pools konfigurieren (Admin → Speicherverwaltung)
1) Default‑Pool (App)
- `node_id`: `local`
- `base_path`: `/app/uploads`
- `upload_api_url` (intern, Container‑DNS): `http://app:4000/api/internal/upload`
- `public_base_url` (Browser): `http://app.local:8080`

2) Neuer Pool „img1“ (s01)
- `node_id`: `s01`
- `base_path`: `/app/uploads_s01` (wichtig: separater Pfad!)
- `upload_api_url` (intern): `http://app_s01:4000/api/internal/upload`
- `public_base_url` (Browser): `http://s01.local:8082`

Hinweis: `upload_api_url` muss vom Quell‑Container aus erreichbar sein (interne DNS‑Namen `app`/`app_s01`). `public_base_url` ist nur für Links/Downloads vom Browser‑Client relevant.

## Testszenarien
- Direct‑Upload (optional): Admin → Einstellungen → „Direct‑to‑Storage Upload aktivieren“, dann Uploads auf Startseite testen.
- Move to Pool (App → s01): Verschiebe ein Bild vom Default‑Pool zu „img1“. Erwartung: Quellnode streamt zu `http://app_s01:4000/api/internal/replicate`; Logs am Ziel zeigen „Stored file …“.
- Move to Pool (s01 → App): Bild von „img1“ zurück zum Default‑Pool verschieben. Erwartung: HTTP‑Push an `http://app:4000/api/internal/replicate`.

## Logs beobachten
- Haupt‑App: `docker-compose logs -f app --tail 200`
- Storage‑Node: `docker logs -f pxlfox-app-s01 --tail 200`
- Typische Meldungen:
  - `[Replicate] Stored file (pool_id=…, path=…, size=…)` – Transfer ok
  - `[Replicate] Skip existing file …` – Idempotenz (Datei existiert bereits, passende Größe)
  - `[Replicate] Checksum mismatch …` – Integritätsfehler (Ziel löscht Datei, Job retryt)
  - `[MoveImage] Moved image …` – DB‑Update nach erfolgreichem Move
  - `[JobQueue] Recovering stuck job …` – Sweeper holt hängende Jobs zurück

## Direkter Replicate‑Test (optional)
- Beispiel (Linux):
```
curl -X PUT \
  -H "Authorization: Bearer $REPLICATION_SECRET" \
  -F "pool_id=<POOL_ID_VOM_ZIEL>" \
  -F "stored_path=original/2025/08/31/test.jpg" \
  -F "size=$(stat -c%s ./test.jpg)" \
  -F "file=@./test.jpg" \
  http://s01.local:8082/api/internal/replicate
```
- Erfolg: `{\"status\":\"ok\"}`; Existenz‑Skip: `{\"status\":\"ok\",\"skipped\":true,\"reason\":\"exists\"}`

## Troubleshooting
- 401 Unauthorized am Replicate‑Endpoint: `REPLICATION_SECRET` unterschiedlich oder fehlt → in beiden Containern identisch setzen (ENV `.env`).
- „no such file“ beim Move: Quell‑Datei fehlt → Job überspringt non‑fatal; ggf. verwaister DB‑Eintrag.
- 0‑Byte Transfers: Vermeiden, indem die Pools nicht auf denselben `base_path` zeigen. Same‑Path‑Guard ist aktiv, aber trenne Pfade lokal (z. B. `/app/uploads` vs. `/app/uploads_s01`).
- Node‑Routing: Stelle sicher, dass `NODE_ID` je Node korrekt ist und die Pools entsprechend konfiguriert sind. Jobs werden sonst requeued.
- Checksum‑Pflicht: Admin → Einstellungen → „Checksum bei Replikation erzwingen“ (Default EIN). Interne Moves senden SHA‑256 immer mit.

## Hinweise
- Für internen Service‑Traffic (App↔s01) immer Container‑DNS (`app`, `app_s01`) in `upload_api_url` nutzen; für Browser‑Zugriff `public_base_url` (Hosts‑Einträge).
- Der Sweeper zieht Jobs aus `processing` zurück, wenn sie >10 Minuten hängen (Intervall 1 Minute).
- Entferne/benenne `docker-compose.override.yml` um, wenn du den zweiten Node nicht standardmäßig starten möchtest.



Kurz: Dein Link ist falsch. Im Browser gibt es nur den Pfad /uploads/…, nicht /uploads_s01/… (uploads_s01 ist nur ein Verzeichnisname im Container-FS). Außerdem muss der s01‑Container unter /uploads das richtige Verzeichnis ausliefern.

Was du beachten musst

- docker-compose.override.yml
    - Dem s01‑Container fehlt aktuell ein Mount auf /app/uploads (die App servt statisch immer von /uploads → /app/uploads).
    - Ergänze in app_s01.volumes:
        - ./uploads_s01:/app/uploads     (zusätzlich zu ./uploads_s01:/app/uploads_s01)
    - Ergebnis: Dateien, die unter /app/uploads_s01 liegen, sind im Container gleichzeitig unter /app/uploads sichtbar und damit unter /uploads im Browser erreichbar.
    - Ports sind ok: 8082 → 4000.
- Storage‑Pool (Admin → Speicherverwaltung) für s01
    - base_path: weiterhin /app/uploads_s01 (saubere Trennung)
    - upload_api_url: http://app_s01:4000/api/internal/upload (interner Container‑DNS)
    - public_base_url: http://s01.local:8082 (nur Host/Port, KEIN /uploads oder /uploads_s01 anhängen)
    - node_id: s01
- Hosts-Einträge (Browser)
    - 127.0.0.1 app.local
    - 127.0.0.1 s01.local
- Aufruf der Bilder
    - Korrekt: http://s01.local:8082/uploads/original/…/…jpg (oder http://localhost:8082/uploads/original/…)
    - Falsch: http://localhost:8082/uploads_s01/original/… (gibt es als Route nicht)
- Warum 404 kam
    - Die App mountet statisch nur /uploads → /app/uploads. Deine Dateien lagen unter /app/uploads_s01, daher fand der s01‑Container sie nicht. Mit dem zusätzlichen Mount zeigst du /app/uploads auf dieselben Inhalte.
- Checks/Debug
    - Nach Compose‑Änderung neu starten: docker-compose up -d (oder make start)
    - Im s01‑Container prüfen, ob die Datei dort unter /app/uploads/original/… liegt.
    - curl -I http://s01.local:8082/uploads/original/… sollte 200 liefern.

Optionaler Hinweis

- Falls du den Pool lieber auf /app/uploads zeigen lassen willst, kannst du base_path im Pool auf /app/uploads ändern und den zusätzlichen Mount sparen. Mit zwei Knoten lokal ist die getrennte base_path‑Variante (/app/uploads vs. /app/
  uploads_s01) meist übersichtlicher; mit dem Doppelmapping auf /app/uploads ist die Auslieferung sauber gelöst.