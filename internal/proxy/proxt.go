package proxy

import (
	"loadbalancer/internal/balancer"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"time"
)

type ReverseProxy struct {
	balanver balancer.Balancer
	log      *slog.Logger
}

// cейчас создается новый транспорт при каждом запросе, что не очень хорошо
// по хорошему надо сделать транспорт переиспользуемым и наверное сделать pool транспортов
func NewReverseProxy(balancer balancer.Balancer, log *slog.Logger) *ReverseProxy {
	return &ReverseProxy{
		balanver: balancer,
		log:      log,
	}
}

var defaultTransport = &http.Transport{
	DialContext:           (&net.Dialer{Timeout: 5 * time.Second}).DialContext,
	TLSHandshakeTimeout:   5 * time.Second,
	ResponseHeaderTimeout: 2 * time.Second,
	ExpectContinueTimeout: 1 * time.Second,
	IdleConnTimeout:       90 * time.Second,
	MaxIdleConns:          100,
	MaxIdleConnsPerHost:   10,
}

func (p *ReverseProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	transport := &retryRoundTripper{
		next:        defaultTransport,
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
