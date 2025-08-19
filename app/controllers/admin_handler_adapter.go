package controllers

import (
	"github.com/ManuelReschke/PixelFox/app/repository"
	"github.com/gofiber/fiber/v2"
)

// Global admin controller instance
var adminController *AdminController

// InitializeAdminController initializes the global admin controller with repositories
func InitializeAdminController() {
	repos := repository.GetGlobalRepositories()
	adminController = NewAdminController(repos)
}

// GetAdminController returns the global admin controller instance
func GetAdminController() *AdminController {
	if adminController == nil {
		InitializeAdminController()
	}
	return adminController
}

// Adapter functions to maintain compatibility with existing router

// HandleAdminDashboard - Adapter for admin dashboard
func HandleAdminDashboard(c *fiber.Ctx) error {
	return GetAdminController().HandleDashboard(c)
}

// HandleAdminUsers - Adapter for user management
func HandleAdminUsers(c *fiber.Ctx) error {
	return GetAdminController().HandleUsers(c)
}

// HandleAdminUserEdit - Adapter for user edit
func HandleAdminUserEdit(c *fiber.Ctx) error {
	return GetAdminController().HandleUserEdit(c)
}

// HandleAdminUserUpdate - Adapter for user update
func HandleAdminUserUpdate(c *fiber.Ctx) error {
	return GetAdminController().HandleUserUpdate(c)
}

// HandleAdminUserDelete - Adapter for user delete
func HandleAdminUserDelete(c *fiber.Ctx) error {
	return GetAdminController().HandleUserDelete(c)
}

// HandleAdminImages - Adapter for image management using dedicated AdminImagesController
func HandleAdminImages(c *fiber.Ctx) error {
	return GetAdminImagesController().HandleAdminImages(c)
}

// HandleAdminSearch - Adapter for search functionality
func HandleAdminSearch(c *fiber.Ctx) error {
	return GetAdminController().HandleSearch(c)
}

// HandleAdminSettings - Adapter for settings page
func HandleAdminSettings(c *fiber.Ctx) error {
	return GetAdminController().HandleSettings(c)
}

// HandleAdminSettingsUpdate - Adapter for settings update
func HandleAdminSettingsUpdate(c *fiber.Ctx) error {
	return GetAdminController().HandleSettingsUpdate(c)
}

// Repository Pattern Functions - These use the AdminController with repositories

// HandleAdminResendActivation - Adapter for resend activation
func HandleAdminResendActivation(c *fiber.Ctx) error {
	return GetAdminController().HandleResendActivation(c)
}

// HandleAdminImageEdit - Adapter for image edit using dedicated AdminImagesController
func HandleAdminImageEdit(c *fiber.Ctx) error {
	return GetAdminImagesController().HandleAdminImageEdit(c)
}

// HandleAdminImageUpdate - Adapter for image update using dedicated AdminImagesController
func HandleAdminImageUpdate(c *fiber.Ctx) error {
	return GetAdminImagesController().HandleAdminImageUpdate(c)
}

// HandleAdminImageDelete - Adapter for image delete using dedicated AdminImagesController
func HandleAdminImageDelete(c *fiber.Ctx) error {
	return GetAdminImagesController().HandleAdminImageDelete(c)
}

// News Management - Repository Pattern Functions using dedicated AdminNewsController

// HandleAdminNews - Adapter for news management
func HandleAdminNews(c *fiber.Ctx) error {
	return GetAdminNewsController().HandleAdminNews(c)
}

// HandleAdminNewsCreate - Adapter for news create form
func HandleAdminNewsCreate(c *fiber.Ctx) error {
	return GetAdminNewsController().HandleAdminNewsCreate(c)
}

// HandleAdminNewsStore - Adapter for news creation
func HandleAdminNewsStore(c *fiber.Ctx) error {
	return GetAdminNewsController().HandleAdminNewsStore(c)
}

// HandleAdminNewsEdit - Adapter for news edit form
func HandleAdminNewsEdit(c *fiber.Ctx) error {
	return GetAdminNewsController().HandleAdminNewsEdit(c)
}

// HandleAdminNewsUpdate - Adapter for news update
func HandleAdminNewsUpdate(c *fiber.Ctx) error {
	return GetAdminNewsController().HandleAdminNewsUpdate(c)
}

