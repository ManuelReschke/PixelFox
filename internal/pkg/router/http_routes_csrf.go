package router

import (
	"strings"
	"time"

	"github.com/ManuelReschke/PixelFox/app/controllers"
	"github.com/ManuelReschke/PixelFox/internal/pkg/env"
	"github.com/ManuelReschke/PixelFox/internal/pkg/middleware"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/csrf"
)

func (h HttpRouter) registerCSRFProtectedRoutes(app *fiber.App) {
	csrfConf := csrf.Config{
		KeyLookup:      "form:_csrf",
		ContextKey:     "csrf",
		CookieName:     "csrf_",
		CookieSameSite: "Lax",
		Expiration:     1 * time.Hour,
		CookieSecure:   !env.IsDev(),
		Next: func(c *fiber.Ctx) bool {
			return strings.HasPrefix(c.Path(), "/api/")
		},
	}

	group := app.Group("", cors.New(), csrf.New(csrfConf))
	group.Get("/", loggedInMiddleware, controllers.HandleStart)
	group.Post("/upload", middleware.RequireAuth, controllers.HandleUpload)
	group.Get("/upload/batch/:id", middleware.RequireAuth, controllers.HandleUploadBatchView)
	group.Post("/upload/batch/:id/album", middleware.RequireAuth, controllers.HandleUploadBatchSaveAsAlbum)
	group.Get("/login", loggedInMiddleware, controllers.HandleAuthLogin)
	group.Post("/login", loggedInMiddleware, controllers.HandleAuthLogin)
	group.Get("/register", loggedInMiddleware, controllers.HandleAuthRegister)
	group.Post("/register", loggedInMiddleware, controllers.HandleAuthRegister)
	group.Get("/activate", loggedInMiddleware, controllers.HandleAuthActivate)
	group.Post("/activate", loggedInMiddleware, controllers.HandleAuthActivate)
	group.Get("/user/profile", middleware.RequireAuth, controllers.HandleUserProfile)
	group.Get("/user/profile/edit", middleware.RequireAuth, controllers.HandleUserProfileEdit)
	group.Post("/user/profile/edit", middleware.RequireAuth, controllers.HandleUserProfileEditPost)
	group.Get("/user/profile/verify-email-change", controllers.HandleEmailChangeVerification)
	group.Get("/user/profile/edit/cancel-email-change", middleware.RequireAuth, controllers.HandleCancelEmailChange)
	group.Get("/user/profile/edit/resend-email-change", middleware.RequireAuth, controllers.HandleResendEmailChange)
	group.Get("/user/settings", middleware.RequireAuth, controllers.HandleUserSettings)
	group.Post("/user/settings", middleware.RequireAuth, controllers.HandleUserSettingsPost)
	group.Post("/user/settings/api-key", middleware.RequireAuth, controllers.HandleUserAPIKeyGenerate)
	group.Post("/user/settings/api-key/revoke", middleware.RequireAuth, controllers.HandleUserAPIKeyRevoke)
	group.Get("/user/images", middleware.RequireAuth, controllers.HandleUserImages)
	group.Get("/user/images/load", middleware.RequireAuth, controllers.HandleLoadMoreImages)
	group.Get("/user/images/edit/:uuid", middleware.RequireAuth, controllers.HandleUserImageEdit)
	group.Post("/user/images/update/:uuid", middleware.RequireAuth, controllers.HandleUserImageUpdate)
	group.Post("/user/images/delete/:uuid", middleware.RequireAuth, controllers.HandleUserImageDelete)

	// User albums
	group.Get("/user/albums", middleware.RequireAuth, controllers.HandleUserAlbums)
	group.Get("/user/albums/create", middleware.RequireAuth, controllers.HandleUserAlbumCreate)
	group.Post("/user/albums/create", middleware.RequireAuth, controllers.HandleUserAlbumCreate)
	group.Get("/user/albums/:id", middleware.RequireAuth, controllers.HandleUserAlbumView)
	group.Get("/user/albums/edit/:id", middleware.RequireAuth, controllers.HandleUserAlbumEdit)
	group.Post("/user/albums/edit/:id", middleware.RequireAuth, controllers.HandleUserAlbumEdit)
	group.Post("/user/albums/delete/:id", middleware.RequireAuth, controllers.HandleUserAlbumDelete)
	group.Post("/user/albums/:id/add-image", middleware.RequireAuth, controllers.HandleUserAlbumAddImage)
	group.Post("/user/albums/:id/set-cover", middleware.RequireAuth, controllers.HandleUserAlbumSetCover)
	group.Post("/user/albums/:id/remove-image/:image_id", middleware.RequireAuth, controllers.HandleUserAlbumRemoveImage)

	// Image reports (guest allowed)
	group.Get("/image/:uuid/report", loggedInMiddleware, controllers.HandleImageReportForm)
	group.Post("/image/:uuid/report", loggedInMiddleware, controllers.HandleImageReportSubmit)

	// Admin pages/settings/storage/reports
	group.Get("/admin/pages", middleware.RequireAdmin, controllers.HandleAdminPages)
	group.Get("/admin/pages/create", middleware.RequireAdmin, controllers.HandleAdminPageCreate)
	group.Post("/admin/pages/store", middleware.RequireAdmin, controllers.HandleAdminPageStore)
	group.Get("/admin/pages/edit/:id", middleware.RequireAdmin, controllers.HandleAdminPageEdit)
	group.Post("/admin/pages/update/:id", middleware.RequireAdmin, controllers.HandleAdminPageUpdate)
	group.Post("/admin/pages/delete/:id", middleware.RequireAdmin, controllers.HandleAdminPageDelete)
	group.Get("/admin/settings", middleware.RequireAdmin, controllers.HandleAdminSettings)
	group.Post("/admin/settings", middleware.RequireAdmin, controllers.HandleAdminSettingsUpdate)
	group.Get("/admin/storage/create", middleware.RequireAdmin, controllers.HandleAdminCreateStoragePool)
	group.Post("/admin/storage/create", middleware.RequireAdmin, controllers.HandleAdminCreateStoragePoolPost)
	group.Get("/admin/storage/edit/:id", middleware.RequireAdmin, controllers.HandleAdminEditStoragePool)
	group.Post("/admin/storage/edit/:id", middleware.RequireAdmin, controllers.HandleAdminEditStoragePoolPost)
	group.Get("/admin/storage/move/:id", middleware.RequireAdmin, controllers.HandleAdminMoveStoragePool)
	group.Post("/admin/storage/move/:id", middleware.RequireAdmin, controllers.HandleAdminMoveStoragePoolPost)
	group.Get("/admin/reports", middleware.RequireAdmin, controllers.HandleAdminReports)
	group.Get("/admin/reports/:id", middleware.RequireAdmin, controllers.HandleAdminReportShow)
	group.Post("/admin/reports/:id/resolve", middleware.RequireAdmin, controllers.HandleAdminReportResolve)
	group.Post("/admin/reports/:id/dismiss", middleware.RequireAdmin, controllers.HandleAdminReportDismiss)
}
