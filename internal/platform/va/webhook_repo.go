package va

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

// WebhookRepo handles va_webhooks table operations
type WebhookRepo struct {
	db *gorm.DB
}

// NewWebhookRepo creates a new VA webhook repository
func NewWebhookRepo(db *gorm.DB) *WebhookRepo {
	return &WebhookRepo{db: db}
}

// ListByVA returns all webhooks for a VA
func (r *WebhookRepo) ListByVA(ctx context.Context, vaID string) ([]VAWebhook, error) {
	var list []VAWebhook
	err := r.db.WithContext(ctx).
		Where("va_id = ?", vaID).
		Order("created_at ASC").
		Find(&list).Error
	if err != nil {
		return nil, fmt.Errorf("list webhooks by VA: %w", err)
	}
	return list, nil
}

// ListActiveByType returns all active webhooks for a given type (e.g. live_flights)
// Used by the scheduled job to find VAs to notify
func (r *WebhookRepo) ListActiveByType(ctx context.Context, webhookType string) ([]VAWebhook, error) {
	var list []VAWebhook
	err := r.db.WithContext(ctx).
		Where("webhook_type = ? AND is_active = ?", webhookType, true).
		Find(&list).Error
	if err != nil {
		return nil, fmt.Errorf("list active webhooks by type: %w", err)
	}
	return list, nil
}

// GetWebhookByID returns a webhook by ID
func (r *WebhookRepo) GetWebhookByID(ctx context.Context, id string) (*VAWebhook, error) {
	var w VAWebhook
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&w).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get webhook by ID: %w", err)
	}
	return &w, nil
}

// CreateWebhook creates a new webhook
func (r *WebhookRepo) CreateWebhook(ctx context.Context, w *VAWebhook) error {
	if err := r.db.WithContext(ctx).Create(w).Error; err != nil {
		return fmt.Errorf("create webhook: %w", err)
	}
	return nil
}

// UpdateWebhook updates an existing webhook
func (r *WebhookRepo) UpdateWebhook(ctx context.Context, w *VAWebhook) error {
	if err := r.db.WithContext(ctx).Save(w).Error; err != nil {
		return fmt.Errorf("update webhook: %w", err)
	}
	return nil
}

// DeleteWebhook deletes a webhook by ID
func (r *WebhookRepo) DeleteWebhook(ctx context.Context, id string) error {
	res := r.db.WithContext(ctx).Where("id = ?", id).Delete(&VAWebhook{})
	if res.Error != nil {
		return fmt.Errorf("delete webhook: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
