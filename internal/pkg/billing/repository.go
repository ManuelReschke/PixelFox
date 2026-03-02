package billing

import (
	"time"

	"github.com/ManuelReschke/PixelFox/app/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Repository provides DB operations used by the billing service.
type Repository interface {
	FindActivePlanMapping(provider, providerPlanRef, interval string) (*models.BillingPlanMapping, error)
	UpsertBillingAccount(account *models.BillingAccount) error
	GetBillingAccountByProviderAccountID(provider, providerAccountID string) (*models.BillingAccount, error)
	UpsertSubscription(sub *models.BillingSubscription) error
	ListSubscriptionsByUser(userID uint) ([]models.BillingSubscription, error)
	GetOrCreateUserSettings(userID uint) (*models.UserSettings, error)
	SaveUserSettings(us *models.UserSettings) error
	CreateWebhookEventIfNotExists(event *models.BillingWebhookEvent) (bool, *models.BillingWebhookEvent, error)
	MarkWebhookProcessed(id uint, processingError string) error
}

type gormRepository struct {
	db *gorm.DB
}

// NewRepository creates a billing repository backed by GORM.
func NewRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) FindActivePlanMapping(provider, providerPlanRef, interval string) (*models.BillingPlanMapping, error) {
	var m models.BillingPlanMapping
	err := r.db.
		Where("provider = ? AND provider_plan_ref = ? AND billing_interval = ? AND is_active = ?", provider, providerPlanRef, interval, true).
		First(&m).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *gormRepository) UpsertBillingAccount(account *models.BillingAccount) error {
	if err := r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "provider"},
			{Name: "provider_account_id"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"user_id",
			"email",
			"access_token_enc",
			"refresh_token_enc",
			"token_expires_at",
			"updated_at",
		}),
	}).Create(account).Error; err != nil {
		return err
	}

	return r.db.Where("provider = ? AND provider_account_id = ?", account.Provider, account.ProviderAccountID).
		First(account).Error
}

func (r *gormRepository) GetBillingAccountByProviderAccountID(provider, providerAccountID string) (*models.BillingAccount, error) {
	var account models.BillingAccount
	err := r.db.Where("provider = ? AND provider_account_id = ?", provider, providerAccountID).First(&account).Error
	if err != nil {
		return nil, err
	}
	return &account, nil
}

func (r *gormRepository) UpsertSubscription(sub *models.BillingSubscription) error {
	if err := r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "provider"},
			{Name: "provider_subscription_id"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"user_id",
			"provider_plan_ref",
			"internal_plan",
			"billing_interval",
			"status",
			"current_period_start",
			"current_period_end",
			"cancel_at_period_end",
			"raw_payload_json",
			"updated_at",
		}),
	}).Create(sub).Error; err != nil {
		return err
	}

	// Ensure ID is populated after upsert.
	return r.db.Where("provider = ? AND provider_subscription_id = ?", sub.Provider, sub.ProviderSubscriptionID).
		First(sub).Error
}

func (r *gormRepository) ListSubscriptionsByUser(userID uint) ([]models.BillingSubscription, error) {
	var subs []models.BillingSubscription
	err := r.db.Where("user_id = ?", userID).Find(&subs).Error
	return subs, err
}

func (r *gormRepository) GetOrCreateUserSettings(userID uint) (*models.UserSettings, error) {
	return models.GetOrCreateUserSettings(r.db, userID)
}

func (r *gormRepository) SaveUserSettings(us *models.UserSettings) error {
	return r.db.Save(us).Error
}

func (r *gormRepository) CreateWebhookEventIfNotExists(event *models.BillingWebhookEvent) (bool, *models.BillingWebhookEvent, error) {
	tx := r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "provider"},
			{Name: "provider_event_id"},
		},
		DoNothing: true,
	}).Create(event)
	if tx.Error != nil {
		return false, nil, tx.Error
	}

	created := tx.RowsAffected > 0
	var stored models.BillingWebhookEvent
	if err := r.db.Where("provider = ? AND provider_event_id = ?", event.Provider, event.ProviderEventID).
		First(&stored).Error; err != nil {
		return false, nil, err
	}
	return created, &stored, nil
}

func (r *gormRepository) MarkWebhookProcessed(id uint, processingError string) error {
	now := time.Now()
	updates := map[string]interface{}{
		"processed_at":     &now,
		"processing_error": processingError,
	}
	return r.db.Model(&models.BillingWebhookEvent{}).Where("id = ?", id).Updates(updates).Error
}
