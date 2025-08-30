package controllers

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/sujit-baniya/flash"
	"gorm.io/gorm"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/app/repository"
	"github.com/ManuelReschke/PixelFox/internal/pkg/usercontext"
	"github.com/ManuelReschke/PixelFox/views"
	"github.com/ManuelReschke/PixelFox/views/admin_views"
)

// ============================================================================
// ADMIN STORAGE CONTROLLER - Repository Pattern
// ============================================================================

// AdminStorageController handles admin storage-related HTTP requests using repository pattern
type AdminStorageController struct {
	storagePoolRepo repository.StoragePoolRepository
}

// NewAdminStorageController creates a new admin storage controller with repository
func NewAdminStorageController(storagePoolRepo repository.StoragePoolRepository) *AdminStorageController {
	return &AdminStorageController{
		storagePoolRepo: storagePoolRepo,
	}
}

// handleError is a helper method for consistent error handling
func (asc *AdminStorageController) handleError(c *fiber.Ctx, message string, err error) error {
	fm := fiber.Map{
		"type":    "error",
		"message": message + ": " + err.Error(),
	}
	return flash.WithError(c, fm).Redirect("/admin/storage")
}

// HandleAdminStorageManagement renders the storage management dashboard using repository pattern
func (asc *AdminStorageController) HandleAdminStorageManagement(c *fiber.Ctx) error {
	userCtx := usercontext.GetUserContext(c)
	// Get all storage pool statistics using repository
	poolStats, err := asc.storagePoolRepo.GetAllStats()
	if err != nil {
		fm := fiber.Map{
			"type":    "error",
			"message": "Fehler beim Laden der Speicher-Statistiken: " + err.Error(),
		}
		flash.WithError(c, fm)
		poolStats = []models.StoragePoolStats{}
	}

	// Perform health checks using repository
	healthStatus, err := asc.storagePoolRepo.GetHealthStatus()
	if err != nil {
		fm := fiber.Map{
			"type":    "error",
			"message": "Fehler beim Gesundheitscheck: " + err.Error(),
		}
		flash.WithError(c, fm)
		healthStatus = make(map[uint]bool)
	}

	// Extended heartbeat snapshots
	snapshots, _ := asc.storagePoolRepo.GetHealthSnapshots()

	// Get all storage pools for management using repository
	pools, err := asc.storagePoolRepo.GetAll()
	if err != nil {
		fm := fiber.Map{
			"type":    "error",
			"message": "Fehler beim Laden der Speicherpools: " + err.Error(),
		}
		flash.WithError(c, fm)
		pools = []models.StoragePool{}
	}

	// Calculate total statistics
	totalUsedSize := int64(0)
	totalMaxSize := int64(0)
	totalImageCount := int64(0)
	totalVariantCount := int64(0)
	healthyPoolsCount := 0

	for _, stats := range poolStats {
		totalUsedSize += stats.UsedSize
		totalMaxSize += stats.MaxSize
		totalImageCount += stats.ImageCount
		totalVariantCount += stats.VariantCount

		if healthy, exists := healthStatus[stats.ID]; exists && healthy {
			healthyPoolsCount++
		}
	}

	totalUsagePercentage := float64(0)
	if totalMaxSize > 0 {
		totalUsagePercentage = (float64(totalUsedSize) / float64(totalMaxSize)) * 100
	}

	// Prepare view data
	viewData := struct {
		PoolStats            []models.StoragePoolStats
		HealthStatus         map[uint]bool
		Pools                []models.StoragePool
		Snapshots            map[uint]repository.HealthSnapshot
		TotalUsedSize        int64
		TotalMaxSize         int64
		TotalUsagePercentage float64
		TotalImageCount      int64
		TotalVariantCount    int64
		TotalPoolsCount      int
		HealthyPoolsCount    int
	}{
		PoolStats:            poolStats,
		HealthStatus:         healthStatus,
		Pools:                pools,
		Snapshots:            snapshots,
		TotalUsedSize:        totalUsedSize,
		TotalMaxSize:         totalMaxSize,
		TotalUsagePercentage: totalUsagePercentage,
		TotalImageCount:      totalImageCount,
		TotalVariantCount:    totalVariantCount,
		TotalPoolsCount:      len(pools),
		HealthyPoolsCount:    healthyPoolsCount,
	}

	// Render storage management using the standard layout
	storageManagement := admin_views.StorageManagement(viewData)
	home := views.HomeCtx(c, " | Speicherverwaltung", userCtx.IsLoggedIn, false, flash.Get(c), storageManagement, userCtx.IsAdmin, nil)

	handler := adaptor.HTTPHandler(templ.Handler(home))
	return handler(c)
}

