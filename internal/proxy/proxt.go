package proxy

import (
	"loadbalancer/internal/balancer"
	"loadbalancer/internal/config"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"sync"
)

type ReverseProxy struct {
	balanver       balancer.Balancer
	connectionPool *ConnectionPool
	cfg            config.Proxy
	log            *slog.Logger
}

func NewReverseProxy(balancer balancer.Balancer, log *slog.Logger, cfg config.Proxy) *ReverseProxy {

	return &ReverseProxy{
		balanver:       balancer,
		cfg:            cfg,
		log:            log,
		connectionPool: NewConnectionPool(cfg),
	}
}

func (p *ReverseProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	p.log.Info("proxy request",
		slog.String("method", r.Method),
		slog.String("path", r.URL.Path),
	)

	transport := p.connectionPool.GetTransport()

	retryTransport := &retryRoundTripper{
		next:        transport,
		maxRetries:  p.cfg.MaxRetries,
		maxBackends: p.cfg.MaxBackends,
		balancer:    p.balanver,
		log:         p.log,
		mu:          &sync.Mutex{},
		pool:        p.connectionPool,
	}

	proxy := httputil.ReverseProxy{
		Director: func(request *http.Request) {
			request.Header.Add("X-Origin-Host", request.Host)
		},
		Transport: retryTransport,
	}

	proxy.ServeHTTP(w, r)
}

func (p *ReverseProxy) Close() {
	if p.connectionPool != nil {
		p.connectionPool.Close()
	}
}
