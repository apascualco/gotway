package auth

import (
	"fmt"
	"time"
)

// Claims represents the internal JWT claims used for service-to-service communication.
// These are the claims that the API Gateway generates after validating an external token.
type Claims struct {
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

// Valid checks if the claims are valid (not expired, has required fields).
func (c *Claims) Valid() error {
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

// ValidateAudience checks if the token audience matches the expected audience.
func (c *Claims) ValidateAudience(expectedAudience string) error {
	if c.Audience != expectedAudience {
		return fmt.Errorf("%w: expected %s, got %s", ErrTokenAudienceMismatch, expectedAudience, c.Audience)
	}
	return nil
}

// ValidateIssuer checks if the token issuer is in the allowed list.
func (c *Claims) ValidateIssuer(allowedIssuers []string) error {
	for _, allowed := range allowedIssuers {
		if c.Issuer == allowed {
			return nil
		}
	}
	return fmt.Errorf("%w: %s not in allowed list", ErrTokenIssuerNotAllowed, c.Issuer)
}

// HasScope checks if the claims contain a specific scope.
func (c *Claims) HasScope(scope string) bool {
	for _, s := range c.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

// HasAllScopes checks if the claims contain all required scopes.
func (c *Claims) HasAllScopes(scopes []string) bool {
	for _, required := range scopes {
		if !c.HasScope(required) {
			return false
		}
	}
	return true
}
