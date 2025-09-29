# Multi‑Upload (Premium) – Umsetzungsplan

Ziel: Für Nutzer mit Plan „premium“ und „premium_max“ Multi‑Upload automatisch aktivieren. Nutzer können mehrere Bilder gleichzeitig auswählen/hochladen. Nach Abschluss landet man auf einer übersichtlichen Ergebnis‑Seite mit Mini‑Vorschau je Bild, rechts daneben Share‑Link + „Bearbeiten“‑Button. Diese Seite ist ephemer – verlässt man sie, kann man nicht zurück (Single‑Use/TTL).

## Anforderungen (Akzeptanzkriterien)
- Premium/Premium‑Max sehen auf der Startseite Multi‑Select/Drag&Drop für mehrere Dateien; Free bleibt Single‑Upload.
- Parallel-/Serien‑Uploads mit Fortschritt je Datei (min. pro Datei, optional Gesamtfortschritt).
- Direkte Uploads (Direct‑to‑Storage) werden für jede Datei separat abgewickelt; Fallback auf App‑Upload möglich.
- Ergebnis‑Seite: Liste aller (neu) hochgeladenen Bilder – je Eintrag: kleine Vorschau, Share‑Link (kopierbar), „Bearbeiten“‑Button.
- Doppelte Bilder (Duplicate‑Detection) tauchen als „bereits vorhanden“ mit existierendem Link auf.
- Ergebnis‑Seite ist ephemer: Einmal anzeigen (oder TTL), danach 404/Redirect auf „Meine Bilder“.
- Nichts am bestehenden Single‑Upload/Viewer kaputt machen; weiterhin erreichbar.

## Entitlements/Feature‑Flag
- internal/pkg/entitlements/entitlements.go
  - Neue Funktion `func CanMultiUpload(plan Plan) bool` → true für `premium` und `premium_max`, false für `free`.
  - Optional: `MaxFilesPerBatch(plan Plan) int` (z. B. 20 für Premium, 50 für Premium‑Max, 3–5 für Free wenn später erlaubt).

## Frontend (Startseite Upload‑UI)
- views/home.templ
  - Wenn `CanMultiUpload(plan)` → `input#file-input` mit `multiple` und UI‑Hinweis „Mehrere Dateien möglich“.
  - Drag&Drop: erlaubt „mehrere“; Auswahl im Namen/Counter anzeigen (z. B. „3 Dateien ausgewählt“).

- public/js/app.js
  - `initUploadForm()` erweitern:
    - Wenn `multiple` aktiv: Queue/Array aller `files` aufbereiten.
    - Direct‑Upload‑Flow pro Datei wiederverwenden (bestehendes `directUpload(file)` → extrahieren in Helfer; neuer `directUploadFiles(files)` orchestriert die Aufrufe, optional 2–3 gleichzeitige Uploads).
    - Ergebnisse sammeln: pro Datei `{ uuid, view_url, duplicate }`.
    - Nach Abschluss: Batch registrieren (siehe API unten) → Redirect auf `/upload/batch/:id`.
  - Fallback (nicht Direct‑to‑Storage): HTMX‑Pfad unterstützt aktuell nur eine Datei. Zwei Optionen:
    1) Minimal: Bei Multi‑Select Direct‑Upload erzwingen; sonst Hinweis/Degradation.
    2) Später: Server‑seitige Schleife im App‑Upload bauen (POST mehrfach), Ergebnisse sammeln.

- UI/Progress
  - Pro Datei: kleiner Progress in einer List‑Group oder einfacher Text mit Prozent. Für MVP reicht ein laufender Eintrag + Gesamtfortschrittsbalken.

## Backend/API
- Neue Ephemeral‑Batch‑Registrierung
- Route: `POST /api/v1/upload/batches` (auth required)
  - Request: `{ items: [{uuid: string, view_url: string, duplicate?: bool}] }`
  - Response: `{ batch_id: string, expires_at: int64 }`
  - Server speichert die Liste in Redis/Dragonfly (`cache` util) mit TTL (z. B. 30 Min). Key: `upload:batch:<id>`.

