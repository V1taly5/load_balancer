package ratelimiter

import (
	"fmt"
	"loadbalancer/internal/lib/sl"
	"loadbalancer/internal/storage"
	"log/slog"
	"sync"
	"time"
)

type Storage interface {
	Save(clientID string, state *storage.BucketState) error
	LoadAll() (map[string]*storage.BucketState, error)
	Delete(clientID string) error
}

type RateLimiter struct {
	buckets           map[string]*TokenBucket
	defaultCapacity   float64
	defaultRate       float64
	mu                sync.Mutex
	log               *slog.Logger
	cleanupInterval   time.Duration
	replenishInterval time.Duration
	bucketTTL         time.Duration
	lastUsed          map[string]time.Time
	storage           Storage
	stopCh            chan struct{}
	wg                sync.WaitGroup
}

type RateLimiterOption func(*RateLimiter)

func WithCleanupInterval(interval time.Duration) RateLimiterOption {
	return func(rl *RateLimiter) {
		rl.cleanupInterval = interval
	}
}

func WithBucketTTL(ttl time.Duration) RateLimiterOption {
	return func(rl *RateLimiter) {
		rl.bucketTTL = ttl
	}
}

func WithReplenishInterval(interval time.Duration) RateLimiterOption {
	return func(rl *RateLimiter) {
		rl.replenishInterval = interval
	}
}

func NewRateLimiter(defaultCapacity, defaultRate float64, storage Storage, log *slog.Logger, opts ...RateLimiterOption) *RateLimiter {
	rl := &RateLimiter{
		buckets:           make(map[string]*TokenBucket),
		defaultCapacity:   defaultCapacity,
		defaultRate:       defaultRate,
		log:               log,
		cleanupInterval:   10 * time.Minute,
		replenishInterval: 30 * time.Second,
		bucketTTL:         60 * time.Minute,
		lastUsed:          make(map[string]time.Time),
		storage:           storage,
		stopCh:            make(chan struct{}),
	}

	for _, opt := range opts {
		opt(rl)
	}

	rl.loadFromStorage()
	go rl.startReplenish()
	// go rl.startCleanup()

	return rl
}

func (rl *RateLimiter) GetClient(clientID string) (capacity, rate float64, exists bool) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	bucket, exists := rl.buckets[clientID]
	if !exists {
		return 0, 0, false
	}

	return bucket.GetCapacity(), bucket.GetRate(), true
}

// Останавливает все фоновые процессы
func (rl *RateLimiter) Stop() {
	fmt.Println("RateLimiter compl")
	close(rl.stopCh)
	rl.wg.Wait()
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// сохраняем всех клиентов после остановки
	for clientID, bucket := range rl.buckets {
		state := &storage.BucketState{
			Tokens:     bucket.Tokens,
			Capacity:   bucket.Capacity,
			Rate:       bucket.Rate,
			LastUpdate: bucket.LastUpdate,
		}
		if err := rl.storage.Save(clientID, state); err != nil {
			rl.log.Error("failed to save bucket on Stop",
				slog.String("client_id", clientID),
				sl.Err(err),
			)
		} else {
			rl.log.Debug("saved bucket on Stop", slog.String("client_id", clientID))
		}
	}

}

func (rl *RateLimiter) loadFromStorage() {
	clients, err := rl.storage.LoadAll()
	if err != nil {
		rl.log.Error("failed to load clients", sl.Err(err))
		return
	}

	now := time.Now()
	for clientID, state := range clients {
		bucket := NewTokenBucket(state.Capacity, state.Rate)
		bucket.Tokens = state.Tokens
		bucket.LastUpdate = state.LastUpdate
		bucket.mu.Lock()
		bucket.refresh(now)
		bucket.mu.Unlock()
		rl.buckets[clientID] = bucket
		rl.lastUsed[clientID] = state.LastUpdate
	}
}

// Проверяет можно ли выполнить запрос для конкретного клиента
func (rl *RateLimiter) Allow(clientID string) bool {
	rl.mu.Lock()
	bucket, exist := rl.buckets[clientID]
	// defer rl.mu.Unlock()

	if !exist {
		var err error
		bucket, err = rl.createBucket(clientID, rl.defaultCapacity, rl.defaultRate)
		rl.mu.Unlock()
		if err != nil {
			return false
		}

		rl.log.Debug("created new bucket", slog.String("client_id", clientID))
	} else {

		rl.lastUsed[clientID] = time.Now()
		rl.mu.Unlock()
	}

	allowed := bucket.Allow(time.Now())
	if !allowed {
		rl.log.Debug("not enough tokens",
			slog.String("client_id", clientID),
			slog.Float64("tokens", bucket.Tokens),
		)
	}
	return allowed
}

func (rl *RateLimiter) startReplenish() {
	rl.wg.Add(1)
	defer rl.wg.Done()
	ticker := time.NewTicker(rl.replenishInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.replenishBuckets()
		case <-rl.stopCh:
			return
		}
	}
}

