package proxy

import (
	"loadbalancer/internal/config"
	"net"
	"net/http"
	"sync"
)

type ConnectionPool struct {
	pool      []*http.Transport
	mu        sync.Mutex
	maxSize   int
	available chan int
	cfg       config.Proxy
}

// NewConnectionPool создает новый пул соединений заданного размера
func NewConnectionPool(cfg config.Proxy) *ConnectionPool {
	poolSize := cfg.ConnectionPoolSize
	if poolSize <= 0 {
		poolSize = 10
	}
	pool := make([]*http.Transport, poolSize)
	available := make(chan int, poolSize)

	// инициализируем транспорты и делаем доступным
	for i := 0; i < poolSize; i++ {
		pool[i] = createTransport(cfg)
		available <- i
	}

	return &ConnectionPool{
		pool:      pool,
		maxSize:   poolSize,
		available: available,
	}
}

func createTransport(cfg config.Proxy) *http.Transport {
	return &http.Transport{
		DialContext:           (&net.Dialer{Timeout: cfg.ProxyTransportOptions.DialTimeout}).DialContext,
		TLSHandshakeTimeout:   cfg.ProxyTransportOptions.TLSHandshakeTimeout,
		ResponseHeaderTimeout: cfg.ProxyTransportOptions.ResponseHeaderTimeout,
		ExpectContinueTimeout: cfg.ProxyTransportOptions.ExpectContinueTimeout,
		IdleConnTimeout:       cfg.ProxyTransportOptions.IdleConnTimeout,
		MaxIdleConns:          cfg.ProxyTransportOptions.MaxIdleConns,
		MaxIdleConnsPerHost:   cfg.ProxyTransportOptions.MaxIdleConnsPerHost,
	}
}

func (p *ConnectionPool) GetTransport() *http.Transport {
	idx := <-p.available

	p.mu.Lock()
	transport := p.pool[idx]
	p.mu.Unlock()

	return transport
}

func (p *ConnectionPool) ReleaseTransport(transport *http.Transport) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i, t := range p.pool {
		if t == transport {
			select {
			case p.available <- i:
			default:
			}
			return
		}
	}

	if len(p.pool) < p.maxSize {
		p.pool = append(p.pool, transport)
		p.available <- len(p.pool) - 1
	}
}

// Закрывает все соединения в пуле
func (p *ConnectionPool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	// закрываем все активные транспорты
	for _, transport := range p.pool {
		transport.CloseIdleConnections()
	}

	close(p.available)
}