- Ergebnis‑Seite (Einmalansicht)
  - Route: `GET /upload/batch/:id` (auth required)
  - Controller lädt Liste aus Cache; zeigt Seite (templ) an.
  - „Single‑Use“-Optionen:
    - Variante A (strikt): Beim ersten GET markiert der Server den Batch als „consumed“ und löscht Key unmittelbar; weitere Aufrufe → 404/Redirect.
    - Variante B (TTL): Gültig 30 Min; zusätzlich Header/Info „Seite ist temporär“. Optional Query `?one=1` zum Sofort‑Verbrauch. Empfehlung: Variante A (erfüllt „nicht zurückkommen“ wörtlich), plus sanfter Redirect auf `/user/images`.

- Sicherheit
  - `batch_id` mit hoher Entropie (z. B. 128‑Bit random, Base58/Hex). Immer prüfbar: `user_id` muss zum Batch passen (Key enthält UserID oder Value speichert UserID → Zugriffsschutz).
  - Rate‑Limit‑Anpassung: Per‑User‑Limit wird für Premium mindestens auf `MaxFilesPerBatch(plan)` innerhalb 60s angehoben, damit Multi‑Upload nicht fälschlich blockiert.
  - DB‑Deadlock‑Fix: Temporärer Share‑Link ist jetzt eindeutig (`tmp-<uuid>`), damit der uniqueIndex auf `share_link` bei parallelen Inserts nicht kollidiert. `AfterCreate` setzt danach den finalen, kurzen Link.

## Templates / Seiten
- views/upload/batch_result.templ (neu)
  - Liste (`ul` / DaisyUI `card`/`table`): links Mini‑Vorschau (Thumbnail klein, falls fertig; sonst Platzhalter + „Wird verarbeitet…“), rechts Box:
    - Share‑Link (readonly input + Copy‑Button)
    - „Bearbeiten“‑Button → `/image/:uuid/edit`
    - Label „Bereits vorhanden“ falls `duplicate == true`
  - CTA: „Alle Links kopieren“.
  - Hinweis: „Diese Seite ist temporär.“

## Router/Controller
- internal/pkg/router/http_router.go
  - `app.Post("/api/v1/upload/batches", requireAuthMiddleware, controllers.HandleCreateUploadBatch)`
  - `app.Get("/upload/batch/:id", requireAuthMiddleware, controllers.HandleUploadBatchView)`

- app/controllers/multi_upload_controller.go (neu)
  - `HandleCreateUploadBatch(c)` → liest `user_id` aus Context, validiert Items, speichert in Cache (`cache.SetEx`) inkl. UserID + Items.
  - `HandleUploadBatchView(c)` → lädt + (optional) löscht Key, rendert View, oder Redirect `/user/images` wenn weg.

- Cache‑Schema
  - Key: `upload:batch:<id>`
  - Value (JSON): `{ user_id: <uint>, items: [{uuid, view_url, duplicate}], created_at: <unix> }`
  - TTL: 30 Min (konfigurierbar via ENV/Setting optional)

## Ergebnis‑Flow (Direct‑Upload Premium)
1) Nutzer wählt N Dateien → JS baut Queue.
2) Für jede Datei: `POST /api/v1/upload/sessions` → Token/URL → `POST upload_url` (Storage) → Antwort `{ image_uuid, view_url }` bzw. `{ duplicate: true, ... }`.
3) Nach allen Uploads: `POST /api/v1/upload/batches` mit der Item‑Liste → Server gibt `{ batch_id }`.
4) Redirect: `/upload/batch/{batch_id}` (einmalig nutzbar). Verlassen/Reload → 404/Redirect `/user/images`.
5) Normaler Viewer `/i/<share>` bleibt bestehen; nur der Auto‑Redirect nach Upload führt zur Batch‑Seite.

## Daten/Varianten‑Status
- Thumbnails sind ggf. noch in Verarbeitung. In der Batch‑Seite pro Item Polling optional: `GET /api/v1/images/{uuid}/status` existiert → zeigt „verarbeitet“ und aktualisiert Vorschau.

