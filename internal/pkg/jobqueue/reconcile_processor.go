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

		srcFull := filepath.Clean(filepath.Join(srcPool.BasePath, rel, name))
		tgtFull := filepath.Clean(filepath.Join(tgtPool.BasePath, rel, name))

		if strings.EqualFold(srcFull, tgtFull) {
			return nil
		}

		info, statErr := os.Stat(srcFull)
		if statErr != nil {
			if os.IsNotExist(statErr) {
				log.Warnf("[Reconcile] Source variant missing, skipping: %s", srcFull)
				return nil
			}
			return fmt.Errorf("stat source failed: %w", statErr)
		}

		srcNode := strings.TrimSpace(srcPool.NodeID)
		tgtNode := strings.TrimSpace(tgtPool.NodeID)
		remoteTarget := (srcNode != "" && tgtNode != "" && !strings.EqualFold(srcNode, tgtNode))

		if remoteTarget {
			f, err := os.Open(srcFull)
			if err != nil {
				return fmt.Errorf("open source failed: %w", err)
			}
			pr, pw := io.Pipe()
			mw := multipart.NewWriter(pw)
			writerDone := make(chan struct{})
			go func() {
				defer close(writerDone)
				defer f.Close()
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
				_ = mw.WriteField("sha256", hex.EncodeToString(hasher.Sum(nil)))
			}()
			repURL := strings.TrimSpace(tgtPool.UploadAPIURL)
			if repURL == "" {
				_ = pw.CloseWithError(fmt.Errorf("missing upload_api_url"))
				<-writerDone
				return fmt.Errorf("target pool missing upload_api_url")
			}
			repURL = strings.TrimRight(repURL, "/")
			if strings.HasSuffix(repURL, "/upload") {
				repURL = strings.TrimSuffix(repURL, "/upload") + "/replicate"
			} else {
				repURL = repURL + "/replicate"
			}
			client := &http.Client{Timeout: 300 * time.Second}
			req, err := http.NewRequest(http.MethodPut, repURL, pr)
			if err != nil {
				_ = pw.CloseWithError(err)
				<-writerDone
				return fmt.Errorf("create request failed: %w", err)
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
				_ = pw.CloseWithError(fmt.Errorf("bad status %d", resp.StatusCode))
				<-writerDone
				return fmt.Errorf("replicate failed: status %d", resp.StatusCode)
			}
			<-writerDone
			if _, err := sm.DeleteFile(filepath.Join(rel, name), srcPool.ID); err != nil {
				return fmt.Errorf("delete from source failed: %w", err)
			}
			return nil
		}

		// Local move
		f, err := os.Open(srcFull)
		if err != nil {
			return fmt.Errorf("open source failed: %w", err)
		}
		defer f.Close()
		if _, err = sm.SaveFile(f, filepath.Join(rel, name), targetPoolID); err != nil {
			return fmt.Errorf("save to target failed: %w", err)
		}
		if _, err := sm.DeleteFile(filepath.Join(rel, name), srcPool.ID); err != nil {
			return fmt.Errorf("delete from source failed: %w", err)
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
			if poolNode != "" && !strings.EqualFold(nodeID, poolNode) {
				// Requeue for source node
				if err := q.requeueJob(context.Background(), job); err != nil {
					log.Errorf("[Reconcile] Failed to requeue job %s for node routing: %v", job.ID, err)
				}
				return ErrRequeue
			}
		}

		// Normalize variant relative path
		rel := v.FilePath
		if idx := strings.Index(rel, "variants"); idx >= 0 {
			rel = rel[idx:]
		} else {
			base := strings.TrimRight(srcPool.BasePath, string(filepath.Separator)) + string(filepath.Separator)
			if strings.HasPrefix(rel, base) {
				rel = strings.TrimPrefix(rel, base)
			}
			rel = strings.TrimLeft(rel, string(filepath.Separator))
		}

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
