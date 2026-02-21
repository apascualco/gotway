package auth

import (
	"crypto/rsa"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Generator generates internal JWT tokens for service-to-service communication.
type Generator struct {
	privateKey  *rsa.PrivateKey
	issuer      string
	defaultTTL  time.Duration
}

// GeneratorOption is a functional option for configuring the Generator.
type GeneratorOption func(*Generator) error

// WithPrivateKey sets the RSA private key from a PEM-encoded string.
func WithPrivateKey(pemStr string) GeneratorOption {
	return func(g *Generator) error {
		key, err := ParseRSAPrivateKey(pemStr)
		if err != nil {
			return fmt.Errorf("failed to parse private key: %w", err)
		}
		g.privateKey = key
		return nil
	}
}

// WithPrivateKeyRSA sets the RSA private key directly.
func WithPrivateKeyRSA(key *rsa.PrivateKey) GeneratorOption {
	return func(g *Generator) error {
		g.privateKey = key
		return nil
	}
}

// WithIssuer sets the issuer claim for generated tokens.
func WithIssuer(issuer string) GeneratorOption {
	return func(g *Generator) error {
		g.issuer = issuer
		return nil
	}
}

// WithDefaultTTL sets the default time-to-live for generated tokens.
func WithDefaultTTL(ttl time.Duration) GeneratorOption {
	return func(g *Generator) error {
		g.defaultTTL = ttl
		return nil
	}
}

// NewGenerator creates a new Generator with the given options.
func NewGenerator(opts ...GeneratorOption) (*Generator, error) {
	g := &Generator{
		defaultTTL: 5 * time.Minute,
	}

	for _, opt := range opts {
		if err := opt(g); err != nil {
			return nil, err
		}
	}

	if g.privateKey == nil {
		return nil, ErrPrivateKeyNotSet
	}

	if g.issuer == "" {
		return nil, fmt.Errorf("issuer is required")
	}

	return g, nil
}

// GenerateServiceToken generates a new internal JWT token for calling another service.
// It takes the incoming claims, appends the current service to the trace chain,
// and generates a new token with the specified audience.
func (g *Generator) GenerateServiceToken(incomingClaims *Claims, targetAudience string) (string, error) {
	return g.GenerateServiceTokenWithTTL(incomingClaims, targetAudience, g.defaultTTL)
}

// GenerateServiceTokenWithTTL generates a new internal JWT token with a custom TTL.
func (g *Generator) GenerateServiceTokenWithTTL(incomingClaims *Claims, targetAudience string, ttl time.Duration) (string, error) {
	now := time.Now()
	trace := append(incomingClaims.Trace, g.issuer)

	claims := jwt.MapClaims{
		"sub":          incomingClaims.Subject,
		"email":        incomingClaims.Email,
		"scopes":       incomingClaims.Scopes,
		"iss":          g.issuer,
		"aud":          targetAudience,
		"original_iss": incomingClaims.OriginalIssuer,
		"trace":        trace,
		"iat":          now.Unix(),
		"exp":          now.Add(ttl).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(g.privateKey)
}

// Client combines both Validator and Generator for services that need both capabilities.
type Client struct {
	*Validator
	*Generator
}

// ClientOption is a functional option for configuring the Client.
type ClientOption func(*Client) error

// NewClient creates a new auth Client that can both validate and generate tokens.
func NewClient(validatorOpts []ValidatorOption, generatorOpts []GeneratorOption) (*Client, error) {
	validator, err := NewValidator(validatorOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create validator: %w", err)
	}

	generator, err := NewGenerator(generatorOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create generator: %w", err)
	}

	return &Client{
		Validator: validator,
		Generator: generator,
	}, nil
}
