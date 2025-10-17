package jobqueue

import (
	"encoding/json"
	"time"

	"github.com/ManuelReschke/PixelFox/app/models"
)

// JobType defines the type of job
type JobType string

const (
	JobTypeImageProcessing   JobType = "image_processing"
	JobTypeS3Backup          JobType = "s3_backup"
	JobTypeS3Delete          JobType = "s3_delete"
	JobTypePoolMoveEnqueue   JobType = "pool_move_enqueue"
	JobTypeMoveImage         JobType = "move_image"
	JobTypeDeleteImage       JobType = "delete_image"
	JobTypeReconcileVariants JobType = "reconcile_variants"
)

// JobStatus defines the status of a job
type JobStatus string

const (
	JobStatusPending    JobStatus = "pending"
	JobStatusProcessing JobStatus = "processing"
	JobStatusCompleted  JobStatus = "completed"
	JobStatusFailed     JobStatus = "failed"
	JobStatusRetrying   JobStatus = "retrying"
)

// Job represents a background job
type Job struct {
	ID          string                 `json:"id"`
	Type        JobType                `json:"type"`
	Status      JobStatus              `json:"status"`
	Payload     map[string]interface{} `json:"payload"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	ProcessedAt *time.Time             `json:"processed_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	ErrorMsg    string                 `json:"error_msg,omitempty"`
	RetryCount  int                    `json:"retry_count"`
	MaxRetries  int                    `json:"max_retries"`
}

// ImageProcessingJobPayload contains the payload for image processing jobs
type ImageProcessingJobPayload struct {
	ImageID      uint   `json:"image_id"`
	ImageUUID    string `json:"image_uuid"`
	FilePath     string `json:"file_path"`     // Original file path
	FileName     string `json:"file_name"`     // Original file name
	FileType     string `json:"file_type"`     // File extension (.jpg, .png, etc.)
	EnableBackup bool   `json:"enable_backup"` // Whether to trigger S3 backup after processing
	PoolID       uint   `json:"pool_id"`       // Storage pool ID (routing hint)
	NodeID       string `json:"node_id"`       // Optional node ID (routing hint)
}

// ToMap converts the payload to a map for storage
func (p ImageProcessingJobPayload) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"image_id":      p.ImageID,
		"image_uuid":    p.ImageUUID,
		"file_path":     p.FilePath,
		"file_name":     p.FileName,
		"file_type":     p.FileType,
		"enable_backup": p.EnableBackup,
		"pool_id":       p.PoolID,
		"node_id":       p.NodeID,
	}
}

// FromMap creates a payload from a map
func ImageProcessingJobPayloadFromMap(data map[string]interface{}) (*ImageProcessingJobPayload, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	var payload ImageProcessingJobPayload
	err = json.Unmarshal(jsonData, &payload)
	return &payload, err
}

// S3BackupJobPayload contains the payload for S3 backup jobs
type S3BackupJobPayload struct {
	ImageID   uint                  `json:"image_id"`
	ImageUUID string                `json:"image_uuid"`
	FilePath  string                `json:"file_path"`
	FileName  string                `json:"file_name"`
	FileSize  int64                 `json:"file_size"`
	Provider  models.BackupProvider `json:"provider"`
	BackupID  uint                  `json:"backup_id"`
}

// ToMap converts the payload to a map for storage
func (p S3BackupJobPayload) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"image_id":   p.ImageID,
		"image_uuid": p.ImageUUID,
		"file_path":  p.FilePath,
		"file_name":  p.FileName,
		"file_size":  p.FileSize,
		"provider":   string(p.Provider),
		"backup_id":  p.BackupID,
	}
}

// FromMap creates a payload from a map
func S3BackupJobPayloadFromMap(data map[string]interface{}) (*S3BackupJobPayload, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	var payload S3BackupJobPayload
	err = json.Unmarshal(jsonData, &payload)
	return &payload, err
}

// S3DeleteJobPayload contains the payload for S3 delete jobs
type S3DeleteJobPayload struct {
	ImageID    uint                  `json:"image_id"`
	ImageUUID  string                `json:"image_uuid"`
	ObjectKey  string                `json:"object_key"`
	BucketName string                `json:"bucket_name"`
	Provider   models.BackupProvider `json:"provider"`
	BackupID   uint                  `json:"backup_id"`
}

// ToMap converts the payload to a map for storage
func (p S3DeleteJobPayload) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"image_id":    p.ImageID,
		"image_uuid":  p.ImageUUID,
		"object_key":  p.ObjectKey,
		"bucket_name": p.BucketName,
		"provider":    string(p.Provider),
		"backup_id":   p.BackupID,
	}
}