// HandleAdminNewsDelete - Adapter for news deletion
func HandleAdminNewsDelete(c *fiber.Ctx) error {
	return GetAdminNewsController().HandleAdminNewsDelete(c)
}

// Queue Management - Repository Pattern Functions using dedicated AdminQueueController

// HandleAdminQueues - Adapter for queue management
func HandleAdminQueues(c *fiber.Ctx) error {
	return GetAdminQueueController().HandleAdminQueues(c)
}

// HandleAdminQueuesData - Adapter for queue data API
func HandleAdminQueuesData(c *fiber.Ctx) error {
	return GetAdminQueueController().HandleAdminQueuesData(c)
}

// HandleAdminQueueDelete - Adapter for queue entry deletion
func HandleAdminQueueDelete(c *fiber.Ctx) error {
	return GetAdminQueueController().HandleAdminQueueDelete(c)
}

// Storage Management - Repository Pattern Functions using dedicated AdminStorageController

// HandleAdminStorageManagement - Adapter for storage management dashboard
func HandleAdminStorageManagement(c *fiber.Ctx) error {
	return GetAdminStorageController().HandleAdminStorageManagement(c)
}

// HandleAdminCreateStoragePool - Adapter for create storage pool form
func HandleAdminCreateStoragePool(c *fiber.Ctx) error {
	return GetAdminStorageController().HandleAdminCreateStoragePool(c)
}

// HandleAdminCreateStoragePoolPost - Adapter for create storage pool form submission
func HandleAdminCreateStoragePoolPost(c *fiber.Ctx) error {
	return GetAdminStorageController().HandleAdminCreateStoragePoolPost(c)
}

// HandleAdminEditStoragePool - Adapter for edit storage pool form
func HandleAdminEditStoragePool(c *fiber.Ctx) error {
	return GetAdminStorageController().HandleAdminEditStoragePool(c)
}

// HandleAdminEditStoragePoolPost - Adapter for edit storage pool form submission
func HandleAdminEditStoragePoolPost(c *fiber.Ctx) error {
	return GetAdminStorageController().HandleAdminEditStoragePoolPost(c)
}

// HandleAdminDeleteStoragePool - Adapter for storage pool deletion
func HandleAdminDeleteStoragePool(c *fiber.Ctx) error {
	return GetAdminStorageController().HandleAdminDeleteStoragePool(c)
}

// HandleAdminStoragePoolHealthCheck - Adapter for storage pool health check
func HandleAdminStoragePoolHealthCheck(c *fiber.Ctx) error {
	return GetAdminStorageController().HandleAdminStoragePoolHealthCheck(c)
}

// HandleAdminRecalculateStorageUsage - Adapter for storage usage recalculation
func HandleAdminRecalculateStorageUsage(c *fiber.Ctx) error {
	return GetAdminStorageController().HandleAdminRecalculateStorageUsage(c)
}

// Page Management - Repository Pattern Functions using dedicated AdminPageController

// HandleAdminPages - Adapter for page management
func HandleAdminPages(c *fiber.Ctx) error {
	return GetAdminPageController().HandleAdminPages(c)
}

// HandleAdminPageCreate - Adapter for page create form
func HandleAdminPageCreate(c *fiber.Ctx) error {
	return GetAdminPageController().HandleAdminPageCreate(c)
}

// HandleAdminPageStore - Adapter for page creation
func HandleAdminPageStore(c *fiber.Ctx) error {
	return GetAdminPageController().HandleAdminPageStore(c)
}

// HandleAdminPageEdit - Adapter for page edit form
func HandleAdminPageEdit(c *fiber.Ctx) error {
	return GetAdminPageController().HandleAdminPageEdit(c)
}

// HandleAdminPageUpdate - Adapter for page update
func HandleAdminPageUpdate(c *fiber.Ctx) error {
	return GetAdminPageController().HandleAdminPageUpdate(c)
}

// HandleAdminPageDelete - Adapter for page deletion
func HandleAdminPageDelete(c *fiber.Ctx) error {
	return GetAdminPageController().HandleAdminPageDelete(c)
}
