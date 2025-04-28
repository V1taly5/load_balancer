package ratelimiter

import (
	"sync"
	"time"
)

type TokenBucket struct {
	Tokens     float64   `json:"tokens"`
	Capacity   float64   `json:"capacity"`
	Rate       float64   `json:"rate"`
	LastUpdate time.Time `json:"last_update"`
	mu         sync.Mutex
}

func NewTokenBucket(capacity, rate float64) *TokenBucket {
	// fmt.Printf("%f:%f", capacity, rate)
	return &TokenBucket{
		Tokens:     capacity,
		Capacity:   capacity,
		Rate:       rate,
		LastUpdate: time.Now(),
	}
}

func (tb *TokenBucket) GetCapacity() float64 {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	return tb.Capacity
}

func (tb *TokenBucket) GetRate() float64 {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	return tb.Rate
}

// Проверяет можно ли выполнить запрос
func (t *TokenBucket) Allow(now time.Time) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	// обновлять кол-во токенов в момент проверки
	// t.refresh(now)

	// при наличии списывает токены за запрос
	if t.Tokens >= float64(1) {
		t.Tokens = t.Tokens - float64(1)
		return true
	}
	return false
}

// Обнавляет кол-во токенов в bucket без блокировок
func (t *TokenBucket) refresh(now time.Time) {
	// now := time.Now()
	timePassed := now.Sub(t.LastUpdate).Seconds()

	t.Tokens += timePassed * t.Rate

	if t.Tokens > t.Capacity {
		t.Tokens = t.Capacity
	}

	t.LastUpdate = now
}
