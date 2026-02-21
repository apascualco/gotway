package auth

import "errors"

var (
	ErrTokenExpired          = errors.New("token has expired")
	ErrTokenNotYetValid      = errors.New("token is not yet valid")
	ErrTokenInvalidSubject   = errors.New("token has invalid subject")
	ErrTokenInvalidIssuer    = errors.New("token has invalid issuer")
	ErrTokenInvalidAudience  = errors.New("token has invalid audience")
	ErrTokenAudienceMismatch = errors.New("token audience mismatch")
	ErrTokenIssuerNotAllowed = errors.New("token issuer not allowed")
	ErrTokenInvalidSignature = errors.New("token has invalid signature")
	ErrTokenMalformed        = errors.New("token is malformed")
	ErrPublicKeyNotSet       = errors.New("public key not configured")
	ErrPrivateKeyNotSet      = errors.New("private key not configured")
)
