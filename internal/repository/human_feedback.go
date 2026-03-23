package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/husseinbbassam/intelligent-inbox/internal/domain"
)

// humanFeedbackRepo is the GORM-backed implementation of domain.HumanFeedbackRepository.
type humanFeedbackRepo struct {
	db *gorm.DB
}

// NewHumanFeedbackRepository creates a new repository backed by the given *gorm.DB.
func NewHumanFeedbackRepository(db *gorm.DB) domain.HumanFeedbackRepository {
	return &humanFeedbackRepo{db: db}
}

func (r *humanFeedbackRepo) Create(ctx context.Context, feedback *domain.HumanFeedback) error {
	if err := r.db.WithContext(ctx).Create(feedback).Error; err != nil {
		return fmt.Errorf("create human feedback: %w", err)
	}
	return nil
}

func (r *humanFeedbackRepo) LatestBySystem(
	ctx context.Context,
	systemID domain.SystemID,
	n int,
) ([]*domain.HumanFeedback, error) {
	var feedbacks []*domain.HumanFeedback
	if err := r.db.WithContext(ctx).
		Where("system_id = ?", systemID).
		Order("created_at DESC").
		Limit(n).
		Find(&feedbacks).Error; err != nil {
		return nil, fmt.Errorf("latest feedback by system: %w", err)
	}
	return feedbacks, nil
}
