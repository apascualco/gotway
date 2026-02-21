package handler

import (
	"errors"
	"net/http"

	"github.com/apascualco/gotway/internal/application"
	"github.com/apascualco/gotway/internal/domain"
	"github.com/gin-gonic/gin"
)

const HeaderServiceToken = "X-Service-Token"

type RegistryHandler struct {
	registry *application.Registry
}

func NewRegistryHandler(registry *application.Registry) *RegistryHandler {
	return &RegistryHandler{registry: registry}
}

func (h *RegistryHandler) Register(c *gin.Context) {
	token := c.GetHeader(HeaderServiceToken)
	if !h.registry.ValidateToken(token) {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "invalid_token",
		})
		return
	}

	var req domain.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_request",
			"message": err.Error(),
		})
		return
	}

	resp, err := h.registry.Register(&req)
	if err != nil {
		var collisionErr *domain.CollisionError
		if errors.As(err, &collisionErr) {
			c.JSON(http.StatusConflict, gin.H{
				"error":      "route_collision",
				"message":    "one or more routes are already registered",
				"collisions": collisionErr.Collisions,
			})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "registration_failed",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, resp)
}

func (h *RegistryHandler) Heartbeat(c *gin.Context) {
	token := c.GetHeader(HeaderServiceToken)
	if !h.registry.ValidateToken(token) {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "invalid_token",
		})
		return
	}

	var req domain.HeartbeatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_request",
			"message": err.Error(),
		})
		return
	}

	if err := h.registry.Heartbeat(req.InstanceID); err != nil {
		if errors.Is(err, domain.ErrInstanceNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "instance_not_found",
				"message": "the specified instance does not exist",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "heartbeat_failed",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, domain.HeartbeatResponse{Status: "ok"})
}

func (h *RegistryHandler) Deregister(c *gin.Context) {
	token := c.GetHeader(HeaderServiceToken)
	if !h.registry.ValidateToken(token) {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "invalid_token",
		})
		return
	}

	var req domain.DeregisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_request",
			"message": err.Error(),
		})
		return
	}

	if err := h.registry.Deregister(req.InstanceID); err != nil {
		if errors.Is(err, domain.ErrInstanceNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "instance_not_found",
				"message": "the specified instance does not exist",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "deregister_failed",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deregistered"})
}

func (h *RegistryHandler) ListServices(c *gin.Context) {
	token := c.GetHeader(HeaderServiceToken)
	if !h.registry.ValidateToken(token) {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "invalid_token",
		})
		return
	}

	services := h.registry.GetAllServices()
	c.JSON(http.StatusOK, gin.H{
		"services": services,
	})
}
