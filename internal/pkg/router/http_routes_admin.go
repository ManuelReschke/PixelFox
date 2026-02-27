package router

import (
	"github.com/ManuelReschke/PixelFox/app/controllers"
	"github.com/ManuelReschke/PixelFox/internal/pkg/middleware"
	"github.com/gofiber/fiber/v2"
)

func (h HttpRouter) registerAdminRoutes(app *fiber.App) {
	adminGroup := app.Group("/admin", middleware.RequireAdmin)
	adminGroup.Get("/", controllers.HandleAdminDashboard)
	adminGroup.Get("/users", controllers.HandleAdminUsers)
	adminGroup.Get("/users/edit/:id", controllers.HandleAdminUserEdit)
	adminGroup.Post("/users/update/:id", controllers.HandleAdminUserUpdate)
	adminGroup.Post("/users/update-plan/:id", controllers.HandleAdminUserUpdatePlan)
	adminGroup.Post("/users/delete/:id", controllers.HandleAdminUserDelete)
	adminGroup.Post("/users/resend-activation/:id", controllers.HandleAdminResendActivation)

	// Image management
	adminGroup.Get("/images", controllers.HandleAdminImages)
	adminGroup.Get("/images/edit/:uuid", controllers.HandleAdminImageEdit)
	adminGroup.Post("/images/update/:uuid", controllers.HandleAdminImageUpdate)
	adminGroup.Post("/images/delete/:uuid", controllers.HandleAdminImageDelete)

	// News management
	adminGroup.Get("/news", controllers.HandleAdminNews)
	adminGroup.Get("/news/create", controllers.HandleAdminNewsCreate)
	adminGroup.Post("/news/store", controllers.HandleAdminNewsStore)
	adminGroup.Get("/news/edit/:id", controllers.HandleAdminNewsEdit)
	adminGroup.Post("/news/update/:id", controllers.HandleAdminNewsUpdate)
	adminGroup.Post("/news/delete/:id", controllers.HandleAdminNewsDelete)

	// Search + queue monitor
	adminGroup.Get("/search", controllers.HandleAdminSearch)
	adminGroup.Get("/queues", controllers.HandleAdminQueues)
	adminGroup.Get("/queues/data", controllers.HandleAdminQueuesData)
	adminGroup.Delete("/queues/delete/:key", controllers.HandleAdminQueueDelete)
	adminGroup.Post("/queues/bulk-delete", controllers.HandleAdminQueueBulkDelete)

	// Storage management
	adminGroup.Get("/storage", controllers.HandleAdminStorageManagement)
	adminGroup.Get("/storage/health-check/:id", controllers.HandleAdminStoragePoolHealthCheck)
	adminGroup.Post("/storage/recalculate-usage/:id", controllers.HandleAdminRecalculateStorageUsage)
	adminGroup.Post("/storage/delete/:id", controllers.HandleAdminDeleteStoragePool)
	adminGroup.Post("/storage/tiering/sweep", controllers.HandleAdminTieringSweep)
}
