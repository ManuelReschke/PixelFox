# S3 als Primary Cold Storage (Backblaze B2 + Cloudflare)

Stand: 2026-02-25  
Status: Analyse + laufende Implementierung (Storage-I/O Umbau gestartet)

## Ziel

Backblaze B2 (S3-kompatibel) soll nicht nur Backup-Ziel sein, sondern als normaler Cold-Storage-Pool im Produktivpfad dienen.  
Bilder aus diesem Pool sollen direkt über eine eigene Domain wie `https://images-b2.pixelfox.cc` erreichbar sein (Cloudflare davor).

## Kurzfazit

Die bestehende Architektur hat schon fast alle Bausteine (Storage-Pools mit `storage_type=s3`, `storage_tier=cold`, `public_base_url`, Tiering/Move-Jobs, URL-Auflösung pro Pool).  
Der zentrale Gap: der effektive File-I/O-Pfad ist aktuell filesystem-zentriert (`os.Open`, `os.Create`, `os.Remove`, `/uploads` static) und kann S3-Pools nicht als echtes Ziel/Quelle behandeln.

Damit B2 als „normaler Cold Storage“ funktioniert, müssen wir den Storage-I/O-Pfad auf einen Driver-Ansatz umbauen (local + s3) und Move/Delete-Prozesse S3-fähig machen.

## Verifizierter Ist-Stand (2026-02-25)

Getestet und bestätigt:

- URL funktioniert: `https://images-b2.pixelfox.cc/file/pixelfox-dev/images/test.jpg`
- Bucket: `pixelfox-dev`
- Objekt liegt unter Key: `images/test.jpg`

Geplante Buckets:

- Dev: `pixelfox-dev`
- Prod: `pixelfox-prod`

Umsetzungsentscheidung:

- Finales Key-/URL-Schema bleibt `uploads/...` (kompatibler Modus).
- Objekte werden im Bucket unter `uploads/...` geschrieben.

## Aktueller Ist-Zustand im Code

## Was bereits passt

- Storage-Pools unterstützen `storage_type=s3` und `storage_tier=cold`.
  - `app/models/storage_pool.go`
- Pro Pool existiert `public_base_url`, der bereits für absolute Bild-URLs genutzt wird.
  - `internal/pkg/imageprocessor/variant_helpers.go` (`GetPublicBaseURLForImage`, `MakeAbsoluteForImage`)
- URL-Erzeugung ist pool-basiert und liefert relative Pfade unter `/uploads/...`.
  - `internal/pkg/imageprocessor/imageprocessor.go` (`GetImageURL`)
- Tiering/Move-Mechanik existiert bereits (hot -> warm/cold), inkl. DB-Update `storage_pool_id`.
  - `internal/pkg/jobqueue/tiering.go`
  - `internal/pkg/jobqueue/move_processor.go`
- S3-Client auf Basis StoragePool existiert (`PoolClient`).
  - `internal/pkg/s3backup/pool_client.go`

## Was aktuell blockiert

- `StorageManager.SaveFile/DeleteFile/MigrateFile/GetFilePath` arbeiten nur mit lokalem Dateisystem (`filepath.Join`, `os.*`).
  - `internal/pkg/storage/manager.go`
- Upload-Controller schreiben immer lokal in den Pool-`BasePath`.
  - `app/controllers/upload_controller.go`
  - `app/controllers/storage_upload_controller.go`
- Bildverarbeitung öffnet Originale direkt aus dem Dateisystem und schreibt Varianten lokal.
  - `internal/pkg/imageprocessor/imageprocessor.go`
- Delete-Flow löscht Original/Varianten lokal, nicht objektbasiert aus S3.
  - `internal/pkg/imageprocessor/imageprocessor.go` (`DeleteImageAndVariants`)
- App-Serving `/uploads` ist lokal statisch gemountet.
  - `cmd/pixelfox/main.go`

Konsequenz: Ein S3-Pool kann derzeit als Backup-Ziel funktionieren, aber nicht robust als regulärer Storage-Pool im Lebenszyklus (move, delete, recover, optional reprocess).

## Zielbild (technisch)