## Fehlerfälle
- Rate‑Limit (IP/User) → bestehende Flash/Redirects erhalten; JS behandelt 429/413/415 bereits. Bei Multi‑Upload: pro Datei behandeln und als „fehlgeschlagen“ markieren.
- Duplicate → als „bereits vorhanden“ mit Link kennzeichnen, zählt als erfolgreich bearbeitet.
- Batch‑Erstellung scheitert → Fallback Redirect `/user/images` mit Flash.

## Tests
- Unit: `CanMultiUpload` für alle Pläne; Batch‑Key‑Ownership (falscher User → 403).
- Integration: `POST /api/v1/upload/batches` (auth), `GET /upload/batch/:id` (single‑use/TTL), Template enthält Liste der Items.
- Frontend: `multiple`‑Attribut nur bei Premium sichtbar (Snapshot/DOM Test falls vorhanden).

## Rollout‑Schritte
1) Entitlements‑Funktion hinzufügen.
2) Startseite/Template: `multiple` + UI‑Hinweis.
3) JS: Multi‑Queue (2–3 Concurrency), Ergebnisse sammeln.
4) API: Batch‑Create + View‑Route (Controller + Router + Cache).
5) Neue templ‑Seite für Batch‑Ergebnis.
6) Smoke‑Tests: Premium vs. Free, Duplikate, Abbruchfälle.

## Offene Punkte / Optionen
- Max. Anzahl Dateien pro Batch? Vorschlag: Premium 20, Premium‑Max 50, Limit hard‑coded in Entitlements.
- „Nicht zurückkommen“ streng? Empfehlung: Single‑Use; sonst TTL + Button „Seite schließen“.
- „Alle Links kopieren“: in einzeiligem Format (je Zeile ein Link) oder Markdown‑Liste – Präferenz?
- Optional: Button „Als Album speichern“ → neues Album mit Items erstellen.

## Aufwand (grobe Schätzung)
- Entitlements + Template Toggle: 0.5–1 h
- JS Multi‑Queue + UI: 4–6 h
- API + Cache + Controller: 2–3 h
- Batch‑Templ + Polling: 2–3 h
- Tests + Fine‑Tuning: 2–4 h

---
Diese Umsetzung hält Single‑Upload kompatibel, nutzt bestehende Direct‑Upload‑Logik pro Datei und ergänzt eine kleine, abgeschirmte Ephemeral‑„Batch Result“‑Seite. Keine Migration der Datenbank notwendig; Cache (Redis/Dragonfly) genügt.

## Fortschritt
- [x] Entitlements: `CanMultiUpload`, `MaxFilesPerBatch`
- [x] Home‑Upload‑UI: `multiple` nur für Premium/Premium‑Max + Hinweis
- [x] JS: Multi‑Upload (sequentiell), Batch sammeln und an API senden; Multi‑Submit wird nun unabhängig von `direct_upload_enabled` abgefangen
- [x] API: `POST /api/v1/upload/batches` speichert Batch in Cache (TTL 30m)
- [x] View‑Route: `GET /upload/batch/:id` (Single‑Use → sofortiger Verbrauch)
- [x] Batch‑Ergebnis‑Seite: Liste mit Preview, Share‑Link‑Copy, Edit‑Button
- [ ] „Als Album speichern“: UI‑Button platziert (disabled), Backend‑Flow noch offen
- [ ] Feintuning: Progress je Datei und Gesamtfortschritt, Limitierung per Plan, Fallback ohne Direct‑Upload
- [ ] Tests: Unit/Integration für Batch‑API + View + Entitlements

Nächste Schritte
- Implementieren „Als Album speichern“: POST‑Endpoint, der ein neues Album erstellt und alle Batch‑Items als Bilder hinzufügt. Danach Redirect auf Album‑Seite.
- Optional: Concurrency 2–3 in der JS‑Queue, visuelles per‑Datei‑Feedback.
- Optional: Max‑Files je Plan durchsetzen (UI + JS + Serverseitig im Batch‑Create clampen – bereits mit Hard‑Cap 100).
