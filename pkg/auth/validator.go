package auth

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
)

// Validator validates internal JWT tokens from the API Gateway.
type Validator struct {
	publicKey      *rsa.PublicKey
	allowedIssuers []string
}

// ValidatorOption is a functional option for configuring the Validator.
type ValidatorOption func(*Validator) error

// WithPublicKey sets the RSA public key from a PEM-encoded string.
func WithPublicKey(pemStr string) ValidatorOption {
	return func(v *Validator) error {
		key, err := ParseRSAPublicKey(pemStr)
		if err != nil {
			return fmt.Errorf("failed to parse public key: %w", err)
		}
		v.publicKey = key
		return nil
	}
}

// WithPublicKeyRSA sets the RSA public key directly.
func WithPublicKeyRSA(key *rsa.PublicKey) ValidatorOption {
	return func(v *Validator) error {
		v.publicKey = key
		return nil
	}
}

// WithAllowedIssuers sets the list of allowed token issuers.
func WithAllowedIssuers(issuers []string) ValidatorOption {
	return func(v *Validator) error {
		v.allowedIssuers = issuers
		return nil
	}
}

// NewValidator creates a new Validator with the given options.
func NewValidator(opts ...ValidatorOption) (*Validator, error) {
	v := &Validator{}

	for _, opt := range opts {
		if err := opt(v); err != nil {
			return nil, err
		}
	}

	if v.publicKey == nil {
		return nil, ErrPublicKeyNotSet
	}

	return v, nil
}

// ValidateInternalToken validates an internal JWT token and returns the claims.
// It checks the signature, expiration, audience, and issuer.
func (v *Validator) ValidateInternalToken(tokenString string, expectedAudience string) (*Claims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("%w: unexpected signing method %v", ErrTokenMalformed, token.Header["alg"])
		}
		return v.publicKey, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		if errors.Is(err, jwt.ErrTokenSignatureInvalid) {
			return nil, ErrTokenInvalidSignature
		}
		return nil, fmt.Errorf("%w: %v", ErrTokenMalformed, err)
	}

	if !token.Valid {
		return nil, ErrTokenMalformed
	}

	mapClaims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, ErrTokenMalformed
	}

	claims := &Claims{
		Subject:        getStringClaim(mapClaims, "sub"),
		Email:          getStringClaim(mapClaims, "email"),
		Scopes:         getStringSliceClaim(mapClaims, "scopes"),
		Issuer:         getStringClaim(mapClaims, "iss"),
		Audience:       getStringClaim(mapClaims, "aud"),
		OriginalIssuer: getStringClaim(mapClaims, "original_iss"),
		Trace:          getStringSliceClaim(mapClaims, "trace"),
	}

	if iat, ok := mapClaims["iat"].(float64); ok {
		claims.IssuedAt = int64(iat)
	}
	if exp, ok := mapClaims["exp"].(float64); ok {
		claims.ExpiresAt = int64(exp)
	}

	if err := claims.Valid(); err != nil {
		return nil, err
	}

	if err := claims.ValidateAudience(expectedAudience); err != nil {
		return nil, err
	}

	if len(v.allowedIssuers) > 0 {
		if err := claims.ValidateIssuer(v.allowedIssuers); err != nil {
			return nil, err
		}
	}

	return claims, nil
}

// ParseRSAPublicKey parses a PEM-encoded RSA public key.
func ParseRSAPublicKey(pemStr string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return x509.ParsePKCS1PublicKey(block.Bytes)
	}

	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not an RSA public key")
	}

	return rsaPub, nil
}

// ParseRSAPrivateKey parses a PEM-encoded RSA private key.
func ParseRSAPrivateKey(pemStr string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err == nil {
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("not an RSA private key")
		}
		return rsaKey, nil
	}

	return x509.ParsePKCS1PrivateKey(block.Bytes)
}

func getStringClaim(claims jwt.MapClaims, key string) string {
	if val, ok := claims[key].(string); ok {
		return val
	}
	return ""
}

func getStringSliceClaim(claims jwt.MapClaims, key string) []string {
	if val, ok := claims[key].([]interface{}); ok {
		result := make([]string, 0, len(val))
		for _, v := range val {
			if s, ok := v.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return nil
}
