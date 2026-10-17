package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"flowmonitor/backend/internal/service"
)

// ListRuns lists executions.
//
// GET /api/runs?flowKey=&status=&businessKey=&nodeKey=&from=&to=&page=&size=
func (h *Handler) ListRuns(c *gin.Context) {
	filter := service.RunFilter{
		FlowKey:     strings.TrimSpace(c.Query("flowKey")),
		Status:      normalizeStatus(c.Query("status")),
		BusinessKey: strings.TrimSpace(c.Query("businessKey")),
		NodeKey:     strings.TrimSpace(c.Query("nodeKey")),
		From:        queryTime(c, "from"),
		To:          queryTime(c, "to"),
		Page:        queryInt(c, "page", 1),
		Size:        queryInt(c, "size", 20),
	}
	page, err := h.svc.Query.ListRuns(filter)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, page)
}

// GetRun returns one execution with all its tasks, events and prior runs.
//
// GET /api/runs/:id
func (h *Handler) GetRun(c *gin.Context) {
	run, err := h.svc.Query.GetRun(paramUint(c, "id"))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, run)
}

// GetStep returns one task: payloads, timings, siblings and node history.
//
// GET /api/runs/:id/steps/:stepId
func (h *Handler) GetStep(c *gin.Context) {
	step, err := h.svc.Query.GetStep(paramUint(c, "stepId"))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, step)
}

// ListSteps searches tasks across flows.
//
// GET /api/steps?flowKey=&nodeKey=&status=&businessKey=&stuck=&from=&to=&page=&size=
func (h *Handler) ListSteps(c *gin.Context) {
	filter := service.StepFilter{
		FlowKey:     strings.TrimSpace(c.Query("flowKey")),
		NodeKey:     strings.TrimSpace(c.Query("nodeKey")),
		Status:      normalizeStatus(c.Query("status")),
		BusinessKey: strings.TrimSpace(c.Query("businessKey")),
		StuckOnly:   queryBool(c, "stuck"),
		From:        queryTime(c, "from"),
		To:          queryTime(c, "to"),
		Page:        queryInt(c, "page", 1),
		Size:        queryInt(c, "size", 25),
	}
	page, err := h.svc.Query.ListSteps(filter)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, page)
}

// BusinessKeyHistory returns every execution recorded for a business key.
//
// GET /api/business-keys/:key/runs
func (h *Handler) BusinessKeyHistory(c *gin.Context) {
	runs, err := h.svc.Query.BusinessKeyHistory(c.Param("key"), queryInt(c, "limit", 20))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, runs)
}

// normalizeStatus keeps the "all" sentinel out of queries.
func normalizeStatus(raw string) string {
	status := strings.ToLower(strings.TrimSpace(raw))
	if status == "all" {
		return ""
	}
	return status
}
