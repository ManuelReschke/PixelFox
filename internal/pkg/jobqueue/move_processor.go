package jobqueue

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v2/log"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/internal/pkg/database"
	"github.com/ManuelReschke/PixelFox/internal/pkg/env"
	"github.com/ManuelReschke/PixelFox/internal/pkg/storage"
	"gorm.io/gorm"
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
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Image was deleted or no longer available; treat as a no-op and do not retry
			log.Warnf("[MoveImage] Image %d not found; skipping job %s", payload.ImageID, job.ID)
			return nil
		}
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
	if nodeID != "" && isLocalLikeStoragePool(srcPool) {
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

	// If source is object storage and target is local/NFS, route to target node.
	if nodeID != "" && !isLocalLikeStoragePool(srcPool) && isLocalLikeStoragePool(tgtPool) {
		targetNode := strings.TrimSpace(tgtPool.NodeID)
		if targetNode != "" && !strings.EqualFold(nodeID, targetNode) {
			if err := q.requeueJob(context.Background(), job); err != nil {
				log.Errorf("[MoveImage] Failed to requeue job %s for target node routing: %v", job.ID, err)
			} else {
				log.Infof("[MoveImage] Requeued job %s for target node %s (current node %s)", job.ID, targetNode, nodeID)
			}
			return ErrRequeue
		}
	}

	sm := storage.NewStorageManager()
	var errSourceMissing = errors.New("source file missing")

	// Determine if target is on a different node and requires HTTP push replication.
	srcNode := strings.TrimSpace(srcPool.NodeID)
	tgtNode := strings.TrimSpace(tgtPool.NodeID)
	remoteTarget := isLocalLikeStoragePool(srcPool) &&
		isLocalLikeStoragePool(tgtPool) &&
		srcNode != "" && tgtNode != "" &&
		!strings.EqualFold(srcNode, tgtNode)

	// Helper to copy then delete for a file with safety checks
	moveOne := func(relPath, fileName string, sourcePoolID, targetPoolID uint) error {
		sourcePool := srcPool
		if sourcePoolID != srcPool.ID {
			p, err := models.FindStoragePoolByID(db, sourcePoolID)
			if err != nil || p == nil {
				return fmt.Errorf("source pool %d not found", sourcePoolID)
			}
			sourcePool = p
		}
		targetPool := tgtPool
		if targetPoolID != tgtPool.ID {
			p, err := models.FindStoragePoolByID(db, targetPoolID)
			if err != nil || p == nil {
				return fmt.Errorf("target pool %d not found", targetPoolID)
			}
			targetPool = p
		}

		rel := filepath.Clean(relPath)
		name := filepath.Clean(fileName)
		storedPath := filepath.ToSlash(filepath.Join(rel, name))
		storedPath = strings.TrimLeft(storedPath, "/")

		exists, _, existsErr := sm.FileExists(storedPath, sourcePoolID)
		if existsErr != nil {
			return fmt.Errorf("check source existence failed: %w", existsErr)
		}
		if !exists {
			log.Warnf("[MoveImage] Source file not found, skipping: pool=%d path=%s", sourcePoolID, storedPath)
			return errSourceMissing
		}

		// Remote replication flow
		callRemote := remoteTarget
		if sourcePoolID != srcPool.ID || targetPoolID != tgtPool.ID {
			sourceNode := strings.TrimSpace(sourcePool.NodeID)
			targetNode := strings.TrimSpace(targetPool.NodeID)
			callRemote = isLocalLikeStoragePool(sourcePool) &&
				isLocalLikeStoragePool(targetPool) &&
				sourceNode != "" && targetNode != "" &&
				!strings.EqualFold(sourceNode, targetNode)
		}

		if callRemote {
			srcFull, err := sm.GetFilePath(storedPath, sourcePoolID)
			if err != nil {
				return fmt.Errorf("resolve source path failed: %w", err)
			}
			if err := replicateFileToRemotePool(srcFull, storedPath, targetPoolID, targetPool.UploadAPIURL); err != nil {
				return err
			}
			// Remote stored successfully, delete local source
			if _, err := sm.DeleteFile(storedPath, sourcePoolID); err != nil {
				return fmt.Errorf("delete from source failed: %w", err)
			}
			return nil
		}

		if _, err := sm.MigrateFile(storedPath, sourcePoolID, targetPoolID); err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "source file not found") || errors.Is(err, os.ErrNotExist) {
				return errSourceMissing
			}
			return err
		}
		return nil
	}

	// Move original (required). If source missing, treat job as no-op success without DB update.
	if err := moveOne(image.FilePath, image.FileName, payload.SourcePoolID, payload.TargetPoolID); err != nil {
		if errors.Is(err, errSourceMissing) {
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
		rel := normalizeVariantRelativePath(v.FilePath, srcPool)
		if err := moveOne(rel, v.FileName, vp, payload.TargetPoolID); err != nil {
			if errors.Is(err, errSourceMissing) {
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

	// Enqueue a reconciliation job to move any late-created variants after processing completes
	if _, err := q.EnqueueJob(JobTypeReconcileVariants, ReconcileVariantsJobPayload{
		ImageID:      image.ID,
		ImageUUID:    image.UUID,
		TargetPoolID: payload.TargetPoolID,
	}.ToMap()); err != nil {
		log.Warnf("[MoveImage] Failed to enqueue reconcile variants job for image %d: %v", image.ID, err)
	}
	return nil
}
