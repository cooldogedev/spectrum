package api

type Authentication interface {
	Authenticate(token string) bool
}

type SecretBasedAuthentication struct {
	secret string
}

func NewSecretBasedAuthentication(secret string) *SecretBasedAuthentication {
	return &SecretBasedAuthentication{secret: secret}
}

func (authentication *SecretBasedAuthentication) Authenticate(token string) bool {
	return authentication.secret == token
}
