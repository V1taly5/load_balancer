package balancer

import (
	"loadbalancer/internal/config"
	"net/url"
	"sync"
)

type Backend struct {
	URL    *url.URL
	isDown bool
	mu     sync.RWMutex
}

func (b *Backend) IsDown() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.isDown
}

func (b *Backend) SetHealth(healthy bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.isDown = healthy
}

type Balancer interface {
	Next() (*Backend, error)
	MarkAsDown(backend *Backend)
	AddBackend(backend Backend)
	RemoveBackend(url string) bool
	GetAllBackends() []*Backend
}

func NewBackend(config config.Backend) (*Backend, error) {
	backUrl, err := url.Parse(config.URL)
	if err != nil {
		return nil, err
	}
	return &Backend{
		URL:    backUrl,
		isDown: false,
	}, nil
}
