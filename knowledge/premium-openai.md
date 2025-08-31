# Premium-Pakete – OpenAI-Review, Mapping und Empfehlungen (Stand: 2025-08-31)

Dieses Dokument bewertet den Vorschlag aus `knowledge/premium.md`, gleicht ihn mit dem aktuellen Code ab (Storage, Direct-Upload, Queue, Repos), und ergänzt konkrete Empfehlungen für eine robuste, schrittweise Umsetzung.

## Kurzfazit
- Das Konzept ist grundsätzlich stimmig und passt zur Architektur (Repository-Pattern, Jobqueue, Direct‑to‑Storage). Wichtig ist eine saubere Trennung: „Policy-Ermittlung“ (Premium-Service) vs. „Enforcement“ (Controller/Middleware/Jobs).
- Kritische Punkte: saubere Quotenführung (User‑Storage), per‑User Max‑Size bis in den Direct‑Upload‑Token propagieren, ablaufende Dateien nur für Basic, und eine spätere saubere Payment‑Integration (Stripe/Webhooks).

## Was passt gut zur aktuellen Codebasis
- Direct‑Upload (Session + Token): Max‑Size kann per Session‑Payload gesetzt werden → ideal, um per‑User Limits zu erzwingen (statt globalem BodyLimit).
- Jobqueue vorhanden: kann Cleanup/Expiry und Reconcile‑Jobs (User‑Storage) übernehmen.
- Repository‑Pattern: Premium‑Service lässt sich als internes Paket sauber einhängen, Controller bleiben schlank.
- Storage: Pool‑Move/Replication ist robust – darf User‑Stats nicht aus der Bahn bringen (siehe Reconcile‑Job unten).

## Ergänzungen/Präzisierungen zum Konzept

### 1) Datenmodell und Migration
- `subscriptions` Tabelle (wie vorgeschlagen) ist gut. Felder passen. Ergänzen:
  - `plan_code` (string, index) zusätzlich zu `Type`, für flexible Vermarktung (z. B. „premium_2025q1“).
  - `canceled_at`, `trial_end` optional (spätere Features).
- Kein hartes „Basic“ schreiben: Wenn keine aktive Sub vorhanden → Basic via Fallback.
- Optional: Tabelle `user_storage_stats` (UserID, storage_used, updated_at) als Cache für schnelle Reads – der „Source of Truth“ bleibt weiterhin die Images‑Tabelle + Reconcile.

### 2) Premium‑Service (Policy) vs. Enforcement (Checks)
- Policy (internal/pkg/premium): Strategy‑Pattern wie vorgeschlagen (Basic/Premium/Max). Rückgabe von:
  - `MaxFileSize`, `StorageQuota` (−1 = unlimited), `MaxAlbums`, `SupportsMultiUpload`, `AvailableFormats` (z. B. ["webp"] oder ["webp","avif"]).
  - `FilesExpire` (bool), `ShowAds` (bool), optionale Rate‑Limits (z. B. pro Minute stärker für Basic).
- Enforcement (bestehende Stellen):
  - Upload Session API (`app/controllers/api_upload_controller.go`): bestimmt `claims.MaxBytes` anhand Premium‑Policy. Wichtig, weil die Direct‑Upload‑Speicher‑API den Token prüft.
  - Klassischer Upload (`app/controllers/image_controller.go`): prüft zusätzlich per Premium‑Service (falls der alte Pfad weiterhin genutzt wird).
  - Album Create (`app/controllers/user_controller.go` bzw. der Album‑Controller): prüft `MaxAlbums`.
  - Imageprozessor (`internal/pkg/imageprocessor`): erzeugt Varianten abhängig von `AvailableFormats` (Policy muss `userID` kennen → Limits/Context in Jobpayload mitschicken).
  - Ads/Expire: Templates + Background‑Jobs.

### 3) Quoten & Storage‑Usage
- On‑write Updates: Nach Upload/Delete `user_storage_stats.storage_used += sizeChange` (wie im Vorschlag). Das ist performant, kann aber driften.
- Reconcile‑Job: Periodisch (z. B. 1×/Tag): `SUM(images.file_size)` pro User nachrechnen und Differenzen in `user_storage_stats` korrigieren. Jobqueue vorhanden → einfach ergänzen.
- Read‑Pfad: UI/Checks lesen aus `user_storage_stats` (schnell). Falls leer/nicht vorhanden → live SUM fallback.
- Wichtig: Pool‑Move/Replication verändert User‑Usage nicht (nur Location), kein Update nötig – Reconcile sichert langfristig Konsistenz.

### 4) Dateien ablaufen lassen (nur Basic)
- Policy: `FilesExpire=true` für Basic, sonst false.
- Implementierung: Cleanup‑Job mit Filter:
  - Kandidaten: `images.user_id IN (basic users)` AND `last_view_at < now()-6mo` AND `is_public=false` (optional) oder generell – abhängig von Produktpolicy.
  - Für jeden Kandidaten: Varianten + Original löschen (StorageManager), DB aufräumen. Safety: Soft‑Delete und später Hard‑Delete.
  - Rate‑Limit/Batching im Jobqueue.

### 5) Bildformat‑Konvertierung per Subscription
- Aktuell global via Settings (Thumbnail*Enabled). Empfehlung: Per‑User steuern, indem der Jobpayload `RequestedFormats []string` enthält.
  - Quelle: Premium‑Policy bei Enqueue der Processing‑Jobs (z. B. in `ProcessImageUnified`), nicht tief im Prozessor selbst ermitteln → deterministischer.
