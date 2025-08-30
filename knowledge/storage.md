# PixelFox Storage Architecture Analysis

*Analysiert am: 22. August 2025*
*Version: Refactoring Branch (ba5cc46)*

## Overview

PixelFox implementiert eine hochmoderne, multi-tiered Storage-Architektur mit automatischer Bildverarbeitung und Cloud-Backup. Das System unterstützt verschiedene Storage-Typen (Local, NFS, S3), dynamisches Tiering (Hot/Warm/Cold/Archive) und eine Redis-basierte Job Queue für asynchrone Verarbeitung.

---

## 1. Architektur-Überblick

### 1.1 Hauptkomponenten

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   File Upload   │───▶│  Storage Pools  │───▶│   Job Queue     │
│   Controller    │    │   Management    │    │  (Redis-based)  │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         │                       ▼                       ▼
         │              ┌─────────────────┐    ┌─────────────────┐
         │              │  Storage Pools  │    │ Image Processing│
         │              │ Hot│Warm│Cold    │    │ & Backup System │
         │              └─────────────────┘    └─────────────────┘
         │                       │                       │
         ▼                       ▼                       ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│    Database     │    │   File System   │    │   S3 Backup     │
│  (Images, Meta) │    │ Local│NFS│S3    │    │ Multi-Provider  │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

### 1.2 Datenfluss bei Upload

1. **Upload-Request** → `HandleUpload()` in `image_controller.go:31`
2. **Storage Pool Selection** → Hot-Storage-First Strategie
3. **File Save** → Storage Pool BasePath + `/original/YYYY/MM/DD/UUID.ext`
4. **Database Entry** → Image Record mit Storage Pool ID
5. **Job Enqueueing** → `jobqueue.ProcessImageUnified()`
6. **Image Processing** → Variants Generation (WebP, AVIF, Thumbnails)
7. **S3 Backup** → Optional, konfigurierbar delayed (0-30 Tage)

---

## 2. Storage Pool System

### 2.1 Storage Tiers

| Tier | Beschreibung | Verwendung | Performance |
|------|-------------|------------|-------------|
| **Hot** | High-performance SSD | Neue Uploads, häufiger Zugriff | Sehr hoch |
| **Warm** | Medium performance | Mäßig häufiger Zugriff | Hoch |
| **Cold** | Archive HDD | Seltener Zugriff | Mittel |
| **Archive** | Langzeit-Archiv (Tape) | Sehr seltener Zugriff | Niedrig |

### 2.2 Storage Types

```go
// Unterstützte Storage Types (models/storage_pool.go:23-28)
const (
    StorageTypeLocal = "local" // Local filesystem storage
    StorageTypeNFS   = "nfs"   // Network File System
    StorageTypeS3    = "s3"    // S3-compatible (AWS, B2, MinIO)
)
```

### 2.3 Pool Selection Logik

**Hot-Storage-First Upload** (`SelectOptimalPoolForUpload`):
1. Suche Hot Storage Pools mit verfügbarem Platz
2. Fallback auf Warm Storage
3. Final Fallback auf beliebigen verfügbaren Pool
4. Basiert auf Priority (niedrigere Zahl = höhere Priorität)

**Implementierung:** `models/storage_pool.go:447-480`

---

## 3. Bildverarbeitungs-Pipeline

### 3.1 Image Processing Flow

```
Upload → DB Entry → Job Queue → Processing Worker
                                      ↓
                         ┌─────────────────────────┐
                         │    Image Processing     │
                         │ (imageprocessor.go)     │
                         └─────────────────────────┘
                                      ↓
              ┌─────────┬─────────────┼─────────────┬─────────┐
              ▼         ▼             ▼             ▼         ▼
         Original   WebP Full   AVIF Full   Small Thumbs  Medium Thumbs
                                            (WebP/AVIF/   (WebP/AVIF/
                                             Original)     Original)
```

### 3.2 Generated Variants

**Pro hochgeladenem Bild werden folgende Variants erstellt:**

