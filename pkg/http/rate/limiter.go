package rate

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type Limiter interface {
	Wait(context.Context) error
}

type AdjustableLimiter interface {
	Limiter
	AdjustLimit(http.Header) error
}

func NonAdjustable(rl Limiter) AdjustableLimiter {
	return &nonadjustableLimiter{
		Limiter: rl,
	}
}

type nonadjustableLimiter struct {
	Limiter
}

func (rl *nonadjustableLimiter) AdjustLimit(http.Header) error {
	return nil
}

func LimitFromHeader(header http.Header, headerKeys HeaderKeys) (Limit, error) {
	usedHeader := header.Get(headerKeys.UsedKey)
	var remaining int
	var headerLimit Limit
	limit, err := strconv.Atoi(header.Get(headerKeys.LimitKey))
	if err != nil {
		return headerLimit, fmt.Errorf("error parsing limit: %w", err)
	}
	if usedHeader != "" {
		used, err := strconv.Atoi(header.Get(headerKeys.UsedKey))
		if err != nil {
			return headerLimit, fmt.Errorf("error parsing used: %w", err)
		}
		remaining = limit - used
	} else {
		remainingParsed, err := strconv.Atoi(header.Get(headerKeys.RemainingKey))
		if err != nil {
			return headerLimit, fmt.Errorf("error parsing remaining: %w", err)
		}
		remaining = remainingParsed
	}
	resetsAfter, err := strconv.Atoi(header.Get(headerKeys.ResetsAfterKey))
	if err != nil {
		return headerLimit, fmt.Errorf("error parsing reset: %w", err)
	}
	headerLimit = Limit{
		Limit:             limit,
		Remaining:         remaining,
		ResetAfterSeconds: resetsAfter,
	}
	return headerLimit, nil
}

type Limit struct {
	Limit             int
	Remaining         int
	ResetAfterSeconds int
}

func NewFromHeader(h HeaderKeys) (*HeaderLimiter, error) {
	return &HeaderLimiter{
		headerKeys: h,
	}, nil
}

type HeaderKeys struct {
	LimitKey       string
	UsedKey        string
	RemainingKey   string
	ResetsAfterKey string
}

type HeaderLimiter struct {
	headerKeys HeaderKeys
	mutex      sync.Mutex
	limiter    *rate.Limiter
}

func (r *HeaderLimiter) Wait(ctx context.Context) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if r.limiter == nil {
		return nil
	}
	return r.limiter.Wait(ctx)
}

func (r *HeaderLimiter) AdjustLimit(header http.Header) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if r.limiter != nil {
		return nil
	}
	rl, err := r.limiterFromHeader(header)
	if err != nil {
		return fmt.Errorf("error adjusting limit by header: %w", err)
	}
	r.limiter = rl
	return nil
}

func (r *HeaderLimiter) limiterFromHeader(header http.Header) (*rate.Limiter, error) {
	limit, err := LimitFromHeader(header, r.headerKeys)
	if err != nil {
		return nil, fmt.Errorf("error getting limit from header: %v", err)
	}
	return rate.NewLimiter(rate.Every(time.Duration(limit.ResetAfterSeconds)*time.Second), limit.Remaining), nil
}

func NewLimiter(limit rate.Limit, b int) AdjustableLimiter {
	return NonAdjustable(rate.NewLimiter(limit, b))
}
