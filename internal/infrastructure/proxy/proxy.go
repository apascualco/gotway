package proxy

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/apascualco/gotway/internal/application"
	"github.com/apascualco/gotway/internal/infrastructure/http/middleware"
	"github.com/gin-gonic/gin"
)

var hopByHopHeaders = []string{
	"Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Te",
	"Trailers",
	"Transfer-Encoding",
	"Upgrade",
}

type ProxyHandler struct {
	registry       *application.Registry
	loadBalancer   application.LoadBalancer
	authMiddleware *middleware.AuthMiddleware
}

func NewProxyHandler(registry *application.Registry, lb application.LoadBalancer, auth *middleware.AuthMiddleware) *ProxyHandler {
	return &ProxyHandler{
		registry:       registry,
		loadBalancer:   lb,
		authMiddleware: auth,
	}
}

func (p *ProxyHandler) Handle(c *gin.Context) {
	match := MatchRoute(p.registry, c.Request.Method, c.Request.URL.Path)
	if match == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "route_not_found",
			"message": "no service registered for this route",
		})
		return
	}

	if p.authMiddleware != nil {
		if !p.authMiddleware.AuthenticateRequest(c, match.Entry, match.Entry.ServiceName) {
			return
		}
	}

	instances := p.registry.GetHealthyInstances(match.Entry.ServiceName)
	if len(instances) == 0 {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "service_unavailable",
			"message": "no healthy instances available",
			"service": match.Entry.ServiceName,
		})
		return
	}

	instance := p.loadBalancer.Select(instances)
	if instance == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "service_unavailable",
			"message": "failed to select instance",
		})
		return
	}

	targetURL := &url.URL{
		Scheme: "http",
		Host:   instance.Address(),
	}

	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = targetURL.Scheme
			req.URL.Host = targetURL.Host
			req.Host = targetURL.Host

			req.URL.Path = c.Request.URL.Path

			if c.Request.URL.RawQuery != "" {
				req.URL.RawQuery = c.Request.URL.RawQuery
			}

			for _, h := range hopByHopHeaders {
				req.Header.Del(h)
			}

			clientIP := c.ClientIP()
			if prior := req.Header.Get("X-Forwarded-For"); prior != "" {
				clientIP = prior + ", " + clientIP
			}
			req.Header.Set("X-Forwarded-For", clientIP)

			if c.Request.Host != "" {
				req.Header.Set("X-Forwarded-Host", c.Request.Host)
			}

			proto := "http"
			if c.Request.TLS != nil {
				proto = "https"
			}
			if forwardedProto := c.GetHeader("X-Forwarded-Proto"); forwardedProto != "" {
				proto = forwardedProto
			}
			req.Header.Set("X-Forwarded-Proto", proto)

			if requestID := c.GetHeader("X-Request-ID"); requestID != "" {
				req.Header.Set("X-Request-ID", requestID)
			}

			req.Header.Set("X-Forwarded-Service", match.Entry.ServiceName)
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			c.JSON(http.StatusBadGateway, gin.H{
				"error":   "upstream_error",
				"message": fmt.Sprintf("failed to connect to upstream: %v", err),
			})
		},
	}

	proxy.ServeHTTP(c.Writer, c.Request)
}
