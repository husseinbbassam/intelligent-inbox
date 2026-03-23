package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// AIStatus represents the classification state of an ingested record.
type AIStatus string

const (
	AIStatusPending      AIStatus = "PENDING"
	AIStatusPendingRetry AIStatus = "PENDING_RETRY"
	AIStatusClean        AIStatus = "CLEAN"
	AIStatusNoise        AIStatus = "NOISE"
)

// SystemID identifies the source system that produced a record.
type SystemID string

const (
	SystemProcurement SystemID = "procurement"
	SystemIoT         SystemID = "iot"
	SystemThree       SystemID = "system3"
	SystemFour        SystemID = "system4"
)

// IngestedRecord is the central entity stored in the ingested_records table.
type IngestedRecord struct {
	ID          uuid.UUID       `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	SystemID    SystemID        `gorm:"not null;index"                                 json:"system_id"`
	RawData     json.RawMessage `gorm:"type:jsonb;not null"                            json:"raw_data"`
	AIScore     *float64        `gorm:"column:ai_score"                                json:"ai_score"`
	AIStatus    AIStatus        `gorm:"column:ai_status;default:'PENDING'"             json:"ai_status"`
	AIReasoning string          `gorm:"column:ai_reasoning;type:text"                  json:"ai_reasoning"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// HumanFeedback stores corrections made by human reviewers.
// It is used as Few-Shot context for the AI Judge.
type HumanFeedback struct {
	ID             uuid.UUID       `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	SystemID       SystemID        `gorm:"not null;index"                                 json:"system_id"`
	RecordID       uuid.UUID       `gorm:"type:uuid;index"                                json:"record_id"`
	OriginalData   json.RawMessage `gorm:"type:jsonb"                                     json:"original_data"`
	CorrectLabel   AIStatus        `gorm:"not null"                                       json:"correct_label"`
	CorrectionNote string          `gorm:"type:text"                                      json:"correction_note"`
	CreatedAt      time.Time       `json:"created_at"`
}

// JudgeResult is the structured output produced by an AIJudge evaluation.
type JudgeResult struct {
	Score     float64  `json:"score"`
	Status    AIStatus `json:"status"`
	Reasoning string   `json:"reasoning"`
}

// ProcurementInvoice is the payload schema for System 1.
type ProcurementInvoice struct {
	InvoiceID   string  `json:"invoice_id"   validate:"required"`
	VendorName  string  `json:"vendor_name"  validate:"required"`
	TotalAmount float64 `json:"total_amount" validate:"required"`
	Description string  `json:"description"  validate:"required"`
}

// IoTTelemetry is the payload schema for System 2.
type IoTTelemetry struct {
	DeviceID   string    `json:"device_id"   validate:"required"`
	SensorType string    `json:"sensor_type" validate:"required"`
	Value      float64   `json:"value"`
	Timestamp  time.Time `json:"timestamp"   validate:"required"`
}

// GenericPayload is the placeholder schema for Systems 3 & 4.
type GenericPayload struct {
	SourceKey string          `json:"source_key" validate:"required"`
	Data      json.RawMessage `json:"data"       validate:"required"`
}
