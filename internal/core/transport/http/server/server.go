package core_http_server

import (
	"context"
	"errors"
	"fmt"
	core_http_middleware "getytstatsapi/internal/core/transport/http/middleware"
	"getytstatsapi/pkg/logger"
	"net/http"
	"time"
)

type HTTPServer struct {
	addr string
	mux  *http.ServeMux
	log  *logger.Logger

	middleware []core_http_middleware.Middleware
}

func NewHTTPServer(
	addr string,
	log *logger.Logger,
	middleware ...core_http_middleware.Middleware,
) *HTTPServer {
	return &HTTPServer{
		addr:       addr,
		mux:        http.NewServeMux(),
		log:        log,
		middleware: middleware,
	}
}

func (h *HTTPServer) RegisterAPIRouters(routers ...*APIVersionRouter) {
	for _, router := range routers {
		prefix := "/" + string(router.apiVersion)

		h.mux.Handle(
			prefix+"/",
			http.StripPrefix(prefix, router),
		)
	}
}

func (h *HTTPServer) Run(ctx context.Context) error {
	server := &http.Server{
		Addr:    h.addr,
		Handler: core_http_middleware.ChainMiddleware(h.mux, h.middleware...),
	}

	ch := make(chan error, 1)

	go func() {
		defer close(ch)

		h.log.Info("start HTTP server", logger.String("addr", h.addr))

		err := server.ListenAndServe()
		if !errors.Is(err, http.ErrServerClosed) {
			ch <- err
		}
	}()

	select {
	case err := <-ch:
		if err != nil {
			return fmt.Errorf("listen and server HTTP: %w", err)
		}

	case <-ctx.Done():
		h.log.Warn("shutdown HTTP server", logger.Err(ctx.Err()))

		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			//h.config.ShutdownTimeout,
			5*time.Second,
		)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			_ = server.Close()
			return fmt.Errorf("shutdown HTTP server: %w", err)
		}

		h.log.Warn("HTTP server stopped")
	}

	return nil
}
