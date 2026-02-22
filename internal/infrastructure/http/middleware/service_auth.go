package middleware

import (
	"net/http"

	"github.com/apascualco/gotway/internal/infrastructure/jwt"
	"github.com/gin-gonic/gin"
)

const (
	ContextKeyServiceName = "service_name"
	HeaderServiceToken    = "X-Service-Token"
)

type ServiceAuthMiddleware struct {
	jwtService *jwt.Service
}

func NewServiceAuthMiddleware(jwtService *jwt.Service) *ServiceAuthMiddleware {
	return &ServiceAuthMiddleware{jwtService: jwtService}
}

func (m *ServiceAuthMiddleware) Authenticate() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader(HeaderServiceToken)
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid_token",
			})
			return
		}

		serviceName, err := m.jwtService.ValidateServiceToken(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid_token",
			})
			return
		}

		c.Set(ContextKeyServiceName, serviceName)
		c.Next()
	}
}