1. Uploads bleiben zunächst wie bisher in Hot-Storage (lokal/NFS) für schnelle Verarbeitung.
2. Tiering verschiebt inaktive/alte Bilder inkl. Varianten nach Cold-S3.
3. Für Bilder im Cold-S3-Pool zeigt `public_base_url` auf `https://images-b2.pixelfox.cc`.
4. URLs bleiben aus App-Sicht `/uploads/...`; Cloudflare rewritet auf B2-Originpfad.
5. Delete/Move/Reconcile funktionieren storage-typ-agnostisch (local<->local, local<->s3, s3<->local).

## Cloudflare + B2 Setup (empfohlen)

## DNS/Origin

- Bucket muss `public` sein (oder private + signierte URL-Strategie, siehe Security).
- Cloudflare DNS: `images-b2.pixelfox.cc` als `CNAME` auf den von Backblaze gelieferten Friendly-URL-Endpunkt (z. B. `f005.backblazeb2.com`), Proxy aktiviert.
- Der aktuell verifizierte direkte Origin-Pfad ist:
  - `/file/<bucket>/images/...`
  - Beispiel Dev: `/file/pixelfox-dev/images/test.jpg`
- Für Prod entsprechend:
  - `/file/pixelfox-prod/images/...`

Es gibt damit zwei valide Betriebsmodi:

- Modus A (kompatibel, empfohlen für minimalen App-Umbau):
  - App erzeugt weiter `/uploads/...`
  - Cloudflare Rewrite auf `/file/<bucket>/uploads/...`
- Modus B (B2-nativ):
  - App erzeugt direkt `/file/<bucket>/images/...` oder `/images/...` (mit Rewrite)
  - Dafür muss URL-Generierung im Code angepasst werden

## App-seitige Pool-Config

- Für den Cold-S3-Pool setzen:
  - `storage_type = s3`
  - `storage_tier = cold`
  - `public_base_url = https://images-b2.pixelfox.cc`
  - S3 Credentials/Region/Bucket/Endpoint
- `is_backup_target` nur setzen, wenn dieser Pool wirklich Backup-Ziel sein soll.

## Wichtige Design-Entscheidung für Objektkeys

Mit eurem bestätigten Testpfad sind diese beiden Varianten sinnvoll:

- Variante 1 (kompatibel zur aktuellen App-URL-Logik):
  - Original: `uploads/original/YYYY/MM/DD/<uuid>.<ext>`
  - Varianten: `uploads/variants/YYYY/MM/DD/<uuid>_*.{webp,avif,...}`
  - Vorteil: `GetImageURL()` kann nahezu unverändert bleiben.

- Variante 2 (nahe an eurem aktuellen B2-Test):
  - Original: `images/original/YYYY/MM/DD/<uuid>.<ext>`
  - Varianten: `images/variants/YYYY/MM/DD/<uuid>_*.{webp,avif,...}`
  - Vorteil: konsistent mit eurem bestätigten `/file/<bucket>/images/...` Setup.
  - Nachteil: mehr Codeanpassung in URL-Erzeugung und ggf. Rewrite-Regeln.

Empfehlung für schnellen Rollout: Variante 1.

## Notwendige Umbauten im Code

## 1) Storage Driver Layer einführen (Pflicht)

Neue interne Abstraktion, z. B.:

- `PutObject(pool, key, reader, metadata)`
- `GetObject(pool, key)`
- `DeleteObject(pool, key)`
- `ObjectExists(pool, key)`
- `CopyObject(srcPool, srcKey, dstPool, dstKey)` (oder stream-basiert)

Implementierungen:

- `local` driver (Dateisystem)
- `s3` driver (auf Basis von `internal/pkg/s3backup/pool_client.go`, erweitert um stream-orientierte Methoden)

Dann `StorageManager` von `os.*` auf Driver-Calls umstellen.

## 2) Move/Reconcile für S3 erweitern (Pflicht)

`internal/pkg/jobqueue/move_processor.go` und `.../reconcile_processor.go` so umbauen, dass sie nicht nur lokale Dateien verschieben, sondern:

- local -> s3
- s3 -> local
- s3 -> s3

unterstützen.

Zusätzlich:

- Objektkey-Bildung standardisieren (immer aus `relativePath + fileName`, mit Prefix `uploads/`).
- idempotent bleiben (exists + checksum/size optional).

## 3) Delete-Pfad storage-agnostisch machen (Pflicht)

`DeleteImageAndVariants` darf nicht mehr direkt `os.Remove` verwenden, sondern muss je nach Pooltyp über Driver löschen.

Sonst bleiben bei S3-gehosteten Bildern Objekte liegen.

## 4) Tiering-Sicherheit ergänzen (Pflicht)

