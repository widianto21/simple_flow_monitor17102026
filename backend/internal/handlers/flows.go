package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"flowmonitor/backend/internal/dto"
)

// ListFlows returns every flow with live counters.
//
// GET /api/flows
func (h *Handler) ListFlows(c *gin.Context) {
	flows, err := h.svc.Query.ListFlows()
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, flows)
}

// CreateFlow creates a flow definition.
//
// POST /api/flows
func (h *Handler) CreateFlow(c *gin.Context) {
	var input dto.FlowInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	detail, err := h.svc.Query.SaveFlow("", input)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, detail)
}

// GetFlow returns the full definition with live node stats.
//
// GET /api/flows/:idOrKey
func (h *Handler) GetFlow(c *gin.Context) {
	detail, err := h.svc.Query.GetFlow(c.Param("idOrKey"))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, detail)
}

// UpdateFlow replaces a flow definition (editor save).
//
// PUT /api/flows/:idOrKey
func (h *Handler) UpdateFlow(c *gin.Context) {
	var input dto.FlowInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	detail, err := h.svc.Query.SaveFlow(c.Param("idOrKey"), input)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, detail)
}

// DeleteFlow removes a flow and its telemetry.
//
// DELETE /api/flows/:idOrKey
func (h *Handler) DeleteFlow(c *gin.Context) {
	if err := h.svc.Query.DeleteFlow(c.Param("idOrKey")); err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

// FlowStats returns per-node live/historic statistics.
//
// GET /api/flows/:idOrKey/stats
func (h *Handler) FlowStats(c *gin.Context) {
	nodes, err := h.svc.Query.FlowNodeStats(c.Param("idOrKey"))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, nodes)
}

// NodeDetail returns one diagram component: contract, stats and task history.
//
// GET /api/flows/:idOrKey/nodes/:nodeKey
func (h *Handler) NodeDetail(c *gin.Context) {
	detail, err := h.svc.Query.NodeDetail(c.Param("idOrKey"), c.Param("nodeKey"), queryInt(c, "limit", 25))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, detail)
}