| Variant Type | Format | Größe | Qualität | Zweck |
|-------------|--------|--------|----------|-------|
| `webp` | WebP | Original | 85% | Optimierte Vollgröße |
| `avif` | AVIF | Original | 35 CRF | Beste Kompression |
| `thumbnail_small_webp` | WebP | 200px | 85% | Kleine Vorschau |
| `thumbnail_small_avif` | AVIF | 200px | 35 CRF | Kleine Vorschau |
| `thumbnail_small_original` | Original | 200px | 90% | Kleine Vorschau |
| `thumbnail_medium_webp` | WebP | 500px | 85% | Medium Vorschau |
| `thumbnail_medium_avif` | AVIF | 500px | 35 CRF | Medium Vorschau |
| `thumbnail_medium_original` | Original | 500px | 90% | Medium Vorschau |

**Konfiguration:** Admin kann über `/admin/settings` steuern, welche Formate generiert werden.

### 3.3 Storage Structure

```
Storage Pool Base Path/
├── original/
│   └── YYYY/MM/DD/
│       └── {UUID}.{ext}
└── variants/
    └── YYYY/MM/DD/
        ├── {UUID}.webp
        ├── {UUID}.avif
        ├── {UUID}_small.webp
        ├── {UUID}_small.avif
        ├── {UUID}_small.{ext}
        ├── {UUID}_medium.webp
        ├── {UUID}_medium.avif
        └── {UUID}_medium.{ext}
```

---

## 4. Job Queue System

### 4.1 Unified Redis Job Queue

**Ersetzt das alte in-memory Worker Pool System**

```go
// Job Types (jobqueue/types.go:13-17)
const (
    JobTypeImageProcessing JobType = "image_processing"
    JobTypeS3Backup        JobType = "s3_backup"  
    JobTypeS3Delete        JobType = "s3_delete"
)
```

### 4.2 Job Processing Flow

1. **Enqueue:** `jobqueue.ProcessImageUnified()` 
2. **Redis Storage:** Job wird in Redis gespeichert
3. **Worker Processing:** Konfigurierbare Worker (1-20) verarbeiten Jobs
4. **Status Updates:** Redis Cache für real-time Status
5. **Retry Logic:** Automatische Wiederholung bei Fehlern

### 4.3 Konfigurierbare Parameter

| Parameter | Admin Path | Range | Beschreibung |
|-----------|-----------|-------|--------------|
| Worker Count | `/admin/settings` | 1-20 | Parallele Job Worker |
| S3 Backup Delay | `/admin/settings` | 0-43200 min | Verzögerung für S3 Backup |
| S3 Check Interval | `/admin/settings` | 1-60 min | Intervall für Backup Checks |
| S3 Retry Interval | `/admin/settings` | 1-60 min | Retry-Intervall bei Fehlern |

---

## 5. S3 Backup System

### 5.1 Modern Storage Pool Integration

**Anstatt Environment Variables nutzt das System konfigurierte S3 Storage Pools:**

```go
// S3 Pool Configuration (models/storage_pool.go:44-51)
S3AccessKeyID     *string // Credentials
S3SecretAccessKey *string // (verschlüsselt in DB)
S3Region          *string // Region (e.g., us-west-001)
S3BucketName      *string // Bucket Name
S3EndpointURL     *string // Endpoint (B2, MinIO, etc.)
S3PathPrefix      *string // Optional Path Prefix
```

### 5.2 Backup Process

1. **Trigger:** Nach erfolgreicher Image Processing
2. **Delay:** Konfigurierbar (0 min bis 30 Tage)
3. **Storage Pool Selection:** Höchste Priorität S3 Pool
4. **Upload:** Original + alle Variants
5. **Tracking:** `image_backups` table mit Status
6. **Object Key Structure:** `{prefix}/YYYY/MM/DD/{UUID}.{ext}`

### 5.3 Multi-Provider Support

- **AWS S3:** Native Support
- **Backblaze B2:** Path-style URLs, spezielle Region-Behandlung
- **MinIO:** Self-hosted S3-compatible
- **Andere S3-kompatible Dienste**

---

## 6. Schwächen & Bottlenecks

### 6.1 Upload-System Bottlenecks

| Problem | Beschreibung | Impact | Lösungsansatz |
|---------|-------------|---------|---------------|
| **Synchroner Upload** | Jeder Upload blockiert bis zur Completion | Hoch | Async Upload mit Chunking |
| **Memory Buffer** | 1MB Fixed Buffer für alle Dateien | Mittel | Adaptive Buffer Size |
| **No Resume** | Unterbrochene Uploads starten neu | Hoch | Resumable Upload Protocol |
| **Hot Storage Saturation** | Alle Uploads gehen in Hot Storage | Hoch | Load Balancing zwischen Pools |

