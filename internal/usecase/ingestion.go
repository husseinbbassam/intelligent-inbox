package usecase

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/husseinbbassam/intelligent-inbox/internal/domain"
)

// IngestionUseCase handles the validation and persistence of incoming data.
type IngestionUseCase struct {
	records domain.IngestedRecordRepository
}

// NewIngestionUseCase creates a new IngestionUseCase.
func NewIngestionUseCase(records domain.IngestedRecordRepository) *IngestionUseCase {
	return &IngestionUseCase{records: records}
}

// Ingest validates the payload for the given system, stores it as a PENDING
// record, and returns the created record.
func (uc *IngestionUseCase) Ingest(ctx context.Context, systemID domain.SystemID, payload []byte) (*domain.IngestedRecord, error) {
	if err := validatePayload(systemID, payload); err != nil {
		return nil, fmt.Errorf("payload validation: %w", err)
	}

	status := domain.AIStatusPending
	record := &domain.IngestedRecord{
		SystemID: systemID,
		RawData:  payload,
		AIStatus: status,
	}

	if err := uc.records.Create(ctx, record); err != nil {
		return nil, fmt.Errorf("store record: %w", err)
	}

	return record, nil
}

// validatePayload performs structural validation of the JSON payload per system.
// It ensures required fields are present before the record is stored.
func validatePayload(systemID domain.SystemID, payload []byte) error {
	if !json.Valid(payload) {
		return fmt.Errorf("invalid JSON")
	}

	switch systemID {
	case domain.SystemProcurement:
		var v domain.ProcurementInvoice
		if err := json.Unmarshal(payload, &v); err != nil {
			return fmt.Errorf("unmarshal procurement invoice: %w", err)
		}
		if v.InvoiceID == "" {
			return fmt.Errorf("invoice_id is required")
		}
		if v.VendorName == "" {
			return fmt.Errorf("vendor_name is required")
		}
		if v.Description == "" {
			return fmt.Errorf("description is required")
		}

	case domain.SystemIoT:
		var v domain.IoTTelemetry
		if err := json.Unmarshal(payload, &v); err != nil {
			return fmt.Errorf("unmarshal iot telemetry: %w", err)
		}
		if v.DeviceID == "" {
			return fmt.Errorf("device_id is required")
		}
		if v.SensorType == "" {
			return fmt.Errorf("sensor_type is required")
		}
		if v.Timestamp.IsZero() {
			return fmt.Errorf("timestamp is required")
		}

	case domain.SystemThree, domain.SystemFour:
		var v domain.GenericPayload
		if err := json.Unmarshal(payload, &v); err != nil {
			return fmt.Errorf("unmarshal generic payload: %w", err)
		}
		if v.SourceKey == "" {
			return fmt.Errorf("source_key is required")
		}

	default:
		return fmt.Errorf("unknown system_id: %q", systemID)
	}

	return nil
}