// FromMap creates a delete payload from a map
func S3DeleteJobPayloadFromMap(data map[string]interface{}) (*S3DeleteJobPayload, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	var payload S3DeleteJobPayload
	err = json.Unmarshal(jsonData, &payload)
	return &payload, err
}

// PoolMoveEnqueueJobPayload contains payload for scanning a source pool and enqueuing per-image move jobs
type PoolMoveEnqueueJobPayload struct {
	SourcePoolID uint `json:"source_pool_id"`
	TargetPoolID uint `json:"target_pool_id"`
	CursorID     uint `json:"cursor_id"` // last processed Image.ID; 0 = start
}

func (p PoolMoveEnqueueJobPayload) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"source_pool_id": p.SourcePoolID,
		"target_pool_id": p.TargetPoolID,
		"cursor_id":      p.CursorID,
	}
}

func PoolMoveEnqueueJobPayloadFromMap(data map[string]interface{}) (*PoolMoveEnqueueJobPayload, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	var payload PoolMoveEnqueueJobPayload
	err = json.Unmarshal(jsonData, &payload)
	return &payload, err
}

// MoveImageJobPayload contains payload for moving a single image+variants between pools
type MoveImageJobPayload struct {
	ImageID      uint `json:"image_id"`
	SourcePoolID uint `json:"source_pool_id"`
	TargetPoolID uint `json:"target_pool_id"`
}

func (p MoveImageJobPayload) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"image_id":       p.ImageID,
		"source_pool_id": p.SourcePoolID,
		"target_pool_id": p.TargetPoolID,
	}
}

func MoveImageJobPayloadFromMap(data map[string]interface{}) (*MoveImageJobPayload, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	var payload MoveImageJobPayload
	err = json.Unmarshal(jsonData, &payload)
	return &payload, err
}

// ReconcileVariantsJobPayload contains payload for moving late-created variants to the image's current pool
type ReconcileVariantsJobPayload struct {
	ImageID      uint   `json:"image_id"`
	ImageUUID    string `json:"image_uuid"`
	TargetPoolID uint   `json:"target_pool_id"`
}

func (p ReconcileVariantsJobPayload) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"image_id":       p.ImageID,
		"image_uuid":     p.ImageUUID,
		"target_pool_id": p.TargetPoolID,
	}
}

func ReconcileVariantsJobPayloadFromMap(data map[string]interface{}) (*ReconcileVariantsJobPayload, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	var payload ReconcileVariantsJobPayload
	err = json.Unmarshal(jsonData, &payload)
	return &payload, err
}

// DeleteImageJobPayload contains payload for deleting an image and its variants/files asynchronously
type DeleteImageJobPayload struct {
	ImageID       uint   `json:"image_id"`
	ImageUUID     string `json:"image_uuid"`
	FromReportID  *uint  `json:"from_report_id,omitempty"`
	InitiatedByID *uint  `json:"initiated_by_id,omitempty"`
}

func (p DeleteImageJobPayload) ToMap() map[string]interface{} {
	m := map[string]interface{}{
		"image_id":   p.ImageID,
		"image_uuid": p.ImageUUID,
	}
	if p.FromReportID != nil {
		m["from_report_id"] = *p.FromReportID
	}
	if p.InitiatedByID != nil {
		m["initiated_by_id"] = *p.InitiatedByID
	}
	return m
}

func DeleteImageJobPayloadFromMap(data map[string]interface{}) (*DeleteImageJobPayload, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	var payload DeleteImageJobPayload
	err = json.Unmarshal(jsonData, &payload)
	return &payload, err
}

// IsRetryable checks if the job can be retried
func (j *Job) IsRetryable() bool {
	return j.Status == JobStatusFailed && j.RetryCount < j.MaxRetries
}

// MarkAsProcessing updates the job status to processing
func (j *Job) MarkAsProcessing() {
	now := time.Now()
	j.Status = JobStatusProcessing
	j.UpdatedAt = now
	j.ProcessedAt = &now
}

// MarkAsCompleted updates the job status to completed
func (j *Job) MarkAsCompleted() {
	now := time.Now()
	j.Status = JobStatusCompleted
	j.UpdatedAt = now
	j.CompletedAt = &now
	j.ErrorMsg = ""
}

// MarkAsFailed updates the job status to failed
func (j *Job) MarkAsFailed(errorMsg string) {
	j.Status = JobStatusFailed
	j.UpdatedAt = time.Now()
	j.ErrorMsg = errorMsg
	j.RetryCount++
}

// MarkAsRetrying updates the job status to retrying
func (j *Job) MarkAsRetrying() {
	j.Status = JobStatusRetrying
	j.UpdatedAt = time.Now()
}