// HandleAdminCreateStoragePool shows the create storage pool form using repository pattern
func (asc *AdminStorageController) HandleAdminCreateStoragePool(c *fiber.Ctx) error {
	userCtx := usercontext.GetUserContext(c)
	csrfToken := c.Locals("csrf").(string)

	poolForm := admin_views.StoragePoolForm(models.StoragePool{}, false, csrfToken)
	home := views.HomeCtx(c, " | Speicherpool erstellen", userCtx.IsLoggedIn, false, flash.Get(c), poolForm, userCtx.IsAdmin, nil)

	handler := adaptor.HTTPHandler(templ.Handler(home))
	return handler(c)
}

// HandleAdminCreateStoragePoolPost processes the create storage pool form using repository pattern
func (asc *AdminStorageController) HandleAdminCreateStoragePoolPost(c *fiber.Ctx) error {
	// Parse form data
	pool := models.StoragePool{
		Name:        strings.TrimSpace(c.FormValue("name")),
		BasePath:    strings.TrimSpace(c.FormValue("base_path")),
		StorageType: strings.TrimSpace(c.FormValue("storage_type")),
		StorageTier: strings.TrimSpace(c.FormValue("storage_tier")),
		Description: strings.TrimSpace(c.FormValue("description")),
		IsActive:    c.FormValue("is_active") == "on",
		IsDefault:   c.FormValue("is_default") == "on",
	}
	// Validate storage type early
	switch pool.StorageType {
	case models.StorageTypeLocal, models.StorageTypeNFS, models.StorageTypeS3:
		// ok
	case "":
		pool.StorageType = models.StorageTypeLocal
	default:
		fm := fiber.Map{
			"type":    "error",
			"message": "Ungültiger Speichertyp. Erlaubt: local, nfs, s3",
		}
		flash.WithError(c, fm)
		return c.Redirect("/admin/storage/create")
	}

	// Node/URL awareness
	pool.PublicBaseURL = strings.TrimSpace(c.FormValue("public_base_url"))
	pool.UploadAPIURL = strings.TrimSpace(c.FormValue("upload_api_url"))
	pool.NodeID = strings.TrimSpace(c.FormValue("node_id"))

	// Parse numeric values
	maxSizeStr := strings.TrimSpace(c.FormValue("max_size"))
	if maxSizeStr != "" {
		maxSizeGB, err := strconv.ParseInt(maxSizeStr, 10, 64)
		if err != nil {
			fm := fiber.Map{
				"type":    "error",
				"message": "Ungültige maximale Größe",
			}
			flash.WithError(c, fm)
			return c.Redirect("/admin/storage/create")
		}
		pool.MaxSize = maxSizeGB * 1024 * 1024 * 1024 // Convert GB to bytes
	}

	priorityStr := strings.TrimSpace(c.FormValue("priority"))
	if priorityStr != "" {
		priority, err := strconv.Atoi(priorityStr)
		if err != nil {
			fm := fiber.Map{
				"type":    "error",
				"message": "Ungültige Priorität",
			}
			flash.WithError(c, fm)
			return c.Redirect("/admin/storage/create")
		}
		pool.Priority = priority
	} else {
		pool.Priority = 100
	}

	// Parse S3-specific fields if storage type is S3
	if pool.StorageType == models.StorageTypeS3 {
		s3AccessKeyID := strings.TrimSpace(c.FormValue("s3_access_key_id"))
		if s3AccessKeyID != "" {
			pool.S3AccessKeyID = &s3AccessKeyID
		}

		s3SecretAccessKey := strings.TrimSpace(c.FormValue("s3_secret_access_key"))
		if s3SecretAccessKey != "" {
			pool.S3SecretAccessKey = &s3SecretAccessKey
		}

		s3Region := strings.TrimSpace(c.FormValue("s3_region"))
		if s3Region != "" {
			pool.S3Region = &s3Region
		}

		s3BucketName := strings.TrimSpace(c.FormValue("s3_bucket_name"))
		if s3BucketName != "" {
			pool.S3BucketName = &s3BucketName
		}

		s3EndpointURL := strings.TrimSpace(c.FormValue("s3_endpoint_url"))
		if s3EndpointURL != "" {
			pool.S3EndpointURL = &s3EndpointURL
		}

		s3PathPrefix := strings.TrimSpace(c.FormValue("s3_path_prefix"))
		if s3PathPrefix != "" {
			pool.S3PathPrefix = &s3PathPrefix
		}

		// Set base path for S3 pools
		if pool.S3BucketName != nil {
			pool.BasePath = fmt.Sprintf("s3://%s", *pool.S3BucketName)
		}
	}

	// Validate required fields
	if pool.Name == "" {
		fm := fiber.Map{
			"type":    "error",
			"message": "Name ist erforderlich",
		}
		flash.WithError(c, fm)
		return c.Redirect("/admin/storage/create")
	}

	if pool.BasePath == "" {
		fm := fiber.Map{
			"type":    "error",
			"message": "Basis-Pfad ist erforderlich",
		}
		flash.WithError(c, fm)
		return c.Redirect("/admin/storage/create")
	}

	if pool.MaxSize <= 0 {
		fm := fiber.Map{
			"type":    "error",
			"message": "Maximale Größe muss größer als 0 sein",
		}
		flash.WithError(c, fm)
		return c.Redirect("/admin/storage/create")
	}

	// Create storage pool using repository
	if err := asc.storagePoolRepo.Create(&pool); err != nil {
		if strings.Contains(err.Error(), "Duplicate entry") {
			fm := fiber.Map{
				"type":    "error",
				"message": "Ein Speicherpool mit diesem Namen existiert bereits",
			}
			flash.WithError(c, fm)
		} else {
			fm := fiber.Map{
				"type":    "error",
				"message": "Fehler beim Erstellen des Speicherpools: " + err.Error(),
			}
			flash.WithError(c, fm)
		}
		return c.Redirect("/admin/storage/create")
	}

	fm := fiber.Map{
		"type":    "success",
		"message": fmt.Sprintf("Speicherpool '%s' wurde erfolgreich erstellt", pool.Name),
	}
	flash.WithSuccess(c, fm)
	return c.Redirect("/admin/storage")
}

