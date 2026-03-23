package domain

import (
	"context"

	"github.com/google/uuid"
)

// IngestedRecordRepository defines persistence operations for IngestedRecord.
type IngestedRecordRepository interface {
	// Create persists a new record and returns it with its generated ID.
	Create(ctx context.Context, record *IngestedRecord) error

	// GetByID fetches a single record by primary key.
	GetByID(ctx context.Context, id uuid.UUID) (*IngestedRecord, error)

	// List returns all records, optionally filtered by status.
	List(ctx context.Context, filter ListFilter) ([]*IngestedRecord, error)

	// UpdateJudgment persists the AI scoring fields on an existing record.
	UpdateJudgment(ctx context.Context, id uuid.UUID, score float64, status AIStatus, reasoning string) error
}

// HumanFeedbackRepository defines persistence operations for HumanFeedback.
type HumanFeedbackRepository interface {
	// Create persists a new human-feedback entry.
	Create(ctx context.Context, feedback *HumanFeedback) error

	// LatestBySystem returns the most recent n feedback entries for a given system.
	// These are used as Few-Shot examples in the AI Judge prompt.
	LatestBySystem(ctx context.Context, systemID SystemID, n int) ([]*HumanFeedback, error)
}

// AIJudge is the interface that any LLM provider must satisfy.
// Defining it as an interface allows the concrete provider (Azure OpenAI,
// OpenAI, Anthropic, etc.) to be swapped without touching business logic.
type AIJudge interface {
	// Judge evaluates a raw JSON payload from the given system, optionally
	// enriched with Few-Shot examples from previous human corrections.
	Judge(ctx context.Context, systemID SystemID, rawData []byte, fewShots []*HumanFeedback) (*JudgeResult, error)
}

// ListFilter contains optional predicates for listing ingested records.
type ListFilter struct {
	SystemID *SystemID
	Status   *AIStatus
	Limit    int
	Offset   int
}
