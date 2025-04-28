package balancer

import (
	"errors"
	"log/slog"
	"sync"
)

var (
	ErrNoAvailableBackends = errors.New("no available backends")
)

type RoundRobinBalancer struct {
	backends []*Backend
	current  int
	mu       sync.Mutex
	log      *slog.Logger
}

func NewRoundRobinBalancer(log *slog.Logger) *RoundRobinBalancer {
	return &RoundRobinBalancer{
		backends: make([]*Backend, 0),
		current:  0,
		log:      log,
	}
}

func (rr *RoundRobinBalancer) Next() (*Backend, error) {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	backendCount := len(rr.backends)
	if backendCount == 0 {
		return nil, ErrNoAvailableBackends
	}

	// rr.checkDownBackends()

	availableCount := 0
	for _, backend := range rr.backends {
		if !backend.IsDown() {
			availableCount++
		}
	}

	if availableCount == 0 {
		return nil, ErrNoAvailableBackends
	}

	startIndex := rr.current
	for i := 0; i < backendCount; i++ {
		index := (startIndex + i) % backendCount
		backend := rr.backends[index]

		if !backend.IsDown() {
			rr.current = (index + 1) % backendCount
			return backend, nil
		}
	}
	return nil, ErrNoAvailableBackends
}

func (rr *RoundRobinBalancer) MarkAsDown(backend *Backend) {
	if backend == nil {
		return
	}

	//TODO: можно ставить пометку isDown после нескольких неудачных запросов
	backend.SetHealth(true)
	rr.log.Error("backend marked as down", slog.String("url", backend.URL.String()))
}

func (rr *RoundRobinBalancer) checkDownBackends() {
	for _, backend := range rr.backends {
		if backend.IsDown() {
			backend.SetHealth(false)
			rr.log.Info("backend marked as up", slog.String("url", backend.URL.String()))
		}
	}
}

func (rr *RoundRobinBalancer) AddBackend(backend Backend) {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	backendCopy := backend
	rr.backends = append(rr.backends, &backendCopy)
	rr.log.Info("backend added", slog.String("url", backend.URL.String()))
}

func (rr *RoundRobinBalancer) RemoveBackend(url string) bool {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	for index, backend := range rr.backends {
		if backend.URL.String() == url {
			rr.backends = append(rr.backends[:index], rr.backends[index+1:]...)
			rr.log.Info("backend removed", slog.String("url", backend.URL.String()))
			return true
		}
	}
	return false
}

func (rr *RoundRobinBalancer) GetAllBackends() []*Backend {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	backendsCopy := make([]*Backend, len(rr.backends))
	copy(backendsCopy, rr.backends)

	return backendsCopy
}
