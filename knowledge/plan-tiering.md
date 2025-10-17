• Alles klar — ich habe Variante B und den manuellen Sweep umgesetzt.

Was ich umgesetzt habe

- Zweiter Storage-Node (warm) in docker-compose.override.yml
    - Neuer Service app_s02 mit NODE_ID=s02 und DISABLE_JOB_WORKERS=1 (damit nur s01 Worker/Sweeper läuft)
    - Ports: 8083:4000, Volumes: ./uploads_s02 → /app/uploads_s02 und /app/uploads
    - app_s01 bleibt (hot) auf 8082:4000 mit ./uploads_s01
    - Datei: docker-compose.override.yml
- Manueller Tiering-Sweep (Admin)
    - Route: POST /admin/storage/tiering/sweep (Admin-only)
    - Controller ruft einmalig den Sweep: jobqueue.GetManager().RunTieringSweepOnce()
    - Flash-Meldung auf Erfolg/Fehler und Redirect zurück auf Admin → Storage
    - Dateien:
        - internal/pkg/jobqueue/manager.go: neue Methode RunTieringSweepOnce()
        - app/controllers/admin_storage_controller.go: HandleAdminTieringSweep()
        - app/controllers/admin_handler_adapter.go: Adapter HandleAdminTieringSweep()
        - internal/pkg/router/http_router.go: Admin-Route registriert
- Tiering-Status in Admin → Speicherverwaltung
    - Oben auf der Seite:
        - Button “Tiering‑Sweep ausführen” (hx-post, nutzt globalen CSRF-Input via hx-include)
        - Karte “Tiering”: Status (ein/aus), Hot≥High X/Y, No‑Views Demote + Keep‑in‑Hot, Sweep-Intervall und Max‑Batch
    - Dateien:
        - views/admin_views/storage_management.templ
        - views/admin_views/storage_management_templ.go (aktualisiert mit Button und Status)

Wie du es lokal testest

- Compose
    - Starte mit docker-compose up -d (bestehende Services) und mit override: docker-compose -f docker-compose.yml -f docker-compose.override.yml up -d
    - app (8080), app_s01 hot (8082), app_s02 warm (8083)
- Pools im Admin anlegen
    - Hot (s01):
        - BasePath: /app/uploads_s01
        - Tier: hot
        - NodeID: s01
        - PublicBaseURL: http://localhost:8082
        - UploadAPIURL: http://app_s01:4000/api/internal/upload
    - Warm (s02):
        - BasePath: /app/uploads_s02
        - Tier: warm
        - NodeID: s02
        - PublicBaseURL: http://localhost:8083
        - UploadAPIURL: http://app_s02:4000/api/internal/upload
    - REPLICATION_SECRET und UPLOAD_TOKEN_SECRET in .env identisch für beide
- Tiering-Settings zum Testen (Admin → Einstellungen)
    - hot_keep_days_after_upload: 0
    - demote_if_no_views_days: 0
    - max_tiering_candidates_per_sweep: 5–10
    - tiering_sweep_interval_minutes: 1–2 (oder Button nutzen)
    - tiering_enabled: an
- Testablauf
    - Ein paar Bilder (Hot) hochladen (Node s01).
    - Admin → Speicherverwaltung:
        - Tiering‑Karte kontrollieren (Status/Watermarks/Intervalle)
        - Button “Tiering‑Sweep ausführen” klicken → verschiebt in Warm (s02) via HTTP‑Replicate. Erfolg/Fehler per Flash.
        - Tabelle zeigt steigende Datei-/Nutzungswerte im Warm‑Pool; Hot sinkt

Build-Fix

- interner Fehler “declared and not used: low” behoben:
    - internal/pkg/jobqueue/tiering.go

Hinweise

- Für die neuen Form-Felder unter Admin → Einstellungen bitte einmal Templates generieren:
    - make generate-template
- Der Tiering‑Button nutzt hx-post und bindet den globalen CSRF-Input per hx-include ein; kein eigener Token in der Teilansicht nötig.

Möchtest du, dass ich auch Beispiel-Pooldefinitionen (fertige Werte) in die Knowledge-Datei schreibe oder eine kurze README-Sektion zur Zwei-Node-Dev-Topologie ergänze?
