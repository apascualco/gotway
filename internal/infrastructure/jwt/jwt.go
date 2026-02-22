package jwt

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/apascualco/gotway/internal/domain"
	"github.com/apascualco/gotway/internal/infrastructure/config"
	"github.com/golang-jwt/jwt/v5"
)

type Service struct {
	publicKey      *rsa.PublicKey
	privateKey     *rsa.PrivateKey
	issuer         string
	internalTTL    time.Duration
	allowedIssuers []string
}

func NewService(cfg *config.Config) (*Service, error) {
	s := &Service{
		issuer:         cfg.JWTIssuer,
		internalTTL:    cfg.JWTInternalTTL,
		allowedIssuers: cfg.JWTAllowedIssuers,
	}

	if cfg.JWTPublicKey != "" {
		pubKey, err := parseRSAPublicKey(cfg.JWTPublicKey)
		if err != nil {
			return nil, fmt.Errorf("failed to parse public key: %w", err)
		}
		s.publicKey = pubKey
	}

	if cfg.JWTPrivateKey != "" {
		privKey, err := parseRSAPrivateKey(cfg.JWTPrivateKey)
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
		s.privateKey = privKey

		if s.publicKey == nil {
			s.publicKey = &privKey.PublicKey
		}
	}

	return s, nil
}

func NewServiceWithKeys(privateKey *rsa.PrivateKey, publicKey *rsa.PublicKey, issuer string, internalTTL time.Duration, allowedIssuers []string) *Service {
	return &Service{
		privateKey:     privateKey,
		publicKey:      publicKey,
		issuer:         issuer,
		internalTTL:    internalTTL,
		allowedIssuers: allowedIssuers,
	}
}

func (s *Service) ValidateExternalToken(tokenString string) (*domain.ExternalClaims, error) {
	if s.publicKey == nil {
		return nil, fmt.Errorf("public key not configured")
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("%w: unexpected signing method %v", domain.ErrTokenMalformed, token.Header["alg"])
		}
		return s.publicKey, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, domain.ErrTokenExpired
		}
		if errors.Is(err, jwt.ErrTokenNotValidYet) {
			return nil, domain.ErrTokenNotYetValid
		}
		if errors.Is(err, jwt.ErrTokenSignatureInvalid) {
			return nil, domain.ErrTokenInvalidSignature
		}
		return nil, fmt.Errorf("%w: %v", domain.ErrTokenMalformed, err)
	}

	if !token.Valid {
		return nil, domain.ErrTokenMalformed
	}

	mapClaims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, domain.ErrTokenMalformed
	}

	claims := &domain.ExternalClaims{
		Subject:  getStringClaim(mapClaims, "sub"),
		Email:    getStringClaim(mapClaims, "email"),
		Scopes:   getStringSliceClaim(mapClaims, "scopes"),
		Issuer:   getStringClaim(mapClaims, "iss"),
		Audience: getStringClaim(mapClaims, "aud"),
	}

	if iat, ok := mapClaims["iat"].(float64); ok {
		claims.IssuedAt = int64(iat)
	}
	if exp, ok := mapClaims["exp"].(float64); ok {
		claims.ExpiresAt = int64(exp)
	}
	if nbf, ok := mapClaims["nbf"].(float64); ok {
		claims.NotBefore = int64(nbf)
	}

	if err := claims.Valid(); err != nil {
		return nil, err
	}

	return claims, nil
}

