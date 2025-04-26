package proxy

import (
	"loadbalancer/internal/balancer"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"time"
)

type ReverseProxy struct {
	balanver balancer.Balancer
	log      *slog.Logger
}

func NewReverseProxy(balancer balancer.Balancer, log *slog.Logger) *ReverseProxy {
	return &ReverseProxy{
		balanver: balancer,
		log:      log,
	}
}

func (p *ReverseProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	transport := &retryRoundTripper{
		next: &http.Transport{
			ResponseHeaderTimeout: 2 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
		maxRetries:  3,
		maxBackends: 5,
		balancer:    p.balanver,
		log:         p.log,
	}

	p.log.Info("proxy request",
		slog.String("method", r.Method),
		slog.String("path", r.URL.Path),
	)

	proxy := httputil.ReverseProxy{
		Director: func(request *http.Request) {
			request.Header.Add("X-Origin-Host", request.Host)
		},
		Transport: transport,
	}

	proxy.ServeHTTP(w, r)
}
