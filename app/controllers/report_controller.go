package controllers

import (
	"errors"
	"fmt"
	"time"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/internal/pkg/database"
	"github.com/ManuelReschke/PixelFox/internal/pkg/env"
	"github.com/ManuelReschke/PixelFox/internal/pkg/hcaptcha"
	"github.com/ManuelReschke/PixelFox/internal/pkg/usercontext"
	"github.com/ManuelReschke/PixelFox/views"
	admin_views "github.com/ManuelReschke/PixelFox/views/admin_views"
	report_views "github.com/ManuelReschke/PixelFox/views/report"
	"github.com/a-h/templ"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/sujit-baniya/flash"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const imageAlreadyReportedMessage = "Dieses Bild wurde bereits gemeldet und wird innerhalb von 24 Stunden überprüft."

var errImageAlreadyReported = errors.New("image already has an open report")

func redirectAlreadyReported(c *fiber.Ctx, uuid string) error {
	fm := fiber.Map{"type": "info", "message": imageAlreadyReportedMessage}
	return flash.WithInfo(c, fm).Redirect("/image/" + uuid)
}

func createImageReportIfNoneOpen(db *gorm.DB, imageID uint, report *models.ImageReport) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var lockImage models.Image
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id").
			Where("id = ?", imageID).
			Take(&lockImage).Error; err != nil {
			return err
		}

		hasOpenReport, err := models.HasOpenReportForImage(tx, imageID)
		if err != nil {
			return err
		}
		if hasOpenReport {
			return errImageAlreadyReported
		}

		return tx.Create(report).Error
	})
}

// GET /image/:uuid/report – show report form
func HandleImageReportForm(c *fiber.Ctx) error {
	fromProtected := false
	if v := c.Locals(usercontext.KeyFromProtected); v != nil {
		if b, ok := v.(bool); ok {
			fromProtected = b
		}
	}
	csrfToken := c.Locals("csrf").(string)
	uuid := c.Params("uuid")

	// load image for context
	db := database.GetDB()
	image, err := models.FindImageByUUID(db, uuid)
	if err != nil {
		return c.Redirect("/", fiber.StatusSeeOther)
	}
	if hasOpenReport, err := models.HasOpenReportForImage(db, image.ID); err != nil {
		fm := fiber.Map{"type": "error", "message": "Meldungsstatus konnte nicht geprüft werden."}
		return flash.WithError(c, fm).Redirect("/image/" + uuid)
	} else if hasOpenReport {
		return redirectAlreadyReported(c, uuid)
	}

	displayName := image.FileName
	if image.Title != "" {
		displayName = image.Title
	}
	shareURL := fmt.Sprintf("%s/i/%s", c.BaseURL(), image.ShareLink)
	hcaptchaSitekey := env.GetEnv("HCAPTCHA_SITEKEY", "")

	page := report_views.ReportIndex(fromProtected, csrfToken, uuid, displayName, shareURL, hcaptchaSitekey)
	return page.Render(c.Context(), c.Response().BodyWriter())
}

// POST /image/:uuid/report – submit report
func HandleImageReportSubmit(c *fiber.Ctx) error {
	uuid := c.Params("uuid")
	if uuid == "" {
		return c.Redirect("/", fiber.StatusSeeOther)
	}

	db := database.GetDB()
	image, err := models.FindImageByUUID(db, uuid)
	if err != nil {
		fm := fiber.Map{"type": "error", "message": "Bild wurde nicht gefunden."}
		return flash.WithError(c, fm).Redirect("/")
	}
	if hasOpenReport, err := models.HasOpenReportForImage(db, image.ID); err != nil {
		fm := fiber.Map{"type": "error", "message": "Meldungsstatus konnte nicht geprüft werden."}
		return flash.WithError(c, fm).Redirect("/image/" + uuid)
	} else if hasOpenReport {
		return redirectAlreadyReported(c, uuid)
	}

	reason := c.FormValue("reason")
	details := c.FormValue("details")
	if reason == "" {
		fm := fiber.Map{"type": "error", "message": "Bitte einen Grund auswählen."}
		return flash.WithError(c, fm).Redirect("/image/" + uuid + "/report")
	}
	if reason == "other" && len(details) < 5 {
		fm := fiber.Map{"type": "error", "message": "Bitte eine kurze Begründung angeben."}
		return flash.WithError(c, fm).Redirect("/image/" + uuid + "/report")
	}

	// Guests must solve hCaptcha (if configured)
	uctx := usercontext.GetUserContext(c)
	if !uctx.IsLoggedIn {
		if env.GetEnv("HCAPTCHA_SITEKEY", "") != "" && env.GetEnv("HCAPTCHA_SECRET", "") != "" {
			hcaptchaToken := c.FormValue("h-captcha-response")
			valid, err := hcaptcha.Verify(hcaptchaToken)
			if err != nil || !valid {
				errorMsg := "Captcha validation failed. Please try again."
				if err != nil && env.IsDev() {
					errorMsg = fmt.Sprintf("Captcha validation failed: %v", err)
				}
				fm := fiber.Map{"type": "error", "message": errorMsg}
				return flash.WithError(c, fm).Redirect("/image/" + uuid + "/report")
			}
		}
	}

	// reporter info
	var reporterID *uint
	if uctx.IsLoggedIn && uctx.UserID > 0 {
		rid := uctx.UserID
		reporterID = &rid
	}
	ipv4, ipv6 := GetClientIP(c)

	report := models.ImageReport{
		ImageID:      image.ID,
		ReporterID:   reporterID,
		Reason:       reason,
		Details:      details,
		Status:       models.ReportStatusOpen,
		ReporterIPv4: ipv4,
		ReporterIPv6: ipv6,
	}

	if err := createImageReportIfNoneOpen(db, image.ID, &report); err != nil {
		if errors.Is(err, errImageAlreadyReported) {
			return redirectAlreadyReported(c, uuid)
		}
		fm := fiber.Map{"type": "error", "message": "Meldung konnte nicht gespeichert werden."}
		return flash.WithError(c, fm).Redirect("/image/" + uuid)
	}

	fm := fiber.Map{"type": "success", "message": "Danke! Deine Meldung wurde übermittelt."}
	return flash.WithSuccess(c, fm).Redirect("/image/" + uuid)
}