// HandleAdminEditStoragePool shows the edit storage pool form using repository pattern
func (asc *AdminStorageController) HandleAdminEditStoragePool(c *fiber.Ctx) error {
	userCtx := usercontext.GetUserContext(c)
	poolID, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		fm := fiber.Map{
			"type":    "error",
			"message": "Ungültige Pool-ID",
		}
		flash.WithError(c, fm)
		return c.Redirect("/admin/storage")
	}

	pool, err := asc.storagePoolRepo.GetByID(uint(poolID))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			fm := fiber.Map{
				"type":    "error",
				"message": "Speicherpool nicht gefunden",
			}
			flash.WithError(c, fm)
		} else {
			fm := fiber.Map{
				"type":    "error",
				"message": "Fehler beim Laden des Speicherpools: " + err.Error(),
			}
			flash.WithError(c, fm)
		}
		return c.Redirect("/admin/storage")
	}

	csrfToken := c.Locals("csrf").(string)

	poolForm := admin_views.StoragePoolForm(*pool, true, csrfToken)
	home := views.HomeCtx(c, " | Speicherpool bearbeiten", userCtx.IsLoggedIn, false, flash.Get(c), poolForm, userCtx.IsAdmin, nil)

	handler := adaptor.HTTPHandler(templ.Handler(home))
	return handler(c)
}

