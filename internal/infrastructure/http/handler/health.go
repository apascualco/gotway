package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type HealthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
	Uptime  string `json:"uptime"`
}

func HealthHandler(startTime time.Time, version string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, HealthResponse{
			Status:  "healthy",
			Version: version,
			Uptime:  time.Since(startTime).Truncate(time.Second).String(),
		})
	}
}

type ReadyResponse struct {
	Status string `json:"status"`
}

func ReadyHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, ReadyResponse{
			Status: "ready",
		})
	}
}