// ADMIN – list reports
func HandleAdminReports(c *fiber.Ctx) error {
	db := database.GetDB()
	var reports []models.ImageReport
	// show open first
	if err := db.Preload("Image").Preload("Reporter").Where("status = ?", models.ReportStatusOpen).Order("created_at DESC").Find(&reports).Error; err != nil {
		reports = []models.ImageReport{}
	}

	// Also get recent resolved/dismissed (optional minimal)
	var recentClosed []models.ImageReport
	_ = db.Preload("Image").Preload("Reporter").Where("status != ?", models.ReportStatusOpen).Order("updated_at DESC").Limit(20).Find(&recentClosed).Error

	userCtx := usercontext.GetUserContext(c)
	cmp := admin_views.AdminReportsPage(reports, recentClosed)
	home := views.HomeCtx(c, " | Meldungen", userCtx.IsLoggedIn, false, flash.Get(c), cmp, userCtx.IsAdmin, nil)
	handler := adaptor.HTTPHandler(templ.Handler(home))
	return handler(c)
}

// ADMIN – show single report
func HandleAdminReportShow(c *fiber.Ctx) error {
	db := database.GetDB()
	id := c.Params("id")
	var report models.ImageReport
	if err := db.Preload("Image").Preload("Reporter").Preload("ResolvedBy").First(&report, id).Error; err != nil {
		return c.Redirect("/admin/reports", fiber.StatusSeeOther)
	}
	csrfToken := c.Locals("csrf").(string)
	userCtx := usercontext.GetUserContext(c)
	cmp := admin_views.AdminReportShow(&report, csrfToken)
	title := fmt.Sprintf(" | Meldung #%d", report.ID)
	home := views.HomeCtx(c, title, userCtx.IsLoggedIn, false, flash.Get(c), cmp, userCtx.IsAdmin, nil)
	handler := adaptor.HTTPHandler(templ.Handler(home))
	return handler(c)
}

// ADMIN – mark resolved
func HandleAdminReportResolve(c *fiber.Ctx) error {
	db := database.GetDB()
	id := c.Params("id")
	var report models.ImageReport
	if err := db.First(&report, id).Error; err != nil {
		return c.Redirect("/admin/reports", fiber.StatusSeeOther)
	}
	uctx := usercontext.GetUserContext(c)
	if uctx.UserID == 0 {
		return c.Redirect("/admin/reports", fiber.StatusSeeOther)
	}
	report.Status = models.ReportStatusResolved
	report.ResolvedByID = &uctx.UserID
	t := time.Now()
	report.ResolvedAt = &t
	_ = db.Save(&report).Error
	return c.Redirect("/admin/reports/"+id, fiber.StatusSeeOther)
}

// ADMIN – dismiss
func HandleAdminReportDismiss(c *fiber.Ctx) error {
	db := database.GetDB()
	id := c.Params("id")
	var report models.ImageReport
	if err := db.First(&report, id).Error; err != nil {
		return c.Redirect("/admin/reports", fiber.StatusSeeOther)
	}
	uctx := usercontext.GetUserContext(c)
	if uctx.UserID == 0 {
		return c.Redirect("/admin/reports", fiber.StatusSeeOther)
	}
	report.Status = models.ReportStatusDismissed
	report.ResolvedByID = &uctx.UserID
	t := time.Now()
	report.ResolvedAt = &t
	_ = db.Save(&report).Error
	return c.Redirect("/admin/reports/"+id, fiber.StatusSeeOther)
}
