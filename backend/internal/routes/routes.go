package routes

import (
	"github.com/gin-gonic/gin"

	"flowmonitor/backend/internal/config"
	"flowmonitor/backend/internal/handlers"
	"flowmonitor/backend/internal/service"
)

// New builds the Gin router with every API route registered.
func New(svc *service.Services, cfg *config.Config) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery(), corsMiddleware(cfg.CORSAllow))

	h := handlers.New(svc)
	api := r.Group("/api")
	{
		api.GET("/health", h.Health)
		api.GET("/dashboard", h.Dashboard)

		// ------------------------------------------------------------------
		// Ingestion API — called by other systems to log their task flow.
		// ------------------------------------------------------------------
		ingest := api.Group("/ingest")
		ingest.Use(h.RequireIngestKey())
		{
			ingest.POST("/events", h.IngestEvent)
			ingest.POST("/events/batch", h.IngestBatch)
			ingest.GET("/manifests/:flowKey", h.Manifest)
		}

		// ------------------------------------------------------------------
		// Flow definitions (diagram + node contracts).
		// ------------------------------------------------------------------
		flows := api.Group("/flows")
		{
			flows.GET("", h.ListFlows)
			flows.POST("", h.CreateFlow)
			flows.GET("/:idOrKey", h.GetFlow)
			flows.PUT("/:idOrKey", h.UpdateFlow)
			flows.DELETE("/:idOrKey", h.DeleteFlow)
			flows.GET("/:idOrKey/stats", h.FlowStats)
			flows.GET("/:idOrKey/nodes/:nodeKey", h.NodeDetail)
		}

		// ------------------------------------------------------------------
		// Executions & tasks.
		// ------------------------------------------------------------------
		runs := api.Group("/runs")
		{
			runs.GET("", h.ListRuns)
			runs.GET("/:id", h.GetRun)
			runs.GET("/:id/steps/:stepId", h.GetStep)
		}
		api.GET("/steps", h.ListSteps)
		api.GET("/tasks/stuck", h.ListStuck)
		api.GET("/business-keys/:key/runs", h.BusinessKeyHistory)

		api.GET("/alerts", h.ListAlerts)
		api.POST("/alerts/:id/ack", h.AckAlert)

		api.POST("/demo/seed", h.Simulate)
	}

	return r
}

// corsMiddleware allows the Vite dev server to call the API from the browser.
func corsMiddleware(allowOrigin string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", allowOrigin)
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
