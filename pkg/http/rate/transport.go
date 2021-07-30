package rate

import (
	"fmt"
	"net/http"
)

// NewRateLimitingTransport returns a http RoundTripper which honors the specified rate limit
func NewTransport(rl AdjustableLimiter, transport http.RoundTripper) http.RoundTripper {
	return &rateLimitingTransport{
		wrappedTransport: transport,
		ratelimiter:      rl,
	}
}

// rateLimitingTransport represents a http.RoundTripper valuing the provided rate limit
type rateLimitingTransport struct {
	wrappedTransport http.RoundTripper
	ratelimiter      AdjustableLimiter
}

// RoundTrip dispatches the HTTP request to the network
func (r *rateLimitingTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	fmt.Println("Waiting for rate limiter")
	err := r.ratelimiter.Wait(request.Context()) // This is a blocking call. Honors the rate limit
	if err != nil {
		return nil, err
	}
	response, err := r.wrappedTransport.RoundTrip(request)
	if err != nil {
		return response, err
	}
	r.ratelimiter.AdjustLimit(response.Header)
	return response, err
}