// HandleAdminEditStoragePoolPost processes the edit storage pool form using repository pattern
func (asc *AdminStorageController) HandleAdminEditStoragePoolPost(c *fiber.Ctx) error {
	poolID, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		fm := fiber.Map{
			"type":    "error",
			"message": "Ungültige Pool-ID",
		}
		flash.WithError(c, fm)
		return c.Redirect("/admin/storage")
	}

	// Find existing pool using repository
	pool, err := asc.storagePoolRepo.GetByID(uint(poolID))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			fm := fiber.Map{
				"type":    "error",
				"message": "Speicherpool nicht gefunden",
			}
			flash.WithError(c, fm)
		} else {
			fm := fiber.Map{
				"type":    "error",
				"message": "Fehler beim Laden des Speicherpools: " + err.Error(),
			}
			flash.WithError(c, fm)
		}
		return c.Redirect("/admin/storage")
	}

	// Update pool data
	pool.Name = strings.TrimSpace(c.FormValue("name"))
	pool.BasePath = strings.TrimSpace(c.FormValue("base_path"))
	// Storage type guard: only accept supported values; keep current if empty
	if st := strings.TrimSpace(c.FormValue("storage_type")); st != "" {
		switch st {
		case models.StorageTypeLocal, models.StorageTypeNFS, models.StorageTypeS3:
			pool.StorageType = st
		default:
			fm := fiber.Map{
				"type":    "error",
				"message": "Ungültiger Speichertyp. Erlaubt: local, nfs, s3",
			}
			flash.WithError(c, fm)
			return c.Redirect("/admin/storage/edit/" + c.Params("id"))
		}
	}
	pool.StorageTier = strings.TrimSpace(c.FormValue("storage_tier"))
	pool.Description = strings.TrimSpace(c.FormValue("description"))
	pool.IsActive = c.FormValue("is_active") == "on"
	pool.IsDefault = c.FormValue("is_default") == "on"

	// Node/URL awareness
	pool.PublicBaseURL = strings.TrimSpace(c.FormValue("public_base_url"))
	pool.UploadAPIURL = strings.TrimSpace(c.FormValue("upload_api_url"))
	pool.NodeID = strings.TrimSpace(c.FormValue("node_id"))

	// Parse numeric values
	maxSizeStr := strings.TrimSpace(c.FormValue("max_size"))
	if maxSizeStr != "" {
		maxSizeGB, err := strconv.ParseInt(maxSizeStr, 10, 64)
		if err != nil {
			fm := fiber.Map{
				"type":    "error",
				"message": "Ungültige maximale Größe",
			}
			flash.WithError(c, fm)
			return c.Redirect("/admin/storage/edit/" + c.Params("id"))
		}
		pool.MaxSize = maxSizeGB * 1024 * 1024 * 1024 // Convert GB to bytes
	}

	priorityStr := strings.TrimSpace(c.FormValue("priority"))
	if priorityStr != "" {
		priority, err := strconv.Atoi(priorityStr)
		if err != nil {
			fm := fiber.Map{
				"type":    "error",
				"message": "Ungültige Priorität",
			}
			flash.WithError(c, fm)
			return c.Redirect("/admin/storage/edit/" + c.Params("id"))
		}
		pool.Priority = priority
	}

	// Parse S3-specific fields if storage type is S3
	if pool.StorageType == models.StorageTypeS3 {
		s3AccessKeyID := strings.TrimSpace(c.FormValue("s3_access_key_id"))
		if s3AccessKeyID != "" {
			pool.S3AccessKeyID = &s3AccessKeyID
		}

		s3SecretAccessKey := strings.TrimSpace(c.FormValue("s3_secret_access_key"))
		if s3SecretAccessKey != "" {
			pool.S3SecretAccessKey = &s3SecretAccessKey
		}

		s3Region := strings.TrimSpace(c.FormValue("s3_region"))
		if s3Region != "" {
			pool.S3Region = &s3Region
		}

		s3BucketName := strings.TrimSpace(c.FormValue("s3_bucket_name"))
		if s3BucketName != "" {
			pool.S3BucketName = &s3BucketName
		}

		s3EndpointURL := strings.TrimSpace(c.FormValue("s3_endpoint_url"))
		if s3EndpointURL != "" {
			pool.S3EndpointURL = &s3EndpointURL
		}

		s3PathPrefix := strings.TrimSpace(c.FormValue("s3_path_prefix"))
		if s3PathPrefix != "" {
			pool.S3PathPrefix = &s3PathPrefix
		}

		// Update base path for S3 pools
		if pool.S3BucketName != nil {
			pool.BasePath = fmt.Sprintf("s3://%s", *pool.S3BucketName)
		}
	}

	// Validate required fields
	if pool.Name == "" {
		fm := fiber.Map{
			"type":    "error",
			"message": "Name ist erforderlich",
		}
		flash.WithError(c, fm)
		return c.Redirect("/admin/storage/edit/" + c.Params("id"))
	}

	if pool.BasePath == "" {
		fm := fiber.Map{
			"type":    "error",
			"message": "Basis-Pfad ist erforderlich",
		}
		flash.WithError(c, fm)
		return c.Redirect("/admin/storage/edit/" + c.Params("id"))
	}

	if pool.MaxSize <= 0 {
		fm := fiber.Map{
			"type":    "error",
			"message": "Maximale Größe muss größer als 0 sein",
		}
		flash.WithError(c, fm)
		return c.Redirect("/admin/storage/edit/" + c.Params("id"))
	}

	// Save changes using repository
	if err := asc.storagePoolRepo.Update(pool); err != nil {
		if strings.Contains(err.Error(), "Duplicate entry") {
			fm := fiber.Map{
				"type":    "error",
				"message": "Ein Speicherpool mit diesem Namen existiert bereits",
			}
			flash.WithError(c, fm)
		} else {
			fm := fiber.Map{
				"type":    "error",
				"message": "Fehler beim Aktualisieren des Speicherpools: " + err.Error(),
			}
			flash.WithError(c, fm)
		}
		return c.Redirect("/admin/storage/edit/" + c.Params("id"))
	}

	fm := fiber.Map{
		"type":    "success",
		"message": fmt.Sprintf("Speicherpool '%s' wurde erfolgreich aktualisiert", pool.Name),
	}
	flash.WithSuccess(c, fm)
	return c.Redirect("/admin/storage")
}

