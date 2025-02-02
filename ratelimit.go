package ratelimit

import (
	"context"
	"math"
	"time"
)

// Limiter allows a burst of request during the defined duration
type Limiter struct {
	maxCount uint
	count    uint
	ticker   *time.Ticker
	tokens   chan struct{}
	ctx      context.Context
	// internal
	cancelFunc context.CancelFunc
}

func (limiter *Limiter) run(ctx context.Context) {
	for {
		if limiter.count == 0 {
			<-limiter.ticker.C
			limiter.count = limiter.maxCount
		}
		select {
		case <-ctx.Done():
			// Internal Context
			limiter.ticker.Stop()
			return
		case <-limiter.ctx.Done():
			limiter.ticker.Stop()
			return
		case limiter.tokens <- struct{}{}:
			limiter.count--
		case <-limiter.ticker.C:
			limiter.count = limiter.maxCount
		}
	}
}

// Take one token from the bucket
func (rateLimiter *Limiter) Take() {
	<-rateLimiter.tokens
}

// GetLimit returns current rate limit per given duration
func (ratelimiter *Limiter) GetLimit() uint {
	return ratelimiter.maxCount
}

// SleepandReset stops timer removes all tokens and resets with new limit (used for Adaptive Ratelimiting)
func (ratelimiter *Limiter) SleepandReset(sleepTime time.Duration, newLimit uint, duration time.Duration) {
	// stop existing Limiter using internalContext
	ratelimiter.cancelFunc()
	// drain any token
	close(ratelimiter.tokens)
	<-ratelimiter.tokens
	// sleep
	time.Sleep(sleepTime)
	//reset and start
	ratelimiter.maxCount = newLimit
	ratelimiter.count = newLimit
	ratelimiter.ticker = time.NewTicker(duration)
	ratelimiter.tokens = make(chan struct{})
	ctx, cancel := context.WithCancel(context.TODO())
	ratelimiter.cancelFunc = cancel
	go ratelimiter.run(ctx)
}

// Stop the rate limiter canceling the internal context
func (ratelimiter *Limiter) Stop() {
	defer close(ratelimiter.tokens)
	if ratelimiter.cancelFunc != nil {
		ratelimiter.cancelFunc()
	}
}

// New creates a new limiter instance with the tokens amount and the interval
func New(ctx context.Context, max uint, duration time.Duration) *Limiter {
	internalctx, cancel := context.WithCancel(context.TODO())

	limiter := &Limiter{
		maxCount:   uint(max),
		count:      uint(max),
		ticker:     time.NewTicker(duration),
		tokens:     make(chan struct{}),
		ctx:        ctx,
		cancelFunc: cancel,
	}
	go limiter.run(internalctx)

	return limiter
}

// NewUnlimited create a bucket with approximated unlimited tokens
func NewUnlimited(ctx context.Context) *Limiter {
	internalctx, cancel := context.WithCancel(context.TODO())

	limiter := &Limiter{
		maxCount:   math.MaxUint,
		count:      math.MaxUint,
		ticker:     time.NewTicker(time.Millisecond),
		tokens:     make(chan struct{}),
		ctx:        ctx,
		cancelFunc: cancel,
	}
	go limiter.run(internalctx)

	return limiter
}
