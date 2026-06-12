package core_http_middleware

import (
	"context"
	core_http_response "getytstatsapi/internal/core/transport/http/response"
	"github.com/ra1phdd/logger"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type contextKey string

const requestIDHeader = "X-Request-ID"
const requestIDKey contextKey = "request_id"

func RequestID() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := r.Header.Get(requestIDHeader)
			if requestID == "" {
				requestID = uuid.NewString()
			}

			ctx := context.WithValue(r.Context(), requestIDKey, requestID)
			r = r.WithContext(ctx)

			r.Header.Set(requestIDHeader, requestID)
			w.Header().Set(requestIDHeader, requestID)

			next.ServeHTTP(w, r)
		})
	}
}

func RequestIDFromContext(ctx context.Context) (string, bool) {
	requestID, ok := ctx.Value(requestIDKey).(string)
	return requestID, ok
}

func Logger(log *logger.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID, _ := RequestIDFromContext(r.Context())

			reqLogger := log.With(
				logger.String("request_id", requestID),
				logger.String("url", r.URL.String()),
				logger.String("method", r.Method),
			)

			ctx := logger.NewContext(r.Context(), reqLogger)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func Panic() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			log := logger.FromContext(ctx)
			responseHandler := core_http_response.NewHTTPResponseHandler(log, w)

			defer func() {
				if p := recover(); p != nil {
					responseHandler.PanicResponse("during handle HTTP request got unexpected panic", p)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

func Trace() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			log := logger.FromContext(ctx)
			rw := core_http_response.NewResponseWriter(w)

			before := time.Now()
			log.Debug(
				">>> incoming HTTP request",
				logger.Time("time", before.UTC()),
			)

			next.ServeHTTP(rw, r)

			log.Debug(
				"<<< done HTTP request",
				logger.Int("status_code", rw.StatusCode()),
				logger.Duration("latency", time.Since(before)),
			)
		})
	}
}
