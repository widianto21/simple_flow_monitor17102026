package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"flowmonitor/backend/internal/dto"
	"flowmonitor/backend/internal/service"
)

// IngestEvent logs one task event coming from another system.
//
// POST /api/ingest/events
func (h *Handler) IngestEvent(c *gin.Context) {
	var event dto.IngestEvent
	if err := c.ShouldBindJSON(&event); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON body: " + err.Error()})
		return
	}
	result, err := h.svc.Ingest.Process(c.Request.Context(), event)
	if err != nil {
		h.ingestFail(c, err)
		return
	}
	c.JSON(http.StatusAccepted, result)
}

// IngestBatch logs many events in one call.
//
// POST /api/ingest/events/batch
func (h *Handler) IngestBatch(c *gin.Context) {
	var events []dto.IngestEvent
	if err := c.ShouldBindJSON(&events); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON body (expected an array of events): " + err.Error()})
		return
	}
	if len(events) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one event is required"})
		return
	}

	results := h.svc.Ingest.ProcessBatch(c.Request.Context(), events)
	accepted, duplicates := 0, 0
	for _, r := range results {
		if r.Accepted {
			accepted++
		}
		if r.Duplicate {
			duplicates++
		}
	}
	c.JSON(http.StatusAccepted, gin.H{
		"received":   len(events),
		"accepted":   accepted,
		"rejected":   len(events) - accepted,
		"duplicates": duplicates,
		"results":    results,
	})
}

// Manifest describes how to instrument a flow: every step with the system it
// lives in, the endpoint, accepted parameters and a sample response.
//
// GET /api/ingest/manifests/:flowKey
func (h *Handler) Manifest(c *gin.Context) {
	detail, err := h.svc.Query.GetFlow(c.Param("flowKey"))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"flow": gin.H{
			"key":         detail.Key,
			"name":        detail.Name,
			"description": detail.Description,
			"category":    detail.Category,
			"owner":       detail.Owner,
			"version":     detail.Version,
		},
		"ingest": gin.H{
			"singleEndpoint": "/api/ingest/events",
			"batchEndpoint":  "/api/ingest/events/batch",
			"method":         "POST",
			"authHeader":     "X-API-Key",
			"statuses":       []string{"started", "completed", "failed", "skipped"},
			"businessKey":    "your own correlation id (order id, claim id, ...)",
		},
		"nodes": detail.NodeContracts,
	})
}

// ingestFail keeps ingestion errors meaningful for instrumenting developers.
func (h *Handler) ingestFail(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrUnknownFlow):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
	case strings.Contains(err.Error(), "no active execution"):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case strings.Contains(err.Error(), "is required"), strings.Contains(err.Error(), "invalid"):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}
}
