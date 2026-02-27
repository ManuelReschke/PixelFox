package jobqueue

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/ManuelReschke/PixelFox/app/models"
)

type settingBackup struct {
	exists bool
	value  string
	typ    string
}

func setSettingsForTieringTest(t *testing.T, db *gorm.DB, values map[string]string) {
	t.Helper()

	backups := make(map[string]settingBackup, len(values))
	for key := range values {
		var setting models.Setting
		err := db.Where("setting_key = ?", key).First(&setting).Error
		if err == nil {
			backups[key] = settingBackup{exists: true, value: setting.Value, typ: setting.Type}
			continue
		}
		if err == gorm.ErrRecordNotFound {
			backups[key] = settingBackup{exists: false}
			continue
		}
		require.NoError(t, err)
	}

	for key, value := range values {
		require.NoError(t, db.Exec(
			"INSERT INTO settings (setting_key, value, type, created_at, updated_at) VALUES (?, ?, 'integer', NOW(), NOW()) "+
				"ON DUPLICATE KEY UPDATE value = VALUES(value), updated_at = NOW()",
			key, value,
		).Error)
	}

	require.NoError(t, db.Exec(
		"INSERT INTO settings (setting_key, value, type, created_at, updated_at) VALUES ('tiering_enabled', 'true', 'boolean', NOW(), NOW()) "+
			"ON DUPLICATE KEY UPDATE value = 'true', type = 'boolean', updated_at = NOW()",
	).Error)
	require.NoError(t, models.LoadSettings(db))

	t.Cleanup(func() {
		for key, backup := range backups {
			if backup.exists {
				_ = db.Exec(
					"UPDATE settings SET value = ?, type = ?, updated_at = NOW() WHERE setting_key = ?",
					backup.value, backup.typ, key,
				).Error
			} else {
				_ = db.Exec("DELETE FROM settings WHERE setting_key = ?", key).Error
			}
		}
		_ = db.Exec(
			"UPDATE settings SET value = ?, type = 'boolean', updated_at = NOW() WHERE setting_key = 'tiering_enabled'",
			"true",
		).Error
		_ = models.LoadSettings(db)
	})
}

func setPoolsActiveByTier(t *testing.T, db *gorm.DB, tier string, active bool, excludeIDs ...uint) {
	t.Helper()

	var pools []models.StoragePool
	q := db.Where("storage_tier = ?", tier)
	if len(excludeIDs) > 0 {
		q = q.Not("id IN ?", excludeIDs)
	}
	require.NoError(t, q.Find(&pools).Error)

	previousState := make(map[uint]bool, len(pools))
	for _, p := range pools {
		previousState[p.ID] = p.IsActive
		require.NoError(t, db.Model(&models.StoragePool{}).Where("id = ?", p.ID).UpdateColumn("is_active", active).Error)
	}

	t.Cleanup(func() {
		for id, wasActive := range previousState {
			_ = db.Model(&models.StoragePool{}).Where("id = ?", id).UpdateColumn("is_active", wasActive).Error
		}
	})
}

func TestManager_runTieringSweepOnce_EnqueuesMoveToColdS3(t *testing.T) {
	queue, db, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Keep test deterministic across shared dev DB contents.
	setSettingsForTieringTest(t, db, map[string]string{
		"hot_keep_days_after_upload":       "1",
		"demote_if_no_views_days":          "1",
		"max_tiering_candidates_per_sweep": "1",
		"hot_watermark_high":               "100",
	})

	hotPool := &models.StoragePool{
		Name:        "tiering-hot-" + uuid.NewString(),
		BasePath:    t.TempDir(),
		MaxSize:     1 << 41, // 2 TiB
		UsedSize:    0,
		IsActive:    true,
		IsDefault:   false,
		Priority:    -100, // Ensure this pool is processed first
		StorageType: models.StorageTypeLocal,
		StorageTier: models.StorageTierHot,
		Description: "tiering test hot pool",
	}
	require.NoError(t, db.Create(hotPool).Error)

	accessKey := "tiering-test-ak"
	secretKey := "tiering-test-sk"
	region := "us-west-001"
	bucket := "tiering-test-" + uuid.NewString()
	endpoint := "https://s3.us-west-001.backblazeb2.com"
	prefix := ""
	coldPool := &models.StoragePool{
		Name:              "tiering-cold-s3-" + uuid.NewString(),
		BasePath:          "s3://tiering",
		MaxSize:           1 << 50, // 1 PiB
		UsedSize:          0,
		IsActive:          true,
		IsDefault:         false,
		Priority:          10,
		StorageType:       models.StorageTypeS3,
		StorageTier:       models.StorageTierCold,
		Description:       "tiering test cold s3 pool",
		S3AccessKeyID:     &accessKey,
		S3SecretAccessKey: &secretKey,
		S3Region:          &region,
		S3BucketName:      &bucket,
		S3EndpointURL:     &endpoint,
		S3PathPrefix:      &prefix,
	}
	require.NoError(t, db.Create(coldPool).Error)

	setPoolsActiveByTier(t, db, models.StorageTierWarm, false)
	setPoolsActiveByTier(t, db, models.StorageTierHot, false, hotPool.ID)
	setPoolsActiveByTier(t, db, models.StorageTierCold, false, coldPool.ID)

	old := time.Now().AddDate(0, 0, -14)
	lastViewed := old
	img := &models.Image{
		UUID:          uuid.NewString(),
		UserID:        1,
		FilePath:      "uploads/original/2026/02/26",
		FileName:      "tiering-test.jpg",
		FileSize:      int64(400) * 1024 * 1024 * 1024, // > default warm pool sizes
		FileType:      "image/jpeg",
		Width:         1280,
		Height:        720,
		FileHash:      uuid.NewString(),
		StoragePoolID: hotPool.ID,
		CreatedAt:     old,
		UpdatedAt:     old,
		LastViewedAt:  &lastViewed,
	}
	require.NoError(t, db.Create(img).Error)
	require.NoError(t, db.Model(&models.Image{}).Where("id = ?", img.ID).Updates(map[string]interface{}{
		"created_at":     old,
		"updated_at":     old,
		"last_viewed_at": lastViewed,
	}).Error)

	t.Cleanup(func() {
		_ = db.Unscoped().Delete(&models.Image{}, img.ID).Error
		_ = db.Delete(&models.StoragePool{}, coldPool.ID).Error
		_ = db.Delete(&models.StoragePool{}, hotPool.ID).Error
	})

	manager := &Manager{queue: queue}
	require.NoError(t, manager.runTieringSweepOnce())

	ctx := context.Background()
	queueSize, err := queue.GetQueueSize(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(1), queueSize)

	job, err := queue.dequeueJob(ctx)
	require.NoError(t, err)
	require.NotNil(t, job)
	assert.Equal(t, JobTypeMoveImage, job.Type)

	payload, err := MoveImageJobPayloadFromMap(job.Payload)
	require.NoError(t, err)
	assert.Equal(t, img.ID, payload.ImageID)
	assert.Equal(t, hotPool.ID, payload.SourcePoolID)
	assert.Equal(t, coldPool.ID, payload.TargetPoolID)
}
