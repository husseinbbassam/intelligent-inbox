package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/husseinbbassam/intelligent-inbox/internal/domain"
)

// ingestedRecordRepo is the GORM-backed implementation of domain.IngestedRecordRepository.
type ingestedRecordRepo struct {
	db *gorm.DB
}

// NewIngestedRecordRepository creates a new repository backed by the given *gorm.DB.
func NewIngestedRecordRepository(db *gorm.DB) domain.IngestedRecordRepository {
	return &ingestedRecordRepo{db: db}
}

func (r *ingestedRecordRepo) Create(ctx context.Context, record *domain.IngestedRecord) error {
	if err := r.db.WithContext(ctx).Create(record).Error; err != nil {
		return fmt.Errorf("create ingested record: %w", err)
	}
	return nil
}

func (r *ingestedRecordRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.IngestedRecord, error) {
	var record domain.IngestedRecord
	if err := r.db.WithContext(ctx).First(&record, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("get ingested record by id: %w", err)
	}
	return &record, nil
}

func (r *ingestedRecordRepo) List(ctx context.Context, filter domain.ListFilter) ([]*domain.IngestedRecord, error) {
	query := r.db.WithContext(ctx).Model(&domain.IngestedRecord{})

	if filter.SystemID != nil {
		query = query.Where("system_id = ?", *filter.SystemID)
	}
	if filter.Status != nil {
		query = query.Where("ai_status = ?", *filter.Status)
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	query = query.Limit(limit).Offset(filter.Offset).Order("created_at DESC")

	var records []*domain.IngestedRecord
	if err := query.Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list ingested records: %w", err)
	}
	return records, nil
}

func (r *ingestedRecordRepo) UpdateJudgment(
	ctx context.Context,
	id uuid.UUID,
	score float64,
	status domain.AIStatus,
	reasoning string,
) error {
	result := r.db.WithContext(ctx).Model(&domain.IngestedRecord{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"ai_score":     score,
			"ai_status":    status,
			"ai_reasoning": reasoning,
		})

	if result.Error != nil {
		return fmt.Errorf("update judgment: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("update judgment: record %s not found", id)
	}
	return nil
}