- Vorteil: Admin Settings bleiben globaler Default; Premium überschreibt via Payload.

### 6) Multiupload
- Frontend: File input `multiple` nur, wenn `SupportsMultiUpload == true`.
- Backend: Support für mehrere Dateien in `HandleUpload`/Direct‑Upload bleibt i. d. R. 1‑Datei‑Endpoint (bei Direct‑Upload), aber Client kann pro Datei eine Session anfordern (oder Chunked später). Minimal zuerst: mehrere Files nacheinander mit Sessions.

### 7) Rate‑Limits per User
- Bereits vorhanden: per‑IP + per‑User Rate‑Limit im Storage‑Upload‑Endpoint. Empfehlung: Premium‑Policy kann per‑User Limit erhöhen/abschalten.
  - Umsetzung: Reading Admin‑Settings als Default; Premium‑Policy liefert Override; im Endpoint (Direct‑Upload) bei `userID>0` den Policy‑Wert bevorzugen.

### 8) Ads (nur Basic)
- Einfache Integration: Partials `views/partials/ads.templ` + Einbindung im Layout/Home/Viewer, wenn `ShowAds==true`.
- Später: Feature Flags (A/B), Frequency‑Capping, Datenschutz Banner.

### 9) Pricing/Checkout Flow
- `views/pricing.templ` aktuell statisch. Empfehlung:
  - Buttons „Bald verfügbar“ → `/subscribe/:plan` Routen.
  - Start Phase: Dummy‑Flow (Manuelle Aktivierung in Admin), um alle Policy‑Pfade zu testen.
  - Stripe (empfohlen): Checkout Session pro `plan_code` (monatlich), Webhooks: `checkout.session.completed`, `customer.subscription.updated/deleted`.
  - `subscriptions` aktualisieren anhand Webhooks (Source of Truth), App liest nur Status/Type.

## Minimale Phasenplanung (inkrementell)

Phase 1 – Grundlagen
- DB: `subscriptions` (Migration), optional `user_storage_stats`.
- Paket `internal/pkg/premium` mit Strategy (Basic/Premium/Max), Service und Caching.
- Upload Session API erweitert: setzt `claims.MaxBytes` basierend auf Policy.
- Image‑Processing Enqueue: setzt `RequestedFormats` im Payload.
- User‑Storage Update Hooks (Upload/Delete) + täglicher Reconcile‑Job.
- Album‑Create Check.
- Ads‑Partial einfügen (nur Basic) – rein frontend.

Phase 2 – Cleanup/Expiry
- Cleanup‑Job (6 Monate ohne Views, nur Basic). Soft‑Delete + später Hard‑Delete.
- Admin Settings: Konfiguration von Expiry‑Fenster und Rate (optional).

Phase 3 – Multiupload & Limits
- Frontend multiple Select für Premium; Sessions in Serie.
- Optional: Chunked Upload/Resumable für sehr große Dateien (später).

Phase 4 – Payment
- Checkout‑Flow (Stripe), Webhooks, Subscriptions pflegen.
- Admin Dashboard: Übersicht Subscriptions/Revenue/Churn (backlog‑fähig).

## Konkrete Enforcement‑Punkte (Dateien)
- `app/controllers/api_upload_controller.go` → Session‑Erstellung: setze `MaxBytes` aus Premium‑Policy.
- `app/controllers/image_controller.go` → klassischer Upload: `CheckStorageQuota`, `CheckUploadPermission`.
- `internal/pkg/jobqueue` → neue Jobs: `user_storage_reconcile`, `cleanup_expired_images`.
- `internal/pkg/imageprocessor` → Job‑Payload `RequestedFormats` berücksichtigen.
- `views/layouts` / `views/partials/ads.templ` → Ads only for Basic.
- `app/controllers/user_album_controller.go` (oder entsprechende Datei) → Albumlimit prüfen.

## Performance & Skalierung
- Quotencheck: Nutzung `user_storage_stats` (Cache) → O(1). Reconcile offline.
- Limits im Direct‑Upload: serverseitig sicher, da Token `max_size` trägt.
- Jobs: Batches und Backoff nutzen (wie bei Move/Replication). Sweeper schützt vor Hängern.

## Security & Edge Cases
- Trust no client: Alle Grenzen backend‑seitig erzwingen.
- Token‑Missbrauch: Upload‑Sessions sind kurzlebig, signiert, enthalten Pool‑/User‑ und Max‑Size – passt.
- Abomissbrauch: Statuswechsel nur über Webhooks/Backoffice.
- Pool‑Move/Replikation: Kein Einfluss auf Quoten; Reconcile korrigiert ggf. Drift.
- Expiry: Excludes Premium‑User, Prüfpfad anhand aktiver Sub bei Job‑Laufzeit.

## To‑Do (Follow‑up Tickets)
- Migrationen: `subscriptions`, optional `user_storage_stats`.
- Premium‑Service Implementierung + Strategy + Caching.
- Upload‑Session‑Policy: `claims.MaxBytes` per User.
- Image‑Processing Payload erweitert (`RequestedFormats`).
- Cleanup‑Job + Reconcile‑Job.
- Ads‑Partial und Layout‑Einbindung.
- Pricing‑Buttons → `/subscribe/:plan` + Admin‑Manuell‑Aktivieren (Phase 0) → Stripe (Phase 4).

Diese Roadmap fügt sich sauber in die bestehende Architektur ein, nutzt vorhandene Infrastruktur (Queue, Direct‑to‑Storage) und minimiert Risiko durch klare Phasen und robuste Enforcement‑Punkte.

