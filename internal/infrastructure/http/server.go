package http

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/apascualco/gotway/internal/application"
	"github.com/apascualco/gotway/internal/infrastructure/config"
	"github.com/apascualco/gotway/internal/infrastructure/http/handler"
	"github.com/apascualco/gotway/internal/infrastructure/http/middleware"
	"github.com/apascualco/gotway/internal/infrastructure/jwt"
	"github.com/apascualco/gotway/internal/infrastructure/proxy"
	"github.com/apascualco/gotway/internal/infrastructure/ratelimit"
	"github.com/apascualco/gotway/internal/infrastructure/redis"
	"github.com/gin-gonic/gin"
)

type Server struct {
	router         *gin.Engine
	config         *config.Config
	httpServer     *http.Server
	startTime      time.Time
	registry       *application.Registry
	jwtService     *jwt.Service
	authMiddleware *middleware.AuthMiddleware
	redisClient    *redis.Client
	rateLimiter    ratelimit.RateLimiter
}

func NewServer(cfg *config.Config) (*Server, error) {
	slog.Debug("new application registry",
		slog.Duration("heartbeat_ttl", cfg.HeartbeatTTL),
		slog.Duration("health_check_interval", cfg.HealthCheckInterval),
	)
	registry := application.NewRegistry(application.RegistryConfig{
		HeartbeatTTL:        cfg.HeartbeatTTL,
		HealthCheckInterval: cfg.HealthCheckInterval,
	})

	var jwtService *jwt.Service
	var authMiddleware *middleware.AuthMiddleware

	if cfg.JWTPublicKey != "" || cfg.JWTPrivateKey != "" {
		var err error
		jwtService, err = jwt.NewService(cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create JWT service: %w", err)
		}
		authMiddleware = middleware.NewAuthMiddleware(jwtService)
		slog.Info("jwt authentication enabled")
	} else {
		slog.Warn("JWT keys not configured, authentication disabled")
	}

	var redisClient *redis.Client
	var rateLimiter ratelimit.RateLimiter

	if cfg.RateLimitEnabled {
		if cfg.RedisURL != "" {
			var err error
			redisClient, err = redis.NewClient(cfg.RedisURL)
			if err != nil {
				return nil, fmt.Errorf("failed to create redis client: %w", err)
			}
			rateLimiter = ratelimit.NewLimiter(redisClient.Client)
			slog.Info("rate limiting enabled with Redis")
		} else {
			rateLimiter = ratelimit.NewInMemoryLimiter()
			slog.Warn("rate limiting enabled with in-memory limiter (not recommended for production)")
		}
	} else {
		slog.Debug("rate limiting disabled")
	}

	s := &Server{
		config:         cfg,
		startTime:      time.Now(),
		registry:       registry,
		jwtService:     jwtService,
		authMiddleware: authMiddleware,
		redisClient:    redisClient,
		rateLimiter:    rateLimiter,
	}
	s.setupRouter()
	return s, nil
}

func (s *Server) setupRouter() {
	if s.config.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	s.router = gin.New()
	s.router.Use(middleware.Recovery())
	s.router.Use(middleware.Logger())
	s.router.Use(middleware.RequestID())
	s.router.Use(middleware.CORS(middleware.CORSConfig{
		AllowedOrigins: s.config.CORSAllowedOrigins,
		AllowedMethods: s.config.CORSAllowedMethods,
		AllowedHeaders: s.config.CORSAllowedHeaders,
	}))

	if s.rateLimiter != nil {
		s.router.Use(middleware.RateLimitMiddleware(s.rateLimiter, s.config))
	}

	s.router.GET("/health", handler.HealthHandler(s.startTime, s.config.Version))
	s.router.GET("/ready", handler.ReadyHandler())

	s.setupRegistryRoutes()
	s.setupProxyRoute()
}

func (s *Server) setupRegistryRoutes() {
	registryHandler := handler.NewRegistryHandler(s.registry)

	internal := s.router.Group("/internal/registry")
	if s.jwtService != nil {
		serviceAuth := middleware.NewServiceAuthMiddleware(s.jwtService)
		internal.Use(serviceAuth.Authenticate())
	}
	{
		internal.POST("/register", registryHandler.Register)
		internal.POST("/heartbeat", registryHandler.Heartbeat)
		internal.POST("/deregister", registryHandler.Deregister)
		internal.GET("/services", registryHandler.ListServices)
	}
}

func (s *Server) setupProxyRoute() {
	loadBalancer := application.NewRoundRobinBalancer()
	proxyHandler := proxy.NewProxyHandler(s.registry, loadBalancer, s.authMiddleware)
	s.router.NoRoute(proxyHandler.Handle)
}

func (s *Server) Run() error {
	s.registry.Start()

	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.config.Port),
		Handler: s.router,
	}
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.registry.Stop()
	if s.redisClient != nil {
		err := s.redisClient.Close()
		if err != nil {
			return err
		}
	}
	return s.httpServer.Shutdown(ctx)
}
