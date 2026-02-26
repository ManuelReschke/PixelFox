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
	"github.com/ManuelReschke/PixelFox/internal/pkg/imageprocessor"
	"github.com/ManuelReschke/PixelFox/internal/pkg/storage"
)

// processReconcileVariantsJob moves any variants that were created after a move to the image's current pool
func (q *Queue) processReconcileVariantsJob(job *Job) error {
	payload, err := ReconcileVariantsJobPayloadFromMap(job.Payload)
	if err != nil {
		return fmt.Errorf("invalid reconcile variants payload: %w", err)
	}
	db := database.GetDB()
	if db == nil {
		return fmt.Errorf("database connection is nil")
	}

	// Load image and current pool
	var image models.Image
	if err := db.Preload("StoragePool").First(&image, payload.ImageID).Error; err != nil {
		return fmt.Errorf("image not found: %w", err)
	}

	// Wait until processing is completed to catch late variants
	uuid := image.UUID
	if uuid == "" {
		uuid = payload.ImageUUID
	}
	if uuid == "" || !imageprocessor.IsImageProcessingComplete(uuid) {
		return fmt.Errorf("image processing not complete yet")
	}

	// Determine target pool (from image)
	targetPoolID := image.StoragePoolID
	if payload.TargetPoolID > 0 {
		targetPoolID = payload.TargetPoolID
	}
	tgtPool, err := models.FindStoragePoolByID(db, targetPoolID)
	if err != nil || tgtPool == nil {
		return fmt.Errorf("target pool not found")
	}

	// List variants whose pool differs from image's current pool
	var variants []models.ImageVariant
	if err := db.Where("image_id = ? AND (storage_pool_id IS NULL OR storage_pool_id = 0 OR storage_pool_id != ?)", image.ID, targetPoolID).Find(&variants).Error; err != nil {
		return fmt.Errorf("list variants to reconcile failed: %w", err)
	}
	if len(variants) == 0 {
		log.Infof("[Reconcile] No variants to reconcile for image %d", image.ID)
		return nil
	}

	// Helper to move a single file between pools (local or remote)
	sm := storage.NewStorageManager()
	moveOne := func(srcPool *models.StoragePool, relPath, fileName string, targetPoolID uint) error {
		rel := filepath.Clean(relPath)
		name := filepath.Clean(fileName)
		storedPath := filepath.ToSlash(filepath.Join(rel, name))
		storedPath = strings.TrimLeft(storedPath, "/")

		exists, _, existsErr := sm.FileExists(storedPath, srcPool.ID)
		if existsErr != nil {
			return fmt.Errorf("source existence check failed: %w", existsErr)
		}
		if !exists {
			return os.ErrNotExist
		}

		srcNode := strings.TrimSpace(srcPool.NodeID)
		tgtNode := strings.TrimSpace(tgtPool.NodeID)
		remoteTarget := isLocalLikeStoragePool(srcPool) &&
			isLocalLikeStoragePool(tgtPool) &&
			srcNode != "" && tgtNode != "" &&
			!strings.EqualFold(srcNode, tgtNode)

		if remoteTarget {
			srcFull, err := sm.GetFilePath(storedPath, srcPool.ID)
			if err != nil {
				return fmt.Errorf("resolve source path failed: %w", err)
			}
			if err := replicateFileToRemotePool(srcFull, storedPath, targetPoolID, tgtPool.UploadAPIURL); err != nil {
				return err
			}
			if _, err := sm.DeleteFile(storedPath, srcPool.ID); err != nil {
				return fmt.Errorf("delete from source failed: %w", err)
			}
			return nil
		}

		if _, err := sm.MigrateFile(storedPath, srcPool.ID, targetPoolID); err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "source file not found") {
				return os.ErrNotExist
			}
			return fmt.Errorf("migrate failed: %w", err)
		}
		return nil
	}

	moved := 0
	for i := range variants {
		v := &variants[i]
		// identify source pool for this variant
		sourcePoolID := v.StoragePoolID
		if sourcePoolID == 0 {
			sourcePoolID = image.StoragePoolID
		}
		srcPool, err := models.FindStoragePoolByID(db, sourcePoolID)
		if err != nil || srcPool == nil {
			log.Warnf("[Reconcile] Source pool missing for variant %d of image %d", v.ID, image.ID)
			continue
		}
		// Node routing: ensure we run on source node
		nodeID := strings.TrimSpace(env.GetEnv("NODE_ID", ""))
		if nodeID != "" {
			poolNode := strings.TrimSpace(srcPool.NodeID)
			if !isLocalLikeStoragePool(srcPool) && isLocalLikeStoragePool(tgtPool) {
				poolNode = strings.TrimSpace(tgtPool.NodeID)
			}
			if poolNode != "" && !strings.EqualFold(nodeID, poolNode) {
				// Requeue for source node
				if err := q.requeueJob(context.Background(), job); err != nil {
					log.Errorf("[Reconcile] Failed to requeue job %s for node routing: %v", job.ID, err)
				}
				return ErrRequeue
			}
		}

		// Normalize variant relative path
		rel := normalizeVariantRelativePath(v.FilePath, srcPool)

		if err := moveOne(srcPool, rel, v.FileName, targetPoolID); err != nil {
			// Non-fatal for missing sources; otherwise fail to retry
			if errors.Is(err, os.ErrNotExist) {
				log.Warnf("[Reconcile] Variant source missing (image %d): %v", image.ID, err)
				continue
			}
			return fmt.Errorf("move variant failed: %w", err)
		}

		// Update variant pool
		if err := db.Model(&models.ImageVariant{}).Where("id = ?", v.ID).Update("storage_pool_id", targetPoolID).Error; err != nil {
			return fmt.Errorf("update variant pool failed: %w", err)
		}
		moved++
	}

	if moved > 0 {
		log.Infof("[Reconcile] Moved %d late variants for image %d to pool %d", moved, image.ID, targetPoolID)
	} else {
		log.Infof("[Reconcile] No variants needed moving for image %d", image.ID)
	}
	return nil
}