// HandleAdminDeleteStoragePool deletes a storage pool using repository pattern
func (asc *AdminStorageController) HandleAdminDeleteStoragePool(c *fiber.Ctx) error {
	poolID, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		fm := fiber.Map{
			"type":    "error",
			"message": "Ungültige Pool-ID",
		}
		flash.WithError(c, fm)
		return c.Redirect("/admin/storage")
	}

	// Find existing pool using repository
	pool, err := asc.storagePoolRepo.GetByID(uint(poolID))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			fm := fiber.Map{
				"type":    "error",
				"message": "Speicherpool nicht gefunden",
			}
			flash.WithError(c, fm)
		} else {
			fm := fiber.Map{
				"type":    "error",
				"message": "Fehler beim Laden des Speicherpools: " + err.Error(),
			}
			flash.WithError(c, fm)
		}
		return c.Redirect("/admin/storage")
	}

	// Check if pool is default
	if pool.IsDefault {
		fm := fiber.Map{
			"type":    "error",
			"message": "Der Standard-Speicherpool kann nicht gelöscht werden",
		}
		flash.WithError(c, fm)
		return c.Redirect("/admin/storage")
	}

	// Check if pool has files using repository
	imageCount, err := asc.storagePoolRepo.CountImagesInPool(uint(poolID))
	if err != nil {
		return asc.handleError(c, "Fehler beim Zählen der Bilder", err)
	}

	variantCount, err := asc.storagePoolRepo.CountVariantsInPool(uint(poolID))
	if err != nil {
		return asc.handleError(c, "Fehler beim Zählen der Varianten", err)
	}

	if imageCount > 0 || variantCount > 0 {
		fm := fiber.Map{
			"type":    "error",
			"message": fmt.Sprintf("Speicherpool kann nicht gelöscht werden: %d Bilder und %d Varianten sind noch vorhanden", imageCount, variantCount),
		}
		flash.WithError(c, fm)
		return c.Redirect("/admin/storage")
	}

	// Delete the pool using repository
	if err := asc.storagePoolRepo.Delete(uint(poolID)); err != nil {
		fm := fiber.Map{
			"type":    "error",
			"message": "Fehler beim Löschen des Speicherpools: " + err.Error(),
		}
		flash.WithError(c, fm)
		return c.Redirect("/admin/storage")
	}

	fm := fiber.Map{
		"type":    "success",
		"message": fmt.Sprintf("Speicherpool '%s' wurde erfolgreich gelöscht", pool.Name),
	}
	flash.WithSuccess(c, fm)
	return c.Redirect("/admin/storage")
}

