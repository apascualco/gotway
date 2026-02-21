package config

import (
	"time"

	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	Port                int           `envconfig:"PORT" default:"8080"`
	Env                 string        `envconfig:"ENV" default:"development"`
	LogLevel            string        `envconfig:"LOG_LEVEL" default:"debug"`
	CORSAllowedOrigins  []string      `envconfig:"CORS_ALLOWED_ORIGINS" default:"*"`
	CORSAllowedMethods  []string      `envconfig:"CORS_ALLOWED_METHODS" default:"GET,POST,PUT,DELETE,OPTIONS"`
	CORSAllowedHeaders  []string      `envconfig:"CORS_ALLOWED_HEADERS" default:"Origin,Content-Type,Accept,Authorization,X-Request-ID"`
	ServiceToken        string        `envconfig:"SERVICE_TOKEN" required:"true"`
	HeartbeatTTL        time.Duration `envconfig:"HEARTBEAT_TTL" default:"30s"`
	HealthCheckInterval time.Duration `envconfig:"HEALTH_CHECK_INTERVAL" default:"10s"`

	JWTPublicKey      string        `envconfig:"JWT_PUBLIC_KEY"`
	JWTPrivateKey     string        `envconfig:"JWT_PRIVATE_KEY"`
	JWTIssuer         string        `envconfig:"JWT_ISSUER" default:"api-api"`
	JWTInternalTTL    time.Duration `envconfig:"JWT_INTERNAL_TTL" default:"5m"`
	JWTAllowedIssuers []string      `envconfig:"JWT_ALLOWED_ISSUERS" default:"auth-service"`

	RedisURL           string `envconfig:"REDIS_URL" default:""`
	RateLimitEnabled   bool   `envconfig:"RATE_LIMIT_ENABLED" default:"false"`
	RateLimitGlobalRPM int    `envconfig:"RATE_LIMIT_GLOBAL_RPM" default:"10000"`
	RateLimitUserRPM   int    `envconfig:"RATE_LIMIT_USER_RPM" default:"100"`
	RateLimitIPRPM     int    `envconfig:"RATE_LIMIT_IP_RPM" default:"60"`

	Version, Commit, BuildDate string
}

func Load(version, commit, buildDate string) (*Config, error) {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, err
	}
	cfg.Version, cfg.Commit, cfg.BuildDate = version, commit, buildDate
	return &cfg, nil
}
