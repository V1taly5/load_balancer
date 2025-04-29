package balancer

import (
	"errors"
	"math/rand"
	"sync"
	"time"
)

type RandomBalancer struct {
	backends []*Backend
	mu       sync.RWMutex
	rand     *rand.Rand
}

func NewRandomBalancer() *RandomBalancer {
	return &RandomBalancer{
		backends: make([]*Backend, 0),
		rand:     rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (rb *RandomBalancer) Next() (*Backend, error) {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	available := make([]*Backend, 0, len(rb.backends))
	for _, b := range rb.backends {
		b.mu.RLock()
		if !b.isDown {
			available = append(available, b)
		}
		b.mu.RUnlock()
	}

	if len(available) == 0 {
		return nil, errors.New("no available backends")
	}

	return available[rb.rand.Intn(len(available))], nil
}

func (rb *RandomBalancer) MarkAsDown(backend *Backend) {
	backend.mu.Lock()
	defer backend.mu.Unlock()
	backend.isDown = true
}

func (rb *RandomBalancer) AddBackend(backend Backend) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	newBackend := &Backend{
		isDown: backend.isDown,
	}

	if backend.URL != nil {
		copyURL := *backend.URL
		newBackend.URL = &copyURL
	}

	rb.backends = append(rb.backends, newBackend)
}

func (rb *RandomBalancer) RemoveBackend(urlStr string) bool {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	for i, b := range rb.backends {
		b.mu.RLock()
		u := ""
		if b.URL != nil {
			u = b.URL.String()
		}
		b.mu.RUnlock()

		if u == urlStr {
			rb.backends = append(rb.backends[:i], rb.backends[i+1:]...)
			return true
		}
	}
	return false
}

func (rb *RandomBalancer) GetAllBackends() []*Backend {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	backends := make([]*Backend, len(rb.backends))
	copy(backends, rb.backends)
	return backends
}
