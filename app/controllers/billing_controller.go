package controllers

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/internal/pkg/billing"
	"github.com/ManuelReschke/PixelFox/internal/pkg/database"
	"github.com/ManuelReschke/PixelFox/internal/pkg/env"
	"github.com/ManuelReschke/PixelFox/internal/pkg/session"
	"github.com/ManuelReschke/PixelFox/internal/pkg/usercontext"
	"github.com/gofiber/fiber/v2"
	"github.com/sujit-baniya/flash"
	"gorm.io/gorm"
)

const patreonOAuthStateSessionKey = "patreon_oauth_state"

func HandlePatreonConnect(c *fiber.Ctx) error {
	userCtx := usercontext.GetUserContext(c)
	if !userCtx.IsLoggedIn {
		return c.Redirect("/login", fiber.StatusSeeOther)
	}

	state, err := generateOAuthState(24)
	if err != nil {
		return flash.WithError(c, fiber.Map{"type": "error", "message": "OAuth-Status konnte nicht erzeugt werden"}).Redirect("/user/settings/membership")
	}

	sess, err := session.GetSessionStore().Get(c)
	if err != nil {
		return flash.WithError(c, fiber.Map{"type": "error", "message": "Session konnte nicht geladen werden"}).Redirect("/user/settings/membership")
	}
	sess.Set(patreonOAuthStateSessionKey, state)
	if err := sess.Save(); err != nil {
		return flash.WithError(c, fiber.Map{"type": "error", "message": "Session konnte nicht gespeichert werden"}).Redirect("/user/settings/membership")
	}

	client := billing.NewPatreonClientFromEnv()
	url, err := client.AuthorizeURLWithState(state)
	if err != nil {
		return flash.WithError(c, fiber.Map{"type": "error", "message": "Patreon OAuth ist nicht korrekt konfiguriert"}).Redirect("/user/settings/membership")
	}

	return c.Redirect(url, fiber.StatusSeeOther)
}

func HandlePatreonCallback(c *fiber.Ctx) error {
	userCtx := usercontext.GetUserContext(c)
	if !userCtx.IsLoggedIn {
		return c.Redirect("/login", fiber.StatusSeeOther)
	}

	if oauthErr := strings.TrimSpace(c.Query("error")); oauthErr != "" {
		msg := c.Query("error_description", oauthErr)
		return flash.WithError(c, fiber.Map{"type": "error", "message": "Patreon OAuth fehlgeschlagen: " + msg}).Redirect("/user/settings/membership")
	}

	sess, err := session.GetSessionStore().Get(c)
	if err != nil {
		return flash.WithError(c, fiber.Map{"type": "error", "message": "Session konnte nicht geladen werden"}).Redirect("/user/settings/membership")
	}
	expectedState, _ := sess.Get(patreonOAuthStateSessionKey).(string)
	gotState := strings.TrimSpace(c.Query("state"))
	sess.Delete(patreonOAuthStateSessionKey)
	_ = sess.Save()
	if expectedState == "" || gotState == "" || expectedState != gotState {
		return flash.WithError(c, fiber.Map{"type": "error", "message": "Ungueltiger OAuth-Status (state mismatch)"}).Redirect("/user/settings/membership")
	}

	code := strings.TrimSpace(c.Query("code"))
	if code == "" {
		return flash.WithError(c, fiber.Map{"type": "error", "message": "OAuth-Code fehlt"}).Redirect("/user/settings/membership")
	}

	client := billing.NewPatreonClientFromEnv()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	token, err := client.ExchangeCode(ctx, code)
	if err != nil {
		return flash.WithError(c, fiber.Map{"type": "error", "message": "Token-Austausch mit Patreon fehlgeschlagen"}).Redirect("/user/settings/membership")
	}

	identity, err := client.GetIdentity(ctx, token.AccessToken)
	if err != nil {
		return flash.WithError(c, fiber.Map{"type": "error", "message": "Patreon-Identitaet konnte nicht geladen werden"}).Redirect("/user/settings/membership")
	}

	svc := billing.NewServiceFromDB(database.GetDB())
	var tokenExpiresAt *time.Time
	if token.ExpiresIn > 0 {
		t := time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
		tokenExpiresAt = &t
	}

	// Tokens are intentionally not stored yet until at-rest encryption is wired.
	if _, err := svc.UpsertBillingAccount(
		ctx,
		userCtx.UserID,
		models.BillingProviderPatreon,
		identity.PatreonUserID,
		identity.Email,
		"",
		"",
		tokenExpiresAt,
	); err != nil {
		return flash.WithError(c, fiber.Map{"type": "error", "message": "Patreon-Konto konnte nicht verknuepft werden"}).Redirect("/user/settings/membership")
	}

	tierRef, _, _ := svc.ResolveBestMappedTier(ctx, models.BillingProviderPatreon, identity.TierIDs, models.BillingIntervalUnknown)
	if tierRef == "" {
		tierRef = "none"
	}
	subscriptionID := strings.TrimSpace(identity.MembershipID)
	if subscriptionID == "" {
		subscriptionID = "member:" + identity.PatreonUserID
	}

	_, effectivePlan, err := svc.SyncSubscription(ctx, billing.NormalizedSubscription{
		UserID:                 userCtx.UserID,
		Provider:               models.BillingProviderPatreon,
		ProviderSubscriptionID: subscriptionID,
		ProviderPlanRef:        tierRef,
		BillingInterval:        models.BillingIntervalUnknown,
		Status:                 billing.PatreonMembershipToBillingStatus(identity.PatronStatus, identity.IsFollower),
		RawPayloadJSON:         "",
	})
	if err != nil {
		return flash.WithError(c, fiber.Map{"type": "error", "message": "Mitgliedschaft konnte nicht synchronisiert werden"}).Redirect("/user/settings/membership")
	}

	_ = session.SetSessionValue(c, "user_plan", effectivePlan)
	msg := fmt.Sprintf("Patreon erfolgreich verbunden. Aktiver Plan: %s", effectivePlan)
	return flash.WithSuccess(c, fiber.Map{"type": "success", "message": msg}).Redirect("/user/settings/membership")
}

