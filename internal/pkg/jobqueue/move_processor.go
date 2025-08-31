package jobqueue

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2/log"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/internal/pkg/database"
	"github.com/ManuelReschke/PixelFox/internal/pkg/env"
	"github.com/ManuelReschke/PixelFox/internal/pkg/storage"
)

// processPoolMoveEnqueueJob scans images in source pool and enqueues per-image move jobs in batches
func (q *Queue) processPoolMoveEnqueueJob(job *Job) error {
	payload, err := PoolMoveEnqueueJobPayloadFromMap(job.Payload)
	if err != nil {
		return fmt.Errorf("invalid pool move enqueue payload: %w", err)
	}
	db := database.GetDB()
	if db == nil {
		return fmt.Errorf("database connection is nil")
	}
	const batchSize = 200
	var images []models.Image
	tx := db.Where("storage_pool_id = ? AND id > ?", payload.SourcePoolID, payload.CursorID).
		Order("id ASC").Limit(batchSize).Find(&images)
	if tx.Error != nil {
		return fmt.Errorf("failed to list images for pool %d: %w", payload.SourcePoolID, tx.Error)
	}
	if len(images) == 0 {
		log.Infof("[MoveEnqueue] No more images to enqueue from pool %d to %d", payload.SourcePoolID, payload.TargetPoolID)
		return nil
	}
	// Enqueue per-image move jobs
	for _, img := range images {
		p := MoveImageJobPayload{ImageID: img.ID, SourcePoolID: payload.SourcePoolID, TargetPoolID: payload.TargetPoolID}
		if _, err := q.EnqueueJob(JobTypeMoveImage, p.ToMap()); err != nil {
			log.Errorf("[MoveEnqueue] Failed to enqueue move job for image %d: %v", img.ID, err)
		}
	}
	// Re-enqueue enqueuer with next cursor if there might be more
	nextCursor := images[len(images)-1].ID
	next := PoolMoveEnqueueJobPayload{SourcePoolID: payload.SourcePoolID, TargetPoolID: payload.TargetPoolID, CursorID: nextCursor}
	if _, err := q.EnqueueJob(JobTypePoolMoveEnqueue, next.ToMap()); err != nil {
		log.Errorf("[MoveEnqueue] Failed to enqueue next batch: %v", err)
		// not fatal for this batch
	}
	return nil
}

