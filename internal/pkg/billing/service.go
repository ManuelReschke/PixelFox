package billing

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/internal/pkg/entitlements"
	"gorm.io/gorm"
)

// Service provides provider-neutral billing synchronization and reconciliation.
type Service struct {
	repo Repository
}

// NewService creates a billing service from an injected repository.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// NewServiceFromDB creates a billing service from a GORM DB handle.
func NewServiceFromDB(db *gorm.DB) *Service {
	return NewService(NewRepository(db))
}

// UpsertBillingAccount creates or updates a linked billing identity for a user.
func (s *Service) UpsertBillingAccount(
	ctx context.Context,
	userID uint,
	provider,
	providerAccountID,
	email,
	accessTokenEnc,
	refreshTokenEnc string,
	tokenExpiresAt *time.Time,
) (*models.BillingAccount, error) {
	_ = ctx
	p := strings.ToLower(strings.TrimSpace(provider))
	paID := strings.TrimSpace(providerAccountID)
	if userID == 0 || p == "" || paID == "" {
		return nil, errors.New("user_id, provider and provider_account_id are required")
	}

	account := &models.BillingAccount{
		UserID:            userID,
		Provider:          p,
		ProviderAccountID: paID,
		Email:             strings.TrimSpace(email),
		AccessTokenEnc:    accessTokenEnc,
		RefreshTokenEnc:   refreshTokenEnc,
		TokenExpiresAt:    tokenExpiresAt,
	}
	if err := s.repo.UpsertBillingAccount(account); err != nil {
		return nil, err
	}
	return account, nil
}

// GetBillingAccountByProviderAccountID resolves a provider account to local account linkage.
func (s *Service) GetBillingAccountByProviderAccountID(ctx context.Context, provider, providerAccountID string) (*models.BillingAccount, error) {
	_ = ctx
	p := strings.ToLower(strings.TrimSpace(provider))
	paID := strings.TrimSpace(providerAccountID)
	if p == "" || paID == "" {
		return nil, errors.New("provider and provider_account_id are required")
	}
	return s.repo.GetBillingAccountByProviderAccountID(p, paID)
}

// ResolveMappedPlan resolves provider plan references to an internal plan.
func (s *Service) ResolveMappedPlan(ctx context.Context, provider, providerPlanRef, interval string) (string, error) {
	_ = ctx
	p := strings.ToLower(strings.TrimSpace(provider))
	ref := strings.TrimSpace(providerPlanRef)
	i := normalizeInterval(interval)
	if p == "" || ref == "" {
		return string(entitlements.PlanFree), errors.New("provider and provider plan ref are required")
	}

	// Prefer exact interval match.
	m, err := s.repo.FindActivePlanMapping(p, ref, i)
	if err == nil {
		return normalizePlan(m.InternalPlan), nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}

	// Fallback for mappings that intentionally use "unknown".
	m, err = s.repo.FindActivePlanMapping(p, ref, "unknown")
	if err == nil {
		return normalizePlan(m.InternalPlan), nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return string(entitlements.PlanFree), gorm.ErrRecordNotFound
	}
	return "", err
}

// ResolveBestMappedTier selects the best mapped internal plan from a list of
// provider plan refs and returns the winning provider plan ref + internal plan.
func (s *Service) ResolveBestMappedTier(ctx context.Context, provider string, providerPlanRefs []string, interval string) (string, string, error) {
	if len(providerPlanRefs) == 0 {
		return "", string(entitlements.PlanFree), gorm.ErrRecordNotFound
	}

	bestTier := ""
	bestPlan := string(entitlements.PlanFree)
	foundMapped := false
	seen := make(map[string]struct{}, len(providerPlanRefs))

	for _, raw := range providerPlanRefs {
		ref := strings.TrimSpace(raw)
		if ref == "" {
			continue
		}
		if _, ok := seen[ref]; ok {
			continue
		}
		seen[ref] = struct{}{}

		plan, err := s.ResolveMappedPlan(ctx, provider, ref, interval)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return "", "", err
		}

		if !foundMapped || planRank(plan) > planRank(bestPlan) {
			foundMapped = true
			bestTier = ref
			bestPlan = plan
		}
	}

	if foundMapped {
		return bestTier, bestPlan, nil
	}

	// Fallback: keep the first valid tier ref with free plan.
	for _, raw := range providerPlanRefs {
		ref := strings.TrimSpace(raw)
		if ref != "" {
			return ref, string(entitlements.PlanFree), gorm.ErrRecordNotFound
		}
	}
	return "", string(entitlements.PlanFree), gorm.ErrRecordNotFound
}