Vor Demotion in Cold-S3 prüfen:

- Verarbeitung komplett (`IsImageProcessingComplete`) ist bereits vorhanden.
- Keine offenen Reconcile-Operationen.

Zusätzlich sinnvoll:

- Retry/Backoff für Move-Fehler (S3 5xx / Netzwerk).
- sauberes Error-Tagging in Job Payload/Logs.

## 5) Processing für S3-Quellen (optional, aber empfohlen)

Wenn später Re-Processing für Bilder im Cold-Pool nötig ist, braucht `imageprocessor` einen temp-download/upload-Flow.

Für Phase 1 (nur Cold nach abgeschlossener Verarbeitung) kann das vorerst optional bleiben.

## 6) Stats/Accounting für S3-Pools korrigieren (empfohlen)

`GetStoragePoolStats` zählt bei S3 aktuell Backup-Daten und direkte Storage-Nutzung zusammen. Das kann bei gemischter Nutzung doppelt zählen.

Empfehlung:

- klare Trennung „Primary Storage“ vs „Backup Storage“ in Stats,
- oder Backup-Zählung nur für explizite Backup-Pools.

## Migrationsstrategie (inkrementell)

1. Infrastruktur vorbereiten
- B2-Bucket + Cloudflare CNAME + Rewrite Rule + Cache Rule.
- Dev mit `pixelfox-dev`, Prod mit `pixelfox-prod`.
- Test mit manuell hochgeladenem Objekt unter dem finalen Präfix (`uploads/...` oder `images/...`).

2. Code-Phase A (Storage Driver + Delete + Move)
- Driver Layer implementieren.
- `StorageManager`, `move_processor`, `reconcile_processor`, Delete-Flow umstellen.
- Integrationstests für local<->s3 Moves.

3. Code-Phase B (Produktiv aktivieren)
- Cold-S3-Pool in Admin anlegen.
- `public_base_url` auf `images-b2.pixelfox.cc`.
- Tiering aktivieren und kleine Kohorte demoten.

4. Beobachtung & Rollout
- 404/403/5xx auf `images-b2` monitoren.
- Queue-Fehler (move/reconcile/delete) monitoren.
- dann schrittweise größere Demotion-Mengen.

## Security/Compliance Hinweise

- B2 App Key nur bucket-scoped und minimalen Rechten vergeben.
- Wenn Public-Bucket: Hotlinking/Abuse-Schutz über Cloudflare WAF/Rate Limits.
- Cache-Control bei Upload setzen (Dateinamen sind UUID-basiert -> gut für lange TTL).
- Für private Bilder später: signierte URL-Strategie (Cloudflare Worker oder B2 Native Download Authorization).

## Offene Punkte vor Implementierung

- Soll Cold-S3 nur Demotion-Ziel sein, oder auch Upload-Fallback wenn Hot/Warm voll?
- Soll es zusätzlich weiterhin S3-Backups geben (separater Bucket), oder ersetzt Cold-S3 das Backup-Konzept teilweise?
- Brauchen wir Re-Processing aus Cold-S3 in Phase 1 bereits, oder erst später?

## Test-Checkliste

- Upload -> Processing -> Tiering -> URL über `images-b2.pixelfox.cc` lädt Original + Varianten.
- Delete entfernt Original + Varianten aus S3.
- Move-Rückweg (s3 -> warm/hot) funktioniert.
- `public_base_url` pro Pool greift korrekt in Viewer/API-Antworten.
- Keine doppelten/inkonsistenten `used_size` Werte nach Move/Delete.

## Externe Referenzen

- Backblaze + Cloudflare CNAME/Rewrite:
  - https://www.backblaze.com/docs/cloud-storage-deliver-public-backblaze-b2-content-through-cloudflare-cdn
- Backblaze Cache-Control Verhalten:
  - https://www.backblaze.com/docs/cloud-storage-deliver-public-backblaze-b2-content-through-cloudflare-cdn-cache
- Backblaze S3-Kompatibilität (URL-Styles, Endpoint):
  - https://www.backblaze.com/docs/cloud-storage-s3-compatible-api
- Backblaze S3 Quickstart (öffentliche URL-Varianten):
  - https://www.backblaze.com/docs/cloud-storage-s3-compatible-api-quickstart
- Cloudflare Default Cache-Verhalten:
  - https://developers.cloudflare.com/cache/concepts/default-cache-behavior/

