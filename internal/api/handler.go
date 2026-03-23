package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/husseinbbassam/intelligent-inbox/internal/domain"
	"github.com/husseinbbassam/intelligent-inbox/internal/usecase"
)

// Handler holds the use-case dependencies for all HTTP endpoints.
type Handler struct {
	ingestion *usecase.IngestionUseCase
	feedback  *usecase.FeedbackUseCase
	records   domain.IngestedRecordRepository
}

// NewHandler creates a new Handler.
func NewHandler(
	ingestion *usecase.IngestionUseCase,
	feedback *usecase.FeedbackUseCase,
	records domain.IngestedRecordRepository,
) *Handler {
	return &Handler{
		ingestion: ingestion,
		feedback:  feedback,
		records:   records,
	}
}

// HealthCheck godoc
// GET /health
func (h *Handler) HealthCheck(c echo.Context) error {
	return c.JSON(http.StatusOK, echo.Map{"status": "ok"})
}

// Ingest godoc
// POST /api/v1/ingest/:systemId
// Accepts a raw JSON body and creates a new PENDING ingested record.
func (h *Handler) Ingest(c echo.Context) error {
	systemID := domain.SystemID(c.Param("systemId"))

	// Read body as raw JSON to preserve it for JSONB storage.
	var raw json.RawMessage
	if err := c.Bind(&raw); err != nil || !json.Valid(raw) {
		return echo.NewHTTPError(http.StatusBadRequest, "request body must be valid JSON")
	}

	record, err := h.ingestion.Ingest(c.Request().Context(), systemID, raw)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, err.Error())
	}

	return c.JSON(http.StatusAccepted, record)
}

// ListRecords godoc
// GET /api/v1/records?system_id=&status=&limit=&offset=
func (h *Handler) ListRecords(c echo.Context) error {
	filter := domain.ListFilter{}

	if s := c.QueryParam("system_id"); s != "" {
		sid := domain.SystemID(s)
		filter.SystemID = &sid
	}
	if s := c.QueryParam("status"); s != "" {
		st := domain.AIStatus(s)
		filter.Status = &st
	}
	if l := queryInt(c, "limit", 50); l > 0 {
		filter.Limit = l
	}
	if o := queryInt(c, "offset", 0); o >= 0 {
		filter.Offset = o
	}

	records, err := h.records.List(c.Request().Context(), filter)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list records")
	}

	return c.JSON(http.StatusOK, records)
}

// GetRecord godoc
// GET /api/v1/records/:id
func (h *Handler) GetRecord(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid record id")
	}

	record, err := h.records.GetByID(c.Request().Context(), id)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "record not found")
	}

	return c.JSON(http.StatusOK, record)
}

// SubmitFeedback godoc
// POST /api/v1/feedback
// Body: { "record_id": "<uuid>", "correct_label": "CLEAN|NOISE", "correction_note": "..." }
func (h *Handler) SubmitFeedback(c echo.Context) error {
	var req struct {
		RecordID       string           `json:"record_id"`
		CorrectLabel   domain.AIStatus  `json:"correct_label"`
		CorrectionNote string           `json:"correction_note"`
	}

	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	recordID, err := uuid.Parse(req.RecordID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid record_id")
	}

	fb, err := h.feedback.SubmitFeedback(
		c.Request().Context(),
		recordID,
		req.CorrectLabel,
		req.CorrectionNote,
	)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, err.Error())
	}

	return c.JSON(http.StatusCreated, fb)
}

// queryInt reads a query-string integer with a fallback default.
func queryInt(c echo.Context, key string, defaultVal int) int {
	v := c.QueryParam(key)
	if v == "" {
		return defaultVal
	}
	var n int
	if _, err := fmt.Sscanf(v, "%d", &n); err != nil {
		return defaultVal
	}
	return n
}
