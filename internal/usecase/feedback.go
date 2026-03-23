package usecase

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/husseinbbassam/intelligent-inbox/internal/domain"
)

// FeedbackUseCase handles human-in-the-loop corrections.
type FeedbackUseCase struct {
	records  domain.IngestedRecordRepository
	feedback domain.HumanFeedbackRepository
}

// NewFeedbackUseCase creates a new FeedbackUseCase.
func NewFeedbackUseCase(
	records domain.IngestedRecordRepository,
	feedback domain.HumanFeedbackRepository,
) *FeedbackUseCase {
	return &FeedbackUseCase{records: records, feedback: feedback}
}

// SubmitFeedback records a human correction for an existing ingested record.
// The correct label must be either CLEAN or NOISE.
func (uc *FeedbackUseCase) SubmitFeedback(
	ctx context.Context,
	recordID uuid.UUID,
	correctLabel domain.AIStatus,
	correctionNote string,
) (*domain.HumanFeedback, error) {
	if correctLabel != domain.AIStatusClean && correctLabel != domain.AIStatusNoise {
		return nil, fmt.Errorf("correct_label must be CLEAN or NOISE, got %q", correctLabel)
	}

	rec, err := uc.records.GetByID(ctx, recordID)
	if err != nil {
		return nil, fmt.Errorf("fetch record for feedback: %w", err)
	}

	fb := &domain.HumanFeedback{
		SystemID:       rec.SystemID,
		RecordID:       rec.ID,
		OriginalData:   rec.RawData,
		CorrectLabel:   correctLabel,
		CorrectionNote: correctionNote,
	}

	if err = uc.feedback.Create(ctx, fb); err != nil {
		return nil, fmt.Errorf("persist feedback: %w", err)
	}

	return fb, nil
}