func HandlePatreonWebhook(c *fiber.Ctx) error {
	rawBody := append([]byte(nil), c.BodyRaw()...)
	eventType := strings.TrimSpace(c.Get("X-Patreon-Event"))
	eventID := firstHeaderValue(c, "X-Patreon-Delivery", "X-Patreon-Event-ID", "X-Patreon-Webhook-ID")
	signature := strings.TrimSpace(c.Get("X-Patreon-Signature"))
	secret := env.GetEnv("PATREON_WEBHOOK_SECRET", "")

	svc := billing.NewServiceFromDB(database.GetDB())
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	signatureValid := billing.VerifyPatreonWebhookSignature(rawBody, signature, secret)
	created, stored, err := svc.RecordWebhookEvent(ctx, billing.WebhookEventInput{
		Provider:        models.BillingProviderPatreon,
		ProviderEventID: eventID,
		EventType:       eventType,
		PayloadJSON:     string(rawBody),
		SignatureValid:  signatureValid,
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "webhook_persist_failed"})
	}
	if !created {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"ok": true, "duplicate": true})
	}
	if !signatureValid {
		_ = svc.MarkWebhookProcessed(ctx, stored.ID, errors.New("invalid webhook signature"))
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid_signature"})
	}
	if !isPatreonMemberEvent(eventType) {
		_ = svc.MarkWebhookProcessed(ctx, stored.ID, nil)
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"ok": true, "ignored": true})
	}

	memberEvent, err := billing.ParsePatreonWebhookMemberEvent(rawBody)
	if err != nil {
		_ = svc.MarkWebhookProcessed(ctx, stored.ID, err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid_payload"})
	}

	account, err := svc.GetBillingAccountByProviderAccountID(ctx, models.BillingProviderPatreon, memberEvent.PatreonUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			_ = svc.MarkWebhookProcessed(ctx, stored.ID, errors.New("no linked local account for patreon user"))
			return c.Status(fiber.StatusOK).JSON(fiber.Map{"ok": true, "ignored": true})
		}
		_ = svc.MarkWebhookProcessed(ctx, stored.ID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "account_lookup_failed"})
	}

	tierRef, _, _ := svc.ResolveBestMappedTier(ctx, models.BillingProviderPatreon, memberEvent.TierIDs, models.BillingIntervalUnknown)
	if tierRef == "" {
		tierRef = "none"
	}
	subscriptionID := memberEvent.MemberID
	if subscriptionID == "" {
		subscriptionID = "member:" + memberEvent.PatreonUserID
	}

	_, _, syncErr := svc.SyncSubscription(ctx, billing.NormalizedSubscription{
		UserID:                 account.UserID,
		Provider:               models.BillingProviderPatreon,
		ProviderSubscriptionID: subscriptionID,
		ProviderPlanRef:        tierRef,
		BillingInterval:        models.BillingIntervalUnknown,
		Status:                 billing.PatreonMembershipToBillingStatus(memberEvent.PatronStatus, memberEvent.IsFollower),
		RawPayloadJSON:         string(rawBody),
	})
	_ = svc.MarkWebhookProcessed(ctx, stored.ID, syncErr)
	if syncErr != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "subscription_sync_failed"})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"ok": true})
}

func HandleUserBillingResync(c *fiber.Ctx) error {
	userCtx := usercontext.GetUserContext(c)
	if !userCtx.IsLoggedIn {
		return c.Redirect("/login", fiber.StatusSeeOther)
	}

	svc := billing.NewServiceFromDB(database.GetDB())
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	effectivePlan, err := svc.ReconcileUserPlan(ctx, userCtx.UserID)
	if err != nil {
		return flash.WithError(c, fiber.Map{"type": "error", "message": "Plan-Re-Sync fehlgeschlagen"}).Redirect("/user/settings/membership")
	}

	_ = session.SetSessionValue(c, "user_plan", effectivePlan)
	msg := fmt.Sprintf("Plan neu berechnet. Aktiver Plan: %s", effectivePlan)
	return flash.WithSuccess(c, fiber.Map{"type": "success", "message": msg}).Redirect("/user/settings/membership")
}

func isPatreonMemberEvent(eventType string) bool {
	switch strings.ToLower(strings.TrimSpace(eventType)) {
	case "members:create", "members:update", "members:delete":
		return true
	default:
		return false
	}
}

func generateOAuthState(size int) (string, error) {
	if size < 16 {
		size = 16
	}
	b := make([]byte, size)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func firstHeaderValue(c *fiber.Ctx, keys ...string) string {
	for _, k := range keys {
		v := strings.TrimSpace(c.Get(k))
		if v != "" {
			return v
		}
	}
	return ""
}