### 6.2 Image Processing Bottlenecks

| Problem | Beschreibung | Impact | Lösungsansatz |
|---------|-------------|---------|---------------|
| **Sequential Processing** | WebP → AVIF → Thumbs nacheinander | Hoch | Parallel Processing |
| **CPU Intensive** | FFMPEG AVIF sehr CPU-intensiv | Hoch | GPU Acceleration / Separate Nodes |
| **Memory Usage** | Große Bilder verbrauchen viel RAM | Mittel | Memory Streaming Processing |
| **Single Point Dependency** | FFMPEG Required für AVIF | Mittel | Alternative AVIF Encoder |

### 6.3 Database Bottlenecks

| Problem | Beschreibung | Impact | Lösungsansatz |
|---------|-------------|---------|---------------|
| ✅ ~~Connection Pooling~~ | **IMPLEMENTIERT:** GORM Connection Pool optimiert | ✅ | **DONE:** MaxIdle=10, MaxOpen=100, Lifetime=1h |
| **Synchronous Writes** | Variants werden einzeln geschrieben | Mittel | Batch Inserts |
| **Single DB Instance** | Keine Skalierung bei hoher Last | Hoch | Read Replicas / Sharding |

### 6.4 S3 Backup Bottlenecks

| Problem | Beschreibung | Impact | Lösungsansatz |
|---------|-------------|---------|---------------|
| **Sequential Upload** | Ein Backup nach dem anderen | Mittel | Parallel S3 Uploads |
| **No Compression** | Files uncompressed übertragen | Niedrig | Compression vor Upload |
| **Rate Limits** | S3 Provider Rate Limits | Hoch | Rate Limiting + Queuing |

---

## 7. Skalierungsanalyse

### 7.1 Aktuelle Kapazität (Schätzung)

**Basierend auf Standard-Konfiguration:**
- **20 Job Workers** (max konfigurierbar)
- **Single MySQL Instance**
- **Hot-Storage-First** ohne Load Balancing

**Geschätzte Kapazität:**
- **~50-100 gleichzeitige Uploads** (abhängig von Bildgröße)
- **~500-1000 Uploads/Stunde** (bei normaler Nutzung)
- **~10.000-20.000 Uploads/Tag** (kontinuierlich)

### 7.2 Skalierung für hunderte Nutzer

**Szenario: 100-500 aktive Nutzer, 50-100 gleichzeitige Uploads**

#### 7.2.1 Kurzfristige Optimierungen (< 1 Monat)

```yaml
Priority: Hoch
Aufwand: Niedrig
Impact: Hoch
```

1. **Worker Scaling**
   - Job Queue Workers: 20 → 50-100
   - Redis Memory: Erhöhen für größere Job Queue
   
2. **Database Optimization**
   - ✅ **Connection Pooling implementiert** (MaxIdle=10, MaxOpen=100, Lifetime=1h)
   - DB Indizes optimieren
   - Bulk Inserts für Variants

3. **Multiple Hot Storage Pools**
   - 3-5 Hot Storage Pools für Lastverteilung
   - Round-Robin oder Capacity-based Selection

4. **Upload Optimierung**
   - Memory Buffer: 1MB → 4-8MB für große Dateien
   - Timeout-Handling verbessern

#### 7.2.2 Mittelfristige Optimierungen (1-3 Monate)

```yaml
Priority: Mittel
Aufwand: Mittel
Impact: Sehr Hoch
```

1. **Parallel Image Processing**
   ```go
   // Statt sequenziell:
   // processWebP() → processAVIF() → processThumbnails()
   
   // Parallel:
   var wg sync.WaitGroup
   wg.Add(3)
   go func() { defer wg.Done(); processWebP() }()
   go func() { defer wg.Done(); processAVIF() }() 
   go func() { defer wg.Done(); processThumbnails() }()
   wg.Wait()
   ```

2. **Upload Chunking**
   - Große Dateien (>10MB) in 2MB Chunks
   - Resumable Upload Support
   - Progressive Upload Status

3. **Auto-Tiering System**
   ```go
   // Automatic migration: Hot → Warm → Cold
   type TieringPolicy struct {
       HotToWarm    time.Duration // 30 days
       WarmToCold   time.Duration // 90 days
       ColdToArchive time.Duration // 365 days
   }
   ```

