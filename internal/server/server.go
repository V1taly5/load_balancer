package server

import (
	"context"
	"fmt"
	"loadbalancer/internal/config"
	"loadbalancer/internal/lib/sl"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type Server struct {
	server *http.Server
	log    *slog.Logger
}

// feat: added HTTP server
func New(handler http.Handler, cfg *config.HTTPServer, log *slog.Logger) *Server {
	return &Server{
		server: &http.Server{
			Addr:         fmt.Sprintf(":%d", cfg.Port),
			Handler:      handler,
			ReadTimeout:  cfg.Timeout,
			WriteTimeout: cfg.Timeout,
			IdleTimeout:  cfg.IdleTimeout,
		},
		log: log,
	}
}

func (s *Server) Start() error {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	errCh := make(chan error)

	s.log.Info("starting server", slog.String("address", s.server.Addr))

	go func() {
		errCh <- s.server.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		s.log.Error("server error", sl.Err(err))
		return err
	case <-stop:
		s.log.Info("shutdown signal")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.server.Shutdown(ctx); err != nil {
			s.log.Error("server shutdown error", sl.Err(err))
			return err
		}

		s.log.Info("server stopped gracefully")
	}
	return nil
}