func (rl *RateLimiter) replenishBuckets() {
	rl.mu.Lock()
	bucketsToRefresh := make([]*TokenBucket, 0, len(rl.buckets))
	for _, bucket := range rl.buckets {
		bucketsToRefresh = append(bucketsToRefresh, bucket)
	}
	rl.mu.Unlock()

	now := time.Now()
	for _, bucket := range bucketsToRefresh {
		bucket.mu.Lock()
		bucket.refresh(now)
		bucket.mu.Unlock()
	}
}

func (rl *RateLimiter) RemoveClient(clientID string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if err := rl.storage.Delete(clientID); err != nil {
		rl.log.Error("failed to delete client", slog.String("client_id", clientID), sl.Err(err))
		return
	}
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

func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	expiredCount := 0

	for clientID, lastUsed := range rl.lastUsed {
		if now.Sub(lastUsed) > rl.bucketTTL {
			bucket, exist := rl.buckets[clientID]
			if !exist {
				continue
			}

			bucketState := storage.BucketState{
				Tokens:     bucket.Tokens,
				Capacity:   bucket.Capacity,
				Rate:       bucket.Rate,
				LastUpdate: bucket.LastUpdate,
			}

			err := rl.storage.Save(clientID, &bucketState)
			if err != nil {
				rl.log.Debug("failed to save bucket state before deleting",
					slog.String("client_id", clientID),
					sl.Err(err),
				)
				continue
			}
			delete(rl.buckets, clientID)
			delete(rl.lastUsed, clientID)
			expiredCount++
		}
	}

	if expiredCount > 0 {
		rl.log.Info("cleaned up expired rate limit buckets", "count", expiredCount)
	}
}

func (rl *RateLimiter) createBucket(clientID string, capacity, rate float64) (*TokenBucket, error) {
	bucket := NewTokenBucket(capacity, rate)

	state := &storage.BucketState{
		Tokens:     bucket.Tokens,
		Capacity:   bucket.Capacity,
		Rate:       bucket.Rate,
		LastUpdate: bucket.LastUpdate,
	}

	if err := rl.storage.Save(clientID, state); err != nil {
		return nil, err
	}

	rl.buckets[clientID] = bucket
	rl.lastUsed[clientID] = time.Now()

	return bucket, nil
}

func (rl *RateLimiter) SetClientLimit(clientID string, capacity, rate float64) (*TokenBucket, error) {

	// rl.buckets[clientID] = NewTokenBucket(capacity, rate)
	bucket := NewTokenBucket(capacity, rate)

	state := &storage.BucketState{
		Tokens:     bucket.Tokens,
		Capacity:   bucket.Capacity,
		Rate:       bucket.Rate,
		LastUpdate: bucket.LastUpdate,
	}

	if err := rl.storage.Save(clientID, state); err != nil {
		rl.log.Error("failed to set clietn limit", slog.String("client_id", clientID), sl.Err(err))
		return nil, err
	}
	rl.mu.Lock()
	rl.buckets[clientID] = bucket
	rl.lastUsed[clientID] = time.Now()
	rl.mu.Unlock()

	rl.log.Info("set custom rate limit",
		slog.String("client_id", clientID),
		slog.Float64("capacity", capacity),
		slog.Float64("rate", rate),
	)
	return bucket, nil
}

// проверяем существование обнавляемого клиента
// существует -> блокируем mutex
// запоминаем время, обнавляем токены для этого времени
// изменяеи информацию, сначала записываем в файл, после в map
// эта функуия не работает правильно, если cleanup уже удалил клиента из локальной map
func (rl *RateLimiter) UpdateClientLimit(clientID string, capacity, rate float64) error {
	rl.mu.Lock()
	bucket, exists := rl.buckets[clientID]
	if !exists {
		rl.mu.Unlock()
		rl.log.Debug("bucket does not exist", slog.String("client_id", clientID))
		return fmt.Errorf("client does not exist")
	}
	now := time.Now()
	rl.lastUsed[clientID] = now
	rl.mu.Unlock()

	bucket.mu.Lock()

	bucket.refresh(now)

	state := &storage.BucketState{
		Tokens:     bucket.Tokens,
		Capacity:   capacity,
		Rate:       rate,
		LastUpdate: now,
	}
	if err := rl.storage.Save(clientID, state); err != nil {
		rl.log.Error("failed to save client", slog.String("client_id", clientID), sl.Err(err))
		return err
	}

	bucket.Capacity = capacity
	bucket.Rate = rate
	bucket.LastUpdate = now

	bucket.mu.Unlock()

	rl.log.Info("updated rate limit",
		slog.String("client_id", clientID),
		slog.Float64("capacity", capacity),
		slog.Float64("rate", rate),
	)

	return nil
}
