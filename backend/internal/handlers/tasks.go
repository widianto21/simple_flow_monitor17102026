package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"flowmonitor/backend/internal/service"
)

// ListStuck returns tasks that are running beyond their SLA.
//
// GET /api/tasks/stuck
func (h *Handler) ListStuck(c *gin.Context) {
	tasks, err := h.svc.Query.ListStuck(queryInt(c, "limit", 50))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": tasks, "total": len(tasks)})
}

// ListAlerts returns alerts filtered by lifecycle state.
//
// GET /api/alerts?status=open|acked|resolved|all&limit=
func (h *Handler) ListAlerts(c *gin.Context) {
	alerts, err := h.svc.Query.ListAlerts(c.Query("status"), queryInt(c, "limit", 100))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": alerts, "total": len(alerts)})
}

// AckAlert acknowledges an alert.
//
// POST /api/alerts/:id/ack
func (h *Handler) AckAlert(c *gin.Context) {
	alert, err := h.svc.Query.AckAlert(paramUint(c, "id"))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, alert)
}

// Dashboard returns KPIs, charts and previews in one round-trip.
//
// GET /api/dashboard
func (h *Handler) Dashboard(c *gin.Context) {
	board, err := h.svc.Query.Dashboard()
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, board)
}

// Simulate generates extra demo traffic.
//
// POST /api/demo/seed
func (h *Handler) Simulate(c *gin.Context) {
	opts := service.SimOptions{Hours: 24, RunsPerFlow: 12, ActiveRuns: 2}
	if err := c.ShouldBindJSON(&opts); err != nil && err.Error() != "EOF" {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	stats, err := h.svc.Simulate.Run(c.Request.Context(), opts)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, stats)
}
