package api

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// NewRouter configures and returns the Echo router with all routes registered.
func NewRouter(h *Handler) *echo.Echo {
	e := echo.New()

	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.RequestID())

	// Health check
	e.GET("/health", h.HealthCheck)

	// API v1
	v1 := e.Group("/api/v1")

	// Data Ingestion — one endpoint supports all system IDs
	v1.POST("/ingest/:systemId", h.Ingest)

	// Record querying
	v1.GET("/records", h.ListRecords)
	v1.GET("/records/:id", h.GetRecord)

	// Human-in-the-loop feedback
	v1.POST("/feedback", h.SubmitFeedback)

	return e
}
