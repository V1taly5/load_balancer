package ratelimiter

import (
	"fmt"
	"log/slog"
	"sync"
	"time"
)

type TokenBucket struct {
	Tokens     float64
	Capacity   float64
	Rate       float64
	lastUpdate time.Time
	mu         sync.Mutex
}

func NewTokenBucket(capacity, rate float64) *TokenBucket {
	// fmt.Printf("%f:%f", capacity, rate)
	return &TokenBucket{
		Tokens:     capacity,
		Capacity:   capacity,
		Rate:       rate,
		lastUpdate: time.Now(),
	}
}

// Проверяет можно ли выполнить запрос
func (t *TokenBucket) Allow() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.refresh()
	// fmt.Printf("tocken bucket Allow: tockens: %f", t.Tokens)
	// при наличии списывает токены за запрос
	if t.Tokens >= float64(1) {
		t.Tokens = t.Tokens - float64(1)
		// fmt.Printf("tocken bucket Allow: tockens: %f", t.Tokens)
		return true
	}
	return false
}

// обнавляет кол-во токенов в bucket
func (t *TokenBucket) refresh() {
	now := time.Now()
	timePassed := now.Sub(t.lastUpdate).Seconds()

	t.Tokens += timePassed * t.Rate

	if t.Tokens > t.Capacity {
		t.Tokens = t.Capacity
	}

	t.lastUpdate = now
}

func (tb *TokenBucket) SetCapacity(newCapacity float64) {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.Capacity = newCapacity
	if tb.Tokens > tb.Capacity {
		tb.Tokens = tb.Capacity
	}
}

func (tb *TokenBucket) SetRate(newRate float64) {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.Rate = newRate
}

type RateLimiter struct {
	buckets         map[string]*TokenBucket
	defaultCapacity float64
	defaultRate     float64
	mu              sync.Mutex
	log             *slog.Logger
	cleanupInterval time.Duration
	bucketTTL       time.Duration
	lastUsed        map[string]time.Time
	stopCh          chan struct{}
}

type RateLimiterOption func(*RateLimiter)

func WithCleanupInterval(interval time.Duration) RateLimiterOption {
	return func(rl *RateLimiter) {
		rl.cleanupInterval = interval
	}
}

func WithBuketTTL(ttl time.Duration) RateLimiterOption {
	return func(rl *RateLimiter) {
		rl.bucketTTL = ttl
	}
}

func NewRateLimeter(defaultCapacity, defaultRate float64, log *slog.Logger, opts ...RateLimiterOption) *RateLimiter {
	rl := &RateLimiter{
		buckets:         make(map[string]*TokenBucket),
		defaultCapacity: defaultCapacity,
		defaultRate:     defaultRate,
		log:             log,
		cleanupInterval: 10 * time.Minute,
		bucketTTL:       60 * time.Minute,
		lastUsed:        make(map[string]time.Time),
		stopCh:          make(chan struct{}),
	}

	for _, opt := range opts {
		opt(rl)
	}
	go rl.startCleanup()
	return rl
}

// Проверяет можно ли выполнить запрос для конкретного клиента
func (rl *RateLimiter) Allow(clientID string) bool {
	rl.mu.Lock()
	bucket, exist := rl.buckets[clientID]
	rl.mu.Unlock()

	if !exist {
		rl.mu.Lock()
		bucket = NewTokenBucket(rl.defaultCapacity, rl.defaultRate)
		// fmt.Printf("%f:%f", bucket.Capacity, bucket.Rate)
		rl.buckets[clientID] = bucket
		rl.lastUsed[clientID] = time.Now()
		rl.mu.Unlock()

		rl.log.Debug("created new bucket", slog.String("client_id", clientID))
	} else {
		rl.mu.Lock()
		rl.lastUsed[clientID] = time.Now()
		rl.mu.Unlock()
	}

	allowed := bucket.Allow()
	if !allowed {
		rl.log.Debug("token of bucket", slog.String("client_id", clientID), slog.Float64("Tockens", bucket.Tokens))
		rl.log.Debug("not enough tokens", slog.String("client_id", clientID))
	}
	return allowed
}

func (rl *RateLimiter) RemoveClient(clientID string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	delete(rl.buckets, clientID)
	delete(rl.lastUsed, clientID)

	rl.log.Info("removed rate limit client", "client_id", clientID)
}

func (rl *RateLimiter) startCleanup() {
	ticker := time.NewTicker(rl.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.cleanup()
		case <-rl.stopCh:
			return
		}
	}
}

func (rl *RateLimiter) StopCleanup() {
	close(rl.stopCh)
}

func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	expiredCount := 0

	for clientID, lastUsed := range rl.lastUsed {
		if now.Sub(lastUsed) > rl.bucketTTL {
			delete(rl.buckets, clientID)
			delete(rl.lastUsed, clientID)
			expiredCount++
		}
	}

	if expiredCount > 0 {
		rl.log.Info("cleaned up expired rate limit buckets", "count", expiredCount)
	}
}

func (rl *RateLimiter) SetClientLimit(clientID string, capacity, rate float64) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.buckets[clientID] = NewTokenBucket(capacity, rate)
	rl.lastUsed[clientID] = time.Now()

	rl.log.Info("set custom rate limit",
		slog.String("client_id", clientID),
		slog.Float64("capacity", capacity),
		slog.Float64("rate", rate),
	)
}

func (rl *RateLimiter) UpdateClientLimit(clientID string, capacity, rate float64) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	bucket, exists := rl.buckets[clientID]
	if !exists {
		rl.log.Debug("bucket does not exist", slog.String("client_id", clientID))
		return fmt.Errorf("client does not exist")
	}

	bucket.SetCapacity(capacity)
	bucket.SetRate(rate)

	rl.lastUsed[clientID] = time.Now()

	rl.log.Info("updated rate limit",
		slog.String("client_id", clientID),
		slog.Float64("capacity", capacity),
		slog.Float64("rate", rate),
	)

	return nil
}
