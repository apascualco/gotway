package middleware

import (
	"net/http"
	"strings"

	"github.com/apascualco/gotway/internal/domain"
	"github.com/apascualco/gotway/internal/infrastructure/jwt"
	"github.com/gin-gonic/gin"
)

const (
	ContextKeyUserID = "user_id"
	ContextKeyEmail  = "user_email"
	ContextKeyScopes = "user_scopes"
	ContextKeyClaims = "claims"

	HeaderAuthorization  = "Authorization"
	HeaderOriginalIssuer = "X-Original-Issuer"
	BearerPrefix         = "Bearer "
)

type AuthMiddleware struct {
	jwtService *jwt.Service
}

func NewAuthMiddleware(jwtService *jwt.Service) *AuthMiddleware {
	return &AuthMiddleware{
		jwtService: jwtService,
	}
}

func (a *AuthMiddleware) Authenticate(route *domain.RouteEntry, serviceName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if route.Route.Public {
			c.Next()
			return
		}

		tokenString := extractBearerToken(c)
		if tokenString == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "missing authorization token",
			})
			return
		}

		claims, err := a.jwtService.ValidateExternalToken(tokenString)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "invalid token",
				"details": err.Error(),
			})
			return
		}

		if len(route.Route.Scopes) > 0 {
			if !hasAllScopes(claims.Scopes, route.Route.Scopes) {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"error":    "forbidden",
					"message":  "insufficient scopes",
					"required": route.Route.Scopes,
					"provided": claims.Scopes,
				})
				return
			}
		}

		internalToken, err := a.jwtService.GenerateInternalToken(claims, serviceName)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error":   "internal_error",
				"message": "failed to generate internal token",
			})
			return
		}

		c.Request.Header.Set(HeaderAuthorization, BearerPrefix+internalToken)
		c.Request.Header.Set(HeaderOriginalIssuer, claims.Issuer)

		c.Next()
	}
}

func (a *AuthMiddleware) AuthenticateRequest(c *gin.Context, route *domain.RouteEntry, serviceName string) bool {
	if route.Route.Public {
		return true
	}

	tokenString := extractBearerToken(c)
	if tokenString == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "unauthorized",
			"message": "missing authorization token",
		})
		return false
	}

	claims, err := a.jwtService.ValidateExternalToken(tokenString)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "unauthorized",
			"message": "invalid token",
			"details": err.Error(),
		})
		return false
	}

	if len(route.Route.Scopes) > 0 {
		if !hasAllScopes(claims.Scopes, route.Route.Scopes) {
			c.JSON(http.StatusForbidden, gin.H{
				"error":    "forbidden",
				"message":  "insufficient scopes",
				"required": route.Route.Scopes,
				"provided": claims.Scopes,
			})
			return false
		}
	}

	internalToken, err := a.jwtService.GenerateInternalToken(claims, serviceName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "failed to generate internal token",
		})
		return false
	}

	c.Request.Header.Set(HeaderAuthorization, BearerPrefix+internalToken)
	c.Request.Header.Set(HeaderOriginalIssuer, claims.Issuer)

	return true
}

func extractBearerToken(c *gin.Context) string {
	auth := c.GetHeader(HeaderAuthorization)
	if auth == "" {
		return ""
	}
	if !strings.HasPrefix(auth, BearerPrefix) {
		return ""
	}
	return strings.TrimPrefix(auth, BearerPrefix)
}

func hasAllScopes(provided, required []string) bool {
	scopeSet := make(map[string]struct{}, len(provided))
	for _, s := range provided {
		scopeSet[s] = struct{}{}
	}
	for _, r := range required {
		if _, ok := scopeSet[r]; !ok {
			return false
		}
	}
	return true
}