func (s *Service) GenerateInternalToken(extClaims *domain.ExternalClaims, audience string) (string, error) {
	if s.privateKey == nil {
		return "", fmt.Errorf("private key not configured")
	}

	now := time.Now()
	claims := jwt.MapClaims{
		"sub":          extClaims.Subject,
		"email":        extClaims.Email,
		"scopes":       extClaims.Scopes,
		"iss":          s.issuer,
		"aud":          audience,
		"original_iss": extClaims.Issuer,
		"trace":        []string{s.issuer},
		"iat":          now.Unix(),
		"exp":          now.Add(s.internalTTL).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(s.privateKey)
}

func (s *Service) GenerateServiceToken(intClaims *domain.InternalClaims, audience string) (string, error) {
	if s.privateKey == nil {
		return "", fmt.Errorf("private key not configured")
	}

	now := time.Now()
	trace := append(intClaims.Trace, s.issuer)

	claims := jwt.MapClaims{
		"sub":          intClaims.Subject,
		"email":        intClaims.Email,
		"scopes":       intClaims.Scopes,
		"iss":          s.issuer,
		"aud":          audience,
		"original_iss": intClaims.OriginalIssuer,
		"trace":        trace,
		"iat":          now.Unix(),
		"exp":          now.Add(s.internalTTL).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(s.privateKey)
}

func (s *Service) ValidateInternalToken(tokenString string, expectedAudience string) (*domain.InternalClaims, error) {
	if s.publicKey == nil {
		return nil, fmt.Errorf("public key not configured")
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("%w: unexpected signing method %v", domain.ErrTokenMalformed, token.Header["alg"])
		}
		return s.publicKey, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, domain.ErrTokenExpired
		}
		if errors.Is(err, jwt.ErrTokenSignatureInvalid) {
			return nil, domain.ErrTokenInvalidSignature
		}
		return nil, fmt.Errorf("%w: %v", domain.ErrTokenMalformed, err)
	}

	if !token.Valid {
		return nil, domain.ErrTokenMalformed
	}

	mapClaims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, domain.ErrTokenMalformed
	}

	claims := &domain.InternalClaims{
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

	if err := claims.ValidateIssuer(s.allowedIssuers); err != nil {
		return nil, err
	}

	return claims, nil
}

func (s *Service) ValidateServiceToken(tokenString string) (string, error) {
	if s.publicKey == nil {
		return "", fmt.Errorf("public key not configured")
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("%w: unexpected signing method %v", domain.ErrTokenMalformed, token.Header["alg"])
		}
		return s.publicKey, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return "", domain.ErrTokenExpired
		}
		if errors.Is(err, jwt.ErrTokenSignatureInvalid) {
			return "", domain.ErrTokenInvalidSignature
		}
		return "", fmt.Errorf("%w: %v", domain.ErrTokenMalformed, err)
	}

	if !token.Valid {
		return "", domain.ErrTokenMalformed
	}

	mapClaims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", domain.ErrTokenMalformed
	}

	aud := getStringClaim(mapClaims, "aud")
	if aud != "api-gateway" {
		return "", fmt.Errorf("%w: invalid audience %q", domain.ErrTokenMalformed, aud)
	}

	sub := getStringClaim(mapClaims, "sub")
	if sub == "" {
		return "", fmt.Errorf("%w: missing subject claim", domain.ErrTokenMalformed)
	}

	return sub, nil
}

func parseRSAPublicKey(pemStr string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(normalizePEM(pemStr)))
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

func parseRSAPrivateKey(pemStr string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(normalizePEM(pemStr)))
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

var pemHeaderRe = regexp.MustCompile(`(?i)(-----BEGIN [A-Z ]+-----)`)
var pemFooterRe = regexp.MustCompile(`(?i)(-----END [A-Z ]+-----)`)

func normalizePEM(s string) string {
	if strings.Contains(s, "\n") {
		return s
	}
	s = pemHeaderRe.ReplaceAllString(s, "$1\n")
	s = pemFooterRe.ReplaceAllString(s, "\n$1")
	s = strings.TrimSpace(s)
	parts := strings.SplitN(s, "\n", 2)
	if len(parts) != 2 {
		return s
	}
	header := parts[0]
	rest := parts[1]
	parts = strings.SplitN(rest, "\n", 2)
	if len(parts) != 2 {
		return s
	}
	body := parts[0]
	footer := parts[1]
	body = strings.ReplaceAll(body, " ", "\n")
	return header + "\n" + body + "\n" + footer + "\n"
}