4. **CDN Integration**
   - CloudFront / CloudFlare für Bildauslieferung
   - Reduced load on Origin Server

#### 7.2.3 Langfristige Architektur (3-6 Monate)

```yaml
Priority: Niedrig
Aufwand: Hoch  
Impact: Sehr Hoch
```

1. **Horizontale Skalierung**
   ```yaml
   # Load Balancer Configuration
   Services:
     - Upload Service (2-3 Instanzen)
     - Processing Service (3-5 Instanzen)  
     - Storage Service (2-3 Instanzen)
   
   Database:
     - Master-Slave Setup
     - Read Replicas für Queries
   ```

2. **Microservice Architecture**
   - **Upload Service:** Nur für File Uploads
   - **Processing Service:** Image Processing Workers
   - **Storage Service:** Storage Pool Management
   - **API Gateway:** Routing und Rate Limiting

3. **Event-Driven Architecture**
   ```go
   // Event Bus (Redis Pub/Sub oder Message Queue)
   Events:
     - ImageUploaded
     - ImageProcessed
     - BackupCompleted
     - StoragePoolFull
   ```

### 7.3 Skalierung für tausende Nutzer

**Szenario: 1.000-5.000 aktive Nutzer, 200-500 gleichzeitige Uploads**

#### 7.3.1 Infrastructure Scaling

1. **Multi-Region Deployment**
   - EU-Central, US-East, Asia-Pacific
   - RegionLocal Storage Pools
   - Cross-Region Backup

2. **Database Sharding**
   ```sql
   -- Sharding Strategy
   Shard 1: user_id % 4 = 0
   Shard 2: user_id % 4 = 1  
   Shard 3: user_id % 4 = 2
   Shard 4: user_id % 4 = 3
   ```

3. **Dedicated Processing Cluster**
   - GPU-beschleunigte Image Processing
   - Separate Nodes für AVIF Processing
   - Auto-scaling basierend auf Queue Length

#### 7.3.2 Advanced Features

1. **AI-basierte Optimierung**
   - Smart Storage Tiering basierend auf Access Patterns
   - Predictive Caching
   - Automatische Kompression

2. **Advanced Monitoring**
   - Prometheus + Grafana
   - Storage Pool Health Monitoring
   - Automated Alerting

---

## 8. Monitoring & Observability

### 8.1 Aktuelle Monitoring-Capabilities

**Verfügbare Metriken:**
- Storage Pool Stats (`/admin/storage`)
- Job Queue Status (`/admin/queue`) 
- Basic Health Checks
- Error Logging

### 8.2 Verbesserungsvorschläge

```yaml
Metrics to Add:
  Upload:
    - upload_duration_seconds
    - upload_file_size_bytes
    - upload_errors_total
    - concurrent_uploads_active
  
  Processing:
    - image_processing_duration_seconds
    - variant_generation_duration_seconds  
    - processing_queue_length
    - processing_errors_total
    
  Storage:
    - storage_pool_usage_percent
    - storage_pool_iops
    - storage_pool_response_time_seconds
    
  S3 Backup:
    - backup_duration_seconds
    - backup_queue_length
    - backup_success_rate
    - s3_upload_bandwidth_mbps
```

### 8.3 Alerting Rules

```yaml
Critical Alerts:
  - Storage Pool > 90% full
  - Job Queue > 1000 pending jobs
  - Image Processing failing > 5%
  - S3 Backup failing > 10%
  
Warning Alerts:
  - Storage Pool > 80% full  
  - Average Processing Time > 2 minutes
  - S3 Upload Rate < expected
  - Database Connection Pool > 80%
```

---

## 9. Sicherheitsanalyse

### 9.1 Aktuelle Sicherheitsmaßnahmen

✅ **Implementiert:**
- File Type Validation
- IP Address Logging
- User Authentication Required
- CSRF Protection
- Storage Pool Path Validation

⚠️ **Verbesserungswürdig:**
- S3 Credentials im Klartext in DB
- Keine File Size Limits per User
- Keine Rate Limiting
- Keine Virus Scanning

### 9.2 Sicherheitsempfehlungen

```yaml
Priority 1 (Hoch):
  - Encrypt S3 Credentials in Database
  - Implement Rate Limiting (per User/IP)
  - Add File Size Limits per User Level
  - Image Content Validation (nicht nur Extension)

Priority 2 (Mittel):  
  - Virus/Malware Scanning Integration
  - Audit Logging für Admin Actions
  - Storage Access Control Lists
  - EXIF Data Sanitization

Priority 3 (Niedrig):
  - WAF Integration
  - DDoS Protection
  - Advanced Threat Detection
```