// SyncSubscription upserts provider subscription data and reconciles user plan.
func (s *Service) SyncSubscription(ctx context.Context, in NormalizedSubscription) (*models.BillingSubscription, string, error) {
	provider := strings.ToLower(strings.TrimSpace(in.Provider))
	if in.UserID == 0 || provider == "" || strings.TrimSpace(in.ProviderSubscriptionID) == "" {
		return nil, "", errors.New("user_id, provider and provider_subscription_id are required")
	}

	interval := normalizeInterval(in.BillingInterval)
	status := strings.ToLower(strings.TrimSpace(in.Status))
	if status == "" {
		status = models.BillingStatusActive
	}

	internalPlan, err := s.ResolveMappedPlan(ctx, provider, in.ProviderPlanRef, interval)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, "", err
	}
	if internalPlan == "" {
		internalPlan = string(entitlements.PlanFree)
	}

	sub := &models.BillingSubscription{
		UserID:                 in.UserID,
		Provider:               provider,
		ProviderSubscriptionID: strings.TrimSpace(in.ProviderSubscriptionID),
		ProviderPlanRef:        strings.TrimSpace(in.ProviderPlanRef),
		InternalPlan:           internalPlan,
		BillingInterval:        interval,
		Status:                 status,
		CurrentPeriodStart:     in.CurrentPeriodStart,
		CurrentPeriodEnd:       in.CurrentPeriodEnd,
		CancelAtPeriodEnd:      in.CancelAtPeriodEnd,
		RawPayloadJSON:         in.RawPayloadJSON,
	}
	if err := s.repo.UpsertSubscription(sub); err != nil {
		return nil, "", err
	}

	effectivePlan, err := s.ReconcileUserPlan(ctx, in.UserID)
	if err != nil {
		return sub, "", err
	}
	return sub, effectivePlan, nil
}

// ReconcileUserPlan computes and writes the best effective plan for a user.
func (s *Service) ReconcileUserPlan(ctx context.Context, userID uint) (string, error) {
	_ = ctx
	if userID == 0 {
		return "", errors.New("user_id is required")
	}

	subs, err := s.repo.ListSubscriptionsByUser(userID)
	if err != nil {
		return "", err
	}

	best := string(entitlements.PlanFree)
	for _, sub := range subs {
		if !isEntitlingStatus(sub.Status) {
			continue
		}
		candidate := normalizePlan(sub.InternalPlan)
		if planRank(candidate) > planRank(best) {
			best = candidate
		}
	}

	us, err := s.repo.GetOrCreateUserSettings(userID)
	if err != nil {
		return "", err
	}
	if normalizePlan(us.Plan) == best {
		return best, nil
	}
	us.Plan = best
	if err := s.repo.SaveUserSettings(us); err != nil {
		return "", err
	}
	return best, nil
}

// RecordWebhookEvent persists webhook payloads idempotently.
func (s *Service) RecordWebhookEvent(ctx context.Context, in WebhookEventInput) (bool, *models.BillingWebhookEvent, error) {
	_ = ctx
	provider := strings.ToLower(strings.TrimSpace(in.Provider))
	if provider == "" {
		return false, nil, errors.New("provider is required")
	}
	eventID := strings.TrimSpace(in.ProviderEventID)
	if eventID == "" {
		sum := sha256.Sum256([]byte(in.PayloadJSON))
		eventID = "hash:" + hex.EncodeToString(sum[:])
	}

	event := &models.BillingWebhookEvent{
		Provider:        provider,
		ProviderEventID: eventID,
		EventType:       strings.TrimSpace(in.EventType),
		PayloadJSON:     in.PayloadJSON,
		SignatureValid:  in.SignatureValid,
	}
	return s.repo.CreateWebhookEventIfNotExists(event)
}

// MarkWebhookProcessed marks an event as processed and stores an optional error.
func (s *Service) MarkWebhookProcessed(ctx context.Context, webhookEventID uint, processingErr error) error {
	_ = ctx
	if webhookEventID == 0 {
		return errors.New("webhook_event_id is required")
	}
	errMsg := ""
	if processingErr != nil {
		errMsg = processingErr.Error()
	}
	return s.repo.MarkWebhookProcessed(webhookEventID, errMsg)
}
