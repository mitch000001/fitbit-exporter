package oauth

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/mitch000001/fitbit-exporter/pkg/http/rate"
	"golang.org/x/oauth2"
)

type ClientProvider interface {
	Client(ctx context.Context) (*http.Client, error)
}

type Config struct {
	*oauth2.Config
	State               string
	RateLimiter         rate.AdjustableLimiter
	InstrumentTransport func(http.RoundTripper) http.RoundTripper
	tokenCache          TokenCache
	tokenSource         oauth2.TokenSource
	mutex               sync.Mutex
}

func (o *Config) Authorize(ctx context.Context, authCode string) error {
	tok, err := o.Exchange(ctx, authCode, oauth2.AccessTypeOffline)
	if err != nil {
		return fmt.Errorf("error exchanging token: %v", err)
	}
	o.tokenSource = o.TokenSource(ctx, tok)
	if o.tokenCache == nil {
		return nil
	}
	if err := o.tokenCache.Refresh(tok); err != nil {
		return fmt.Errorf("error refreshing token cache: %w", err)
	}
	return nil
}

func (o *Config) IsAuthorized() bool {
	tok, err := o.Token()
	if err != nil {
		return false
	}
	return tok.Valid()
}

func (o *Config) IsStateValid(state string) bool {
	return o.State == state
}

func (o *Config) Token() (*oauth2.Token, error) {
	if o.tokenSource == nil {
		return nil, fmt.Errorf("client not yet authorized")
	}
	return o.tokenSource.Token()
}

func (o *Config) Client(ctx context.Context) (*http.Client, error) {
	o.mutex.Lock()
	defer o.mutex.Unlock()
	tok, err := o.Token()
	if err != nil {
		return nil, fmt.Errorf("error getting token: %w", err)
	}
	client := o.Config.Client(ctx, tok)
	if o.RateLimiter != nil {
		transport := rate.NewTransport(
			o.RateLimiter,
			client.Transport,
		)
		client.Transport = transport
	}
	if o.InstrumentTransport != nil {
		client.Transport = o.InstrumentTransport(client.Transport)
	}
	return client, nil
}

func (o *Config) SetTokenCache(cache TokenCache) error {
	o.mutex.Lock()
	defer o.mutex.Unlock()
	o.tokenCache = cache
	tok, err := cache.Token()
	if err != nil {
		return fmt.Errorf("error getting token from cache: %w", err)
	}
	o.tokenSource = o.TokenSource(context.Background(), tok)
	return nil
}
