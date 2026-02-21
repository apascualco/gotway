package domain

import (
	"fmt"
	"time"
)

const (
	IssuerAPIGateway  = "api-api"
	IssuerAuthService = "auth-service"
)

type ExternalClaims struct {
	Subject   string   `json:"sub"`
	Email     string   `json:"email"`
	Scopes    []string `json:"scopes"`
	Issuer    string   `json:"iss"`
	Audience  string   `json:"aud,omitempty"`
	IssuedAt  int64    `json:"iat"`
	ExpiresAt int64    `json:"exp"`
	NotBefore int64    `json:"nbf,omitempty"`
}

func (c *ExternalClaims) Valid() error {
	now := time.Now().Unix()

	if c.ExpiresAt != 0 && now > c.ExpiresAt {
		return ErrTokenExpired
	}

	if c.NotBefore != 0 && now < c.NotBefore {
		return ErrTokenNotYetValid
	}

	if c.Subject == "" {
		return ErrTokenInvalidSubject
	}

	return nil
}

type InternalClaims struct {
	Subject        string   `json:"sub"`
	Email          string   `json:"email,omitempty"`
	Scopes         []string `json:"scopes,omitempty"`
	Issuer         string   `json:"iss"`
	Audience       string   `json:"aud"`
	Trace          []string `json:"trace,omitempty"`
	OriginalIssuer string   `json:"original_iss,omitempty"`
	IssuedAt       int64    `json:"iat"`
	ExpiresAt      int64    `json:"exp"`
}

func (c *InternalClaims) Valid() error {
	now := time.Now().Unix()

	if c.ExpiresAt != 0 && now > c.ExpiresAt {
		return ErrTokenExpired
	}

	if c.Subject == "" {
		return ErrTokenInvalidSubject
	}

	if c.Issuer == "" {
		return ErrTokenInvalidIssuer
	}

	if c.Audience == "" {
		return ErrTokenInvalidAudience
	}

	return nil
}

func (c *InternalClaims) ValidateAudience(expectedAudience string) error {
	if c.Audience != expectedAudience {
		return fmt.Errorf("%w: expected %s, got %s", ErrTokenAudienceMismatch, expectedAudience, c.Audience)
	}
	return nil
}

func (c *InternalClaims) ValidateIssuer(allowedIssuers []string) error {
	for _, allowed := range allowedIssuers {
		if c.Issuer == allowed {
			return nil
		}
	}
	return fmt.Errorf("%w: %s not in allowed list", ErrTokenIssuerNotAllowed, c.Issuer)
}

func (c *InternalClaims) AddToTrace(service string) {
	c.Trace = append(c.Trace, service)
}

func (c *InternalClaims) HasScope(scope string) bool {
	for _, s := range c.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

func (c *InternalClaims) HasAllScopes(scopes []string) bool {
	for _, required := range scopes {
		if !c.HasScope(required) {
			return false
		}
	}
	return true
}

var (
	ErrTokenExpired          = fmt.Errorf("token has expired")
	ErrTokenNotYetValid      = fmt.Errorf("token is not yet valid")
	ErrTokenInvalidSubject   = fmt.Errorf("token has invalid subject")
	ErrTokenInvalidIssuer    = fmt.Errorf("token has invalid issuer")
	ErrTokenInvalidAudience  = fmt.Errorf("token has invalid audience")
	ErrTokenAudienceMismatch = fmt.Errorf("token audience mismatch")
	ErrTokenIssuerNotAllowed = fmt.Errorf("token issuer not allowed")
	ErrTokenInvalidSignature = fmt.Errorf("token has invalid signature")
	ErrTokenMalformed        = fmt.Errorf("token is malformed")
)
