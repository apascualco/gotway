package application

import "crypto/subtle"

func (r *Registry) ValidateToken(token string) bool {
	if r.config.ServiceToken == "" || token == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(r.config.ServiceToken), []byte(token)) == 1
}
