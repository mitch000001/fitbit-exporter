package oauth

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"golang.org/x/oauth2"
)

type TokenCache interface {
	oauth2.TokenSource
	Refresh(*oauth2.Token) error
}

func NewJSONFileTokenCacheFromToken(filePath string, tok *oauth2.Token) (TokenCache, error) {
	tokenCache := &jsonFileTokenCache{
		filePath: filePath,
	}
	if err := tokenCache.Refresh(tok); err != nil {
		return nil, fmt.Errorf("error refreshing token cache: %w", err)
	}
	return tokenCache, nil
}

func NewJSONFileTokenCache(filePath string) (TokenCache, error) {
	cache := &jsonFileTokenCache{
		filePath: filePath,
	}
	if err := cache.load(); err != nil {
		return nil, fmt.Errorf("error loading token: %w", err)
	}
	return cache, nil
}

type jsonFileTokenCache struct {
	filePath string
	token    *oauth2.Token
	mutex    sync.Mutex
}

func (t *jsonFileTokenCache) Token() (*oauth2.Token, error) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	return t.token, nil
}

func (t *jsonFileTokenCache) Refresh(tok *oauth2.Token) error {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	file, err := os.Create(t.filePath)
	if err != nil {
		return fmt.Errorf("error creating token file: %w", err)
	}
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(tok); err != nil {
		return fmt.Errorf("error marshaling json token: %w", err)
	}
	t.token = tok
	return nil
}

func (t *jsonFileTokenCache) load() error {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	if _, err := os.Stat(t.filePath); os.IsNotExist(err) {
		return nil
	}
	file, err := os.Open(t.filePath)
	if err != nil {
		return fmt.Errorf("error reading token file: %w", err)
	}
	var tok oauth2.Token
	if err := json.NewDecoder(file).Decode(&tok); err != nil {
		return fmt.Errorf("error unmarshaling json token: %w", err)
	}
	t.token = &tok
	return nil
}
