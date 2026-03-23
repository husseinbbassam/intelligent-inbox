package usecase

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/husseinbbassam/intelligent-inbox/internal/domain"
	"github.com/husseinbbassam/intelligent-inbox/internal/infrastructure"
)

const (
	// fewShotCount is the number of recent human-feedback examples injected
	// into the AI prompt so the judge learns from past corrections.
	fewShotCount = 5

	// workerInterval controls how often the background worker polls for PENDING records.
	workerInterval = 10 * time.Second
)

// AIJudgeUseCase orchestrates the asynchronous AI evaluation pipeline.
type AIJudgeUseCase struct {
	records  domain.IngestedRecordRepository
	feedback domain.HumanFeedbackRepository
	judge    domain.AIJudge
}

// NewAIJudgeUseCase creates a new AIJudgeUseCase.
func NewAIJudgeUseCase(
	records domain.IngestedRecordRepository,
	feedback domain.HumanFeedbackRepository,
	judge domain.AIJudge,
) *AIJudgeUseCase {
	return &AIJudgeUseCase{
		records:  records,
		feedback: feedback,
		judge:    judge,
	}
}

// StartWorker runs the polling loop in the calling goroutine.
// Cancel ctx to shut it down gracefully.
func (uc *AIJudgeUseCase) StartWorker(ctx context.Context) {
	log.Println("AI Judge worker started")
	ticker := time.NewTicker(workerInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("AI Judge worker stopped")
			return
		case <-ticker.C:
			if err := uc.processPendingBatch(ctx); err != nil {
				log.Printf("WARN: AI Judge worker batch error: %v", err)
			}
		}
	}
}

// processPendingBatch fetches all PENDING records and evaluates them one by one.
func (uc *AIJudgeUseCase) processPendingBatch(ctx context.Context) error {
	pending := domain.AIStatusPending
	records, err := uc.records.List(ctx, domain.ListFilter{
		Status: &pending,
		Limit:  100,
	})
	if err != nil {
		return fmt.Errorf("list pending records: %w", err)
	}

	if len(records) == 0 {
		return nil
	}

	log.Printf("AI Judge: processing %d pending record(s)", len(records))

	for _, rec := range records {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		uc.evaluateRecord(ctx, rec)
	}

	return nil
}

// evaluateRecord judges a single record and persists the result.
func (uc *AIJudgeUseCase) evaluateRecord(ctx context.Context, rec *domain.IngestedRecord) {
	fewShots, err := uc.feedback.LatestBySystem(ctx, rec.SystemID, fewShotCount)
	if err != nil {
		log.Printf("WARN: could not fetch few-shot examples for record %s: %v", rec.ID, err)
		// Proceed without few-shots rather than blocking evaluation.
		fewShots = nil
	}

	result, err := uc.judge.Judge(ctx, rec.SystemID, rec.RawData, fewShots)
	if err != nil {
		uc.handleJudgeError(ctx, rec, err)
		return
	}

	if updateErr := uc.records.UpdateJudgment(ctx, rec.ID, result.Score, result.Status, result.Reasoning); updateErr != nil {
		log.Printf("ERROR: failed to persist judgment for record %s: %v", rec.ID, updateErr)
	} else {
		log.Printf("AI Judge: record %s → %s (score=%.2f)", rec.ID, result.Status, result.Score)
	}
}

// handleJudgeError gracefully handles AI Judge failures.
// Transient errors (401, 429, 500) move the record to PENDING_RETRY; others
// are logged but do not crash the worker.
func (uc *AIJudgeUseCase) handleJudgeError(ctx context.Context, rec *domain.IngestedRecord, err error) {
	var transient *infrastructure.TransientError
	if errors.As(err, &transient) {
		log.Printf("WARN: transient AI error for record %s (HTTP %d) — marking PENDING_RETRY: %v",
			rec.ID, transient.StatusCode, err)
		if updateErr := uc.records.UpdateJudgment(ctx, rec.ID, 0, domain.AIStatusPendingRetry, err.Error()); updateErr != nil {
			log.Printf("ERROR: failed to mark record %s as PENDING_RETRY: %v", rec.ID, updateErr)
		}
		return
	}

	log.Printf("WARN: non-transient AI error for record %s — skipping: %v", rec.ID, err)
}