// processMoveImageJob moves original and variants for a single image and updates DB references
func (q *Queue) processMoveImageJob(job *Job) error {
	payload, err := MoveImageJobPayloadFromMap(job.Payload)
	if err != nil {
		return fmt.Errorf("invalid move image payload: %w", err)
	}
	db := database.GetDB()
	if db == nil {
		return fmt.Errorf("database connection is nil")
	}
	// Load image, preloading current pool
	var image models.Image
	if err := db.Preload("StoragePool").First(&image, payload.ImageID).Error; err != nil {
		return fmt.Errorf("image not found: %w", err)
	}

	// Load pools
	srcPool, err := models.FindStoragePoolByID(db, payload.SourcePoolID)
	if err != nil || srcPool == nil {
		return fmt.Errorf("source pool not found")
	}
	tgtPool, err := models.FindStoragePoolByID(db, payload.TargetPoolID)
	if err != nil || tgtPool == nil {
		return fmt.Errorf("target pool not found")
	}

	// Route to correct node: move must run on source node
	nodeID := strings.TrimSpace(env.GetEnv("NODE_ID", ""))
	if nodeID != "" {
		poolNode := strings.TrimSpace(srcPool.NodeID)
		if poolNode != "" && !strings.EqualFold(nodeID, poolNode) {
			// Requeue for another node
			if err := q.requeueJob(context.Background(), job); err != nil {
				log.Errorf("[MoveImage] Failed to requeue job %s for node routing: %v", job.ID, err)
			} else {
				log.Infof("[MoveImage] Requeued job %s for node %s (current node %s)", job.ID, poolNode, nodeID)
			}
			return ErrRequeue
		}
	}

	sm := storage.NewStorageManager()

	// Sentinel error to indicate the source file is missing. Treated as non-fatal skip.
	var ErrSourceMissing = errors.New("source file missing")

	// Determine if target is on a different node (HTTP push replication)
	srcNode := strings.TrimSpace(srcPool.NodeID)
	tgtNode := strings.TrimSpace(tgtPool.NodeID)
	remoteTarget := (srcNode != "" && tgtNode != "" && !strings.EqualFold(srcNode, tgtNode))

	// Helper to copy then delete for a file with safety checks
	moveOne := func(relPath, fileName string, sourcePoolID, targetPoolID uint) error {
		rel := filepath.Clean(relPath)
		name := filepath.Clean(fileName)

		srcFull := filepath.Clean(filepath.Join(srcPool.BasePath, rel, name))
		tgtFull := filepath.Clean(filepath.Join(tgtPool.BasePath, rel, name))

		// If source and target resolve to the exact same path, skip IO (local single-folder setup)
		if strings.EqualFold(srcFull, tgtFull) {
			log.Infof("[MoveImage] Source and target are identical (%s), skipping IO and keeping file in place", srcFull)
			return nil
		}

		// Check source file exists and get size
		info, statErr := os.Stat(srcFull)
		if statErr != nil {
			if os.IsNotExist(statErr) {
				log.Warnf("[MoveImage] Source file not found, skipping: %s", srcFull)
				return ErrSourceMissing
			}
			return fmt.Errorf("stat source failed: %w", statErr)
		}

		// Remote replication flow
		if remoteTarget {
			f, err := os.Open(srcFull)
			if err != nil {
				return fmt.Errorf("open source failed: %w", err)
			}

			// Build replicate URL from upload_api_url
			repURL := strings.TrimSpace(tgtPool.UploadAPIURL)
			if repURL == "" {
				f.Close()
				return fmt.Errorf("target pool missing upload_api_url for replication")
			}
			repURL = strings.TrimRight(repURL, "/")
			if strings.HasSuffix(repURL, "/upload") {
				repURL = strings.TrimSuffix(repURL, "/upload") + "/replicate"
			} else {
				repURL = repURL + "/replicate"
			}

			pr, pw := io.Pipe()
			mw := multipart.NewWriter(pw)
			writerDone := make(chan struct{})

			go func() {
				defer close(writerDone)
				defer f.Close()
				// Note: Close order: Close the multipart writer before closing the pipe writer
				// so the reader sees EOF cleanly.
				defer pw.Close()
				defer mw.Close()

				_ = mw.WriteField("pool_id", strconv.FormatUint(uint64(targetPoolID), 10))
				_ = mw.WriteField("stored_path", filepath.Join(rel, name))
				_ = mw.WriteField("size", strconv.FormatInt(info.Size(), 10))

				part, err := mw.CreateFormFile("file", name)
				if err != nil {
					_ = pw.CloseWithError(err)
					return
				}
				hasher := sha256.New()
				tee := io.TeeReader(f, hasher)
				if _, err := io.Copy(part, tee); err != nil {
					_ = pw.CloseWithError(err)
					return
				}
				// Append checksum field after file content
				_ = mw.WriteField("sha256", hex.EncodeToString(hasher.Sum(nil)))
			}()

			client := &http.Client{Timeout: 300 * time.Second}
			req, err := http.NewRequest(http.MethodPut, repURL, pr)
			if err != nil {
				_ = pw.CloseWithError(err)
				<-writerDone
				return fmt.Errorf("create replicate request failed: %w", err)
			}
			req.Header.Set("Content-Type", mw.FormDataContentType())
			secret := strings.TrimSpace(env.GetEnv("REPLICATION_SECRET", ""))
			if secret == "" {
				_ = pw.CloseWithError(fmt.Errorf("missing replication secret"))
				<-writerDone
				return fmt.Errorf("REPLICATION_SECRET is not set")
			}
			req.Header.Set("Authorization", "Bearer "+secret)

			resp, err := client.Do(req)
			if err != nil {
				_ = pw.CloseWithError(err)
				<-writerDone
				return fmt.Errorf("replicate HTTP error: %w", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				// Try to read a short error body
				b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
				_ = pw.CloseWithError(fmt.Errorf("bad status %d", resp.StatusCode))
				<-writerDone
				return fmt.Errorf("replicate failed: status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
			}

			// Ensure writer goroutine finished cleanly
			<-writerDone

			// Remote stored successfully, delete local source
			if _, err := sm.DeleteFile(filepath.Join(rel, name), sourcePoolID); err != nil {
				return fmt.Errorf("delete from source failed: %w", err)
			}
			return nil
		}

		// Local filesystem move: open and stream to target pool
		f, err := os.Open(srcFull)
		if err != nil {
			return fmt.Errorf("open source failed: %w", err)
		}
		defer f.Close()

		if _, err = sm.SaveFile(f, filepath.Join(rel, name), targetPoolID); err != nil {
			return fmt.Errorf("save to target failed: %w", err)
		}
		if _, err := sm.DeleteFile(filepath.Join(rel, name), sourcePoolID); err != nil {
			return fmt.Errorf("delete from source failed: %w", err)
		}
		return nil
	}

	// Move original (required). If source missing, treat job as no-op success without DB update.
	if err := moveOne(image.FilePath, image.FileName, payload.SourcePoolID, payload.TargetPoolID); err != nil {
		if errors.Is(err, ErrSourceMissing) {
			// Do not retry this job; nothing to do if source is gone. Leave DB as-is.
			log.Warnf("[MoveImage] Original missing for image %d, leaving records unchanged", image.ID)
			return nil
		}
		return fmt.Errorf("move original failed: %w", err)
	}

	// Move variants
	variants, err := models.FindVariantsByImageID(db, image.ID)
	if err != nil {
		return fmt.Errorf("load variants failed: %w", err)
	}
	for i := range variants {
		v := &variants[i]
		// If variant already in target pool, skip
		vp := v.StoragePoolID
		if vp == 0 {
			vp = image.StoragePoolID
		}
		if vp == payload.TargetPoolID {
			continue
		}
		rel := v.FilePath
		// Normalize variant path to be relative to storage root
		if idx := strings.Index(rel, "variants"); idx >= 0 {
			rel = rel[idx:]
		} else {
			// Trim storage base path prefix if present
			base := strings.TrimRight(srcPool.BasePath, string(filepath.Separator)) + string(filepath.Separator)
			if strings.HasPrefix(rel, base) {
				rel = strings.TrimPrefix(rel, base)
			}
			rel = strings.TrimLeft(rel, string(filepath.Separator))
		}
		if err := moveOne(rel, v.FileName, payload.SourcePoolID, payload.TargetPoolID); err != nil {
			if errors.Is(err, ErrSourceMissing) {
				// Non-fatal: Variant is missing; continue with others
				log.Warnf("[MoveImage] Variant missing for image %d, type %s, skipping", image.ID, v.VariantType)
				continue
			}
			return fmt.Errorf("move variant failed: %w", err)
		}
	}

	// Update DB references in a transaction
	tx := db.Begin()
	if err := tx.Model(&models.Image{}).Where("id = ?", image.ID).Update("storage_pool_id", payload.TargetPoolID).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("update image pool failed: %w", err)
	}
	if len(variants) > 0 {
		if err := tx.Model(&models.ImageVariant{}).Where("image_id = ?", image.ID).Update("storage_pool_id", payload.TargetPoolID).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("update variants pool failed: %w", err)
		}
	}
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("commit failed: %w", err)
	}

	log.Infof("[MoveImage] Moved image %d from pool %d to %d", image.ID, payload.SourcePoolID, payload.TargetPoolID)
	return nil
}
