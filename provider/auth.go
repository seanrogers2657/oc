package provider

import "net/http"

// APIKeyAuth implements Authenticator using the x-api-key header.
type APIKeyAuth struct {
	Key string
}

func (a *APIKeyAuth) Authenticate(req *http.Request) error {
	req.Header.Set("x-api-key", a.Key)
	return nil
}
