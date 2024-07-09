package api

import "strings"

// Authentication defines an interface for authentication methods.
type Authentication interface {
	// Authenticate checks if the provided token is valid.
	Authenticate(token string) bool
}

// SecretBasedAuthentication implements the Authentication interface
// using a secret token for authentication.
type SecretBasedAuthentication struct {
	secret string
}

// NewSecretBasedAuthentication creates a new SecretBasedAuthentication instance
// with the given secret token.
func NewSecretBasedAuthentication(secret string) *SecretBasedAuthentication {
	return &SecretBasedAuthentication{secret: secret}
}

// Authenticate ...
func (authentication *SecretBasedAuthentication) Authenticate(token string) bool {
	return strings.EqualFold(authentication.secret, token)
}