---

## 10. Kostenschätzung & ROI

### 10.1 Storage Kosten-Projektion

**Für 1000 aktive Nutzer (Annahme: 10 Uploads/User/Monat):**

```yaml
Monthly Uploads: 10,000
Average File Size: 5MB
Variants per Upload: 8x (3x sizes × 3x formats - duplicates)
Total Storage Growth: ~400GB/month

Storage Costs:
  Hot Storage (SSD): 200GB × $0.10/GB = $20/month
  Cold Storage (HDD): 200GB × $0.05/GB = $10/month  
  S3 Backup: 400GB × $0.023/GB = $9.20/month
  
Total Monthly Storage Cost: ~$40
```

### 10.2 Infrastructure Scaling Costs

| Nutzer | Uploads/Mon | Storage/Mon | Server Costs | S3 Costs | Total/Mon |
|--------|-------------|-------------|--------------|----------|-----------|
| 100 | 1,000 | 40GB | $50 | $1 | $55 |
| 1,000 | 10,000 | 400GB | $200 | $10 | $250 |  
| 5,000 | 50,000 | 2TB | $800 | $50 | $950 |
| 10,000 | 100,000 | 4TB | $1,500 | $100 | $1,700 |

---

## 11. Implementierungsroadmap

### 11.1 Phase 1: Immediate Optimizations (1-2 Wochen)

```yaml
Priority: Critical
Aufwand: 5-10 Entwicklertage

Tasks:
  1. Worker Count erhöhen (20 → 50)
  2. Multiple Hot Storage Pools konfigurieren  
  3. ✅ **Database Connection Pooling** (IMPLEMENTIERT)
  4. Memory Buffer Optimierung
  5. Basic Monitoring Dashboard
```

### 11.2 Phase 2: Core Scaling (1-2 Monate)

```yaml
Priority: Hoch
Aufwand: 20-30 Entwicklertage

Tasks:
  1. Parallel Image Processing
  2. Upload Chunking System
  3. Auto-Tiering Implementation
  4. CDN Integration
  5. Advanced Job Queue Monitoring
```

### 11.3 Phase 3: Architecture Evolution (3-6 Monate)

```yaml
Priority: Mittel  
Aufwand: 60-90 Entwicklertage

Tasks:
  1. Microservice Extraction
  2. Horizontal Scaling Setup
  3. Database Sharding
  4. Multi-Region Deployment
  5. AI-basierte Optimierungen
```

---

## 12. Fazit

Die aktuelle PixelFox Storage-Architektur ist **solid fundiert** und bietet eine **sehr gute Basis** für Skalierung. Das **moderne Multi-Tier Storage Pool System** mit **Redis Job Queue** und **flexiblem S3 Backup** ist state-of-the-art.

### Stärken:
✅ **Flexible Storage Tiers** (Hot/Warm/Cold/Archive)  
✅ **Multi-Provider S3 Support** (AWS, B2, MinIO)  
✅ **Redis-basierte Job Queue** (ersetzt in-memory)  
✅ **Umfangreiche Image Variants** (WebP, AVIF, Thumbnails)  
✅ **Konfigurierbare Worker Pools** (1-20 parallel)  

### Hauptschwächen:
❌ **Synchroner Upload Process** (Blocking)  
❌ **Sequential Image Processing** (nicht parallel)  
❌ **Hot Storage Saturation Risk** (alle Uploads → Hot)  
❌ **Limited Monitoring & Alerting**  
❌ **Single Database Instance** (kein Sharding)  
✅ **Optimized Connection Pooling** (MaxIdle=10, MaxOpen=100)  

### Skalierbarkeit:
- **Aktuelle Kapazität:** ~500-1000 Uploads/Stunde
- **Mit Phase 1 Optimierungen:** ~2000-3000 Uploads/Stunde  
- **Mit Phase 2 Optimierungen:** ~5000-8000 Uploads/Stunde
- **Mit Phase 3 Architecture:** Praktisch unbegrenzt skalierbar

Die Architektur ist **sehr gut positioniert** für Skalierung auf hunderte und tausende Nutzer mit den vorgeschlagenen Optimierungen.