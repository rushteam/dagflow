package http

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

type Server struct {
	srv *http.Server
}

func NewServer(port string, handler http.Handler) *Server {
	return &Server{
		srv: &http.Server{
			Addr:              port,
			Handler:           handler,
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      60 * time.Second,
			IdleTimeout:       120 * time.Second,
		},
	}
}

func (s *Server) Start() {
	go func() {
		slog.Info("HTTP 服务启动", "addr", s.srv.Addr)
		if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP 服务启动失败", "error", err)
		}
	}()
}

func (s *Server) GracefulShutdown(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	slog.Info("开始优雅关闭 HTTP 服务器...")
	return s.srv.Shutdown(ctx)
}
