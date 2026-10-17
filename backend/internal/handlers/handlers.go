// Package handlers exposes the HTTP API (Gin) on top of the domain services.
package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"flowmonitor/backend/internal/config"
	"flowmonitor/backend/internal/service"
)

// Handler carries the services every endpoint needs.
type Handler struct {
	svc *service.Services
	cfg *config.Config
}

// New builds the handler set.
func New(svc *service.Services) *Handler {
	return &Handler{svc: svc, cfg: svc.Cfg}
}

// fail maps a domain error to the right HTTP status.
func (h *Handler) fail(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrUnknownFlow):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}
}

// Health reports service and database liveness.
func (h *Handler) Health(c *gin.Context) {
	status := "ok"
	dbStatus := "ok"
	if sqlDB, err := h.svc.DB.DB(); err != nil || sqlDB.Ping() != nil {
		status = "degraded"
		dbStatus = "unreachable"
	}
	c.JSON(http.StatusOK, gin.H{
		"status":       status,
		"database":     dbStatus,
		"timestamp":    time.Now().UTC().Format(time.RFC3339),
		"autoRegister": h.cfg.AutoRegister,
		"ingestAuth":   h.cfg.IngestAPIKey != "",
	})
}

// RequireIngestKey enforces X-API-Key on ingestion routes when configured.
func (h *Handler) RequireIngestKey() gin.HandlerFunc {
	if h.cfg.IngestAPIKey == "" {
		return func(c *gin.Context) { c.Next() }
	}
	return func(c *gin.Context) {
		if strings.TrimSpace(c.GetHeader("X-API-Key")) != h.cfg.IngestAPIKey {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or missing X-API-Key"})
			return
		}
		c.Next()
	}
}

// ---------------------------------------------------------------------------
// query param helpers
// ---------------------------------------------------------------------------

func queryInt(c *gin.Context, key string, fallback int) int {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func queryUint(c *gin.Context, key string) uint {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return 0
	}
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0
	}
	return uint(value)
}

// paramUint parses a numeric path parameter (":id", ":stepId", ...).
func paramUint(c *gin.Context, key string) uint {
	value, err := strconv.ParseUint(strings.TrimSpace(c.Param(key)), 10, 64)
	if err != nil {
		return 0
	}
	return uint(value)
}

func queryBool(c *gin.Context, key string) bool {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return false
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false
	}
	return value
}

// queryTime parses RFC3339 timestamps (or unix seconds) for range filters.
func queryTime(c *gin.Context, key string) *time.Time {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return nil
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		utc := t.UTC()
		return &utc
	}
	if seconds, err := strconv.ParseInt(raw, 10, 64); err == nil {
		utc := time.Unix(seconds, 0).UTC()
		return &utc
	}
	return nil
}
