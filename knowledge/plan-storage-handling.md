• Zielbild

- Aktive/heiße Bilder bleiben in Hot-Tier; inaktive werden automatisch in Warm/Cold verschoben.
- Steuerung über Nutzungsdaten (Views), Zeitkriterien und Kapazitätsgrenzen; Moves laufen asynchron über die vorhandene Queue/Move-Jobs.

Option 1: Zeit-/Inaktivitätsbasiert (einfach)

- Idee: Nach Upload N Tage im Hot; demote, wenn seit M Tagen keine Views.
- Benötigt:
    - Feld “last_viewed_at” (oder ähnliches) in images.
    - Leichtgewichtige “Touch”-Aggregation: beim View Redis-Key image:lastview:<id> setzen; periodisch in DB flushen (analog zu View-Counter-Flush).
    - Periodischer “Tiering-Sweeper”-Job: selektiert Kandidaten (created_at/last_viewed_at) und enqueued Move-Jobs in Warm/Cold (per move_image).
- Pros: Klar, deterministisch, geringe Komplexität. Cons: Kein Kapazitätsfeedback, evtl. zu starr.

Option 2: Kapazitätsgetrieben (LRU/Evictor)

- Idee: Wenn Hot-Tier > X% genutzt, demote “kälteste” Bilder (ältestes last_viewed_at bzw. wenig Views in letzter Periode) bis < Y% (Hysterese).
- Benötigt:
    - Last-Viewed/Tagesviews (s.u.).
    - Sweeper, der bei Überschreitung der Hot-Watermark Kandidaten (nach Score) auswählt und move_image enqueued (Batchweise, z. B. 100–500 Bilder pro Lauf).
- Pros: Reagiert dynamisch auf Füllstand. Cons: Etwas mehr Logik (Wasserstände, Score).

Option 3: Heat-Score (bidirektional Demote/Promote)

- Idee: Score = f(LastViewed, Views_7d/30d, Uploadalter, Größe). Demote bei niedrigem Score, Promote bei hohem Score (wenn Bild in Warm/Cold plötzlich wieder “heiß” wird).
- Benötigt:
    - Rolling-Views pro Bild (Redis Hash nach Tag/Woche) oder einfache Zeitfenster; zusätzlich LastViewedAt-Flush.
    - Promotion-Trigger: z. B. bei Überschreiten eines Schwellenwerts Views_7d → move_image in Hot.
- Pros: Sehr treffsicher. Cons: Höchste Komplexität, Feintuning der Schwellen nötig, Thrashing-Schutz (Mindestverweildauer) erforderlich.

Option 4: Archivierung nach S3 (Phase 2/Optional)

- Idee: Nach z. B. 90 Tagen Inaktivität Primärkopien aus lokalem Cold entfernen, S3 dient als Archiv (Backup ist bereits vorhanden).
- Varianten:
    - “Cold = S3 primär”: erfordert Serving via S3/CDN oder Presigned-URLs (derzeit nicht umgesetzt).
    - “Lazy Restore”: bei Zugriff auf archiviertes Bild Job zur Wiederherstellung (s3backup.DownloadFile) in Hot + Re-Generierung Varianten.
- Pros: Spart lokalen Speicher signifikant. Cons: Höherer Integrationsaufwand (Serving/Restore-Pfade, UX während Restore).

Datenbasis für Entscheidungen

- Bereits vorhanden:
    - view_count (Redis → DB Flush) und created_at in Images.
- Sinnvoll zu ergänzen:
    - last_viewed_at (per Redis Touch + periodischem Flush).
    - Optional Rolling-Views (7/30 Tage) in Redis (Hash je Zeitfenster), periodisch gelesen (muss nicht in DB persistent sein).

Nutzung vorhandener Bausteine

- Moves: Bereits implementiert (move_image + PUT /api/internal/replicate), inkl. Node‑Routing, Kapazitätschecks, Pfad-Normalisierung und DB‑Updates.
- Pools/Tiers: Bereits vorhanden (Hot/Warm/Cold/Archive), Auswahl-Helper existieren.
- Queue/Workers: Bereits vorhanden (konfigurierbare Worker, Sweeper, Retry-Mechanismen).
- Health/Capacity: Pool‑Usage + Health verfügbar; für Watermarks nutzbar.

Empfohlener Start (pragmatisch)

- Phase A (einfach, robust):
    - Ergänze last_viewed_at + Redis-Flush; führe Option 1 ein (N/M Tage) mit täglichem Sweeper.
    - Optional: Kapazitäts-Hysterese (falls Hot > 80%, zusätzlich demoten bis < 65%).
- Phase B (verbessert):
    - Kapazitätsgetriebenes LRU (Option 2) mit klaren High-/Low‑Watermarks.
    - Min.-Verweildauer in Tiers (z. B. 7 Tage) zur Thrashing-Vermeidung.
- Phase C (bedarfsgerecht):
    - Promotion-Logik (Option 3) für vielgefragte Warm‑Bilder zurück in Hot.
    - S3‑Archiv (Option 4) mit Lazy‑Restore, falls lokaler Speicher teuer ist.

Konfigurationsvorschläge (Admin/Settings)

- hot_watermark_high/hot_watermark_low (z. B. 80/65).
- hot_keep_days_after_upload (z. B. 7–14).
- demote_if_no_views_days (z. B. 30).
- min_dwell_days_per_tier (z. B. 7), max_candidates_per_sweep (Batchgröße).
- “Pin/Exclude from tiering” pro Bild/Album (optional).

Betriebsaspekte

- Moves in Batches, Backpressure über Queue-Woker begrenzen.
- Hysterese + Mindestverweildauer, um Ping-Pong zu verhindern.
- Monitoring: Anzahl kandidierter/verschobener Bilder, freigeräumter Speicher, Sweep-Dauer, Fehlerrate.
- Rollout in Read‑Only‑Mode (Dry‑Run), bevor echte Moves starten (nur Kand.-Liste + Metriken).

Wenn du möchtest, skizziere ich als Nächstes die konkreten Settings/Jobs (ohne Code) und ein Migrationsschema für last_viewed_at, plus ein Beispiel‑Sweep‑SQL/Redis‑Plan.