// HandleAdminStoragePoolHealthCheck performs health check on a specific pool using repository pattern
func (asc *AdminStorageController) HandleAdminStoragePoolHealthCheck(c *fiber.Ctx) error {
	poolID, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "Ungültige Pool-ID",
		})
	}

	pool, err := asc.storagePoolRepo.GetByID(uint(poolID))
	if err != nil {
		return c.Status(404).JSON(fiber.Map{
			"success": false,
			"error":   "Speicherpool nicht gefunden",
		})
	}

	isHealthy, err := asc.storagePoolRepo.IsPoolHealthy(uint(poolID))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"error":   "Fehler beim Gesundheitscheck: " + err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success":   true,
		"pool_id":   pool.ID,
		"pool_name": pool.Name,
		"healthy":   isHealthy,
	})
}

// HandleAdminRecalculateStorageUsage recalculates storage usage for a pool using repository pattern
func (asc *AdminStorageController) HandleAdminRecalculateStorageUsage(c *fiber.Ctx) error {
	poolID, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "Ungültige Pool-ID",
		})
	}

	pool, err := asc.storagePoolRepo.GetByID(uint(poolID))
	if err != nil {
		return c.Status(404).JSON(fiber.Map{
			"success": false,
			"error":   "Speicherpool nicht gefunden",
		})
	}

	oldUsedSize := pool.UsedSize

	// Recalculate usage using repository
	newUsedSize, err := asc.storagePoolRepo.RecalculatePoolUsage(uint(poolID))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"error":   "Fehler beim Neuberechnen der Speichernutzung: " + err.Error(),
		})
	}

	// Get image and variant counts for detailed response
	imageCount, _ := asc.storagePoolRepo.CountImagesInPool(uint(poolID))
	variantCount, _ := asc.storagePoolRepo.CountVariantsInPool(uint(poolID))

	return c.JSON(fiber.Map{
		"success":       true,
		"pool_id":       pool.ID,
		"pool_name":     pool.Name,
		"old_used_size": oldUsedSize,
		"new_used_size": newUsedSize,
		"image_count":   imageCount,
		"variant_count": variantCount,
	})
}

// ============================================================================
// GLOBAL ADMIN STORAGE CONTROLLER INSTANCE - Singleton Pattern
// ============================================================================

var adminStorageController *AdminStorageController

// InitializeAdminStorageController initializes the global admin storage controller
func InitializeAdminStorageController() {
	storagePoolRepo := repository.GetGlobalFactory().GetStoragePoolRepository()
	adminStorageController = NewAdminStorageController(storagePoolRepo)
}

// GetAdminStorageController returns the global admin storage controller instance
func GetAdminStorageController() *AdminStorageController {
	if adminStorageController == nil {
		InitializeAdminStorageController()
	}
	return adminStorageController
}
