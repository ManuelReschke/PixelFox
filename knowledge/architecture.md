Jetzt verstehe ich das Problem! Wir haben tatsächlich zwei separate Queue-Systeme:

Aktuelle Architektur:

1. ImageProcessor Queue (internal/pkg/imageprocessor/)

- Zweck: Bild-Verarbeitung (Thumbnails, WebP/AVIF Konvertierung)
- Typ: In-Memory Channel-basiert
- Worker: 3 Worker (MaxWorkers)
- Jobs: ProcessJob mit *models.Image
- Processing: Synchron in der Anwendung

2. JobQueue System (internal/pkg/jobqueue/)

- Zweck: S3 Backup/Delete Jobs
- Typ: Redis-basiert, persistent
- Worker: 3 Worker
- Jobs: Job mit verschiedenen JobType (S3Backup, S3Delete)
- Processing: Persistent, retry-fähig

Das Problem:

// Image Upload Controller - ZWEI separate Queues!
go func() {
// Queue 1: ImageProcessor (in-memory)
if err := imageprocessor.ProcessImage(&image); err != nil {
// ...
}
}()

go func() {
// Queue 2: JobQueue (Redis)
if err := enqueueS3BackupIfEnabled(&image); err != nil {
// ...
}
}()

Probleme dieser Architektur:

1. Doppelte Ressourcen: 2×3 = 6 Worker für ähnliche Aufgaben
2. Keine Koordination: Bild-Processing und S3-Backup unabhängig
3. Komplexität: Zwei verschiedene Queue-Systeme pflegen
4. Redundanz: Ähnliche Worker-Pool-Logik

Refactoring-Vorschlag:

Option A: Unified Job Queue ⭐ (Empfohlen)

Alles in das Redis-basierte JobQueue System migrieren:

// Neue Job Types
const (
JobTypeImageProcessing JobType = "image_processing"
JobTypeS3Backup       JobType = "s3_backup"
JobTypeS3Delete       JobType = "s3_delete"
)

Vorteile:
- Ein Queue-System für alles
- Retry-Logik für Image Processing
- Bessere Monitoring
- Koordinierte Ausführung

Option B: Sequential Pipeline

Image Processing → S3 Backup in einer Pipeline:

// Ein Job mit mehreren Schritten
type ImageProcessingJobPayload struct {
ImageID      uint   `json:"image_id"`
ImageUUID    string `json:"image_uuid"`
EnableBackup bool   `json:"enable_backup"`
Steps        []ProcessingStep `json:"steps"`
}

Option C: Trigger-basiert

Image Processing triggert S3 Backup bei Completion.

Meine Empfehlung:

Refactoring zu Option A - Unified Job Queue System:

1. Image Processing als Job in die Redis-Queue
2. Sequenzielle Abarbeitung: Processing → Backup
3. Ein Worker-Pool (3-5 Worker)
4. Retry-fähiges Image Processing
5. Bessere Fehlerbehandlung

Zeitaufwand: 4-6 Stunden
Nutzen: Vereinfachte Architektur, bessere Reliability

Soll ich das Refactoring implementieren? Das würde die Architektur deutlich sauberer und wartbarer machen.
