package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/aldehir/gpt-oss-adapter/providers/llamacpp"
	"github.com/aldehir/gpt-oss-adapter/providers/lmstudio"
	"github.com/aldehir/gpt-oss-adapter/providers/openrouter"
	"github.com/aldehir/gpt-oss-adapter/providers/types"
)

var version = "dev"

var (
	listen   string
	target   string
	verbose  bool
	provider string
)

var rootCmd = &cobra.Command{
	Use:     "gpt-oss-adapter",
	Short:   "gpt-oss adapter to inject reasoning from tool calls",
	Long:    "gpt-oss adapter to inject reasoning from tool calls",
	Version: version,
	Run: func(cmd *cobra.Command, args []string) {
		if target == "" {
			fmt.Fprintf(os.Stderr, "Error: target argument is required\n")
			os.Exit(1)
		}
		startServer()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func startServer() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	cache := NewLRUCache(1000)

	var logLevel slog.Level
	if verbose {
		logLevel = slog.LevelDebug
	} else {
		logLevel = slog.LevelInfo
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))
	providerConfig := getProviderConfig(provider)
	adapter := NewAdapter(target, cache, logger, providerConfig)

	// Wrap adapter with logging middleware
	handler := NewLoggingMiddleware(adapter, logger)

	server := &http.Server{
		Addr:    listen,
		Handler: handler,
	}

	go func() {
		logger.Info("Starting server", "addr", listen)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Server failed to start", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("Shutting down server")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("Server forced to shutdown", "error", err)
		os.Exit(1)
	}

	logger.Info("Server exited")
}

func init() {
	rootCmd.Flags().StringVarP(&listen, "listen", "l", ":8005", "Address to listen on")
	rootCmd.Flags().StringVarP(&target, "target", "t", "", "Target URL to proxy requests to (required)")
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable debug output")
	rootCmd.Flags().StringVarP(&provider, "provider", "p", "llama-cpp", "Backend provider (lmstudio, llama-cpp, openrouter)")
}

// LoggingMiddleware wraps an http.Handler and logs HTTP requests in Apache/nginx format
type LoggingMiddleware struct {
	handler http.Handler
	logger  *slog.Logger
}

// NewLoggingMiddleware creates a new HTTP logging middleware
func NewLoggingMiddleware(handler http.Handler, logger *slog.Logger) *LoggingMiddleware {
	return &LoggingMiddleware{
		handler: handler,
		logger:  logger,
	}
}

// responseWriter wraps http.ResponseWriter to capture status code and response size
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	size       int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	size, err := rw.ResponseWriter.Write(b)
	rw.size += size
	return size, err
}

func (rw *responseWriter) Flush() {
	if flusher, ok := rw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (m *LoggingMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// Wrap the response writer to capture status and size
	rw := &responseWriter{
		ResponseWriter: w,
		statusCode:     200, // default status code
		size:           0,
	}

	// Call the wrapped handler
	m.handler.ServeHTTP(rw, r)

	// Calculate request duration
	duration := time.Since(start)

	// Get client IP, preferring X-Forwarded-For or X-Real-IP headers
	clientIP := getClientIP(r)

	// Get user agent
	userAgent := r.Header.Get("User-Agent")
	if userAgent == "" {
		userAgent = "-"
	}

	// Get referer
	referer := r.Header.Get("Referer")
	if referer == "" {
		referer = "-"
	}

	// Log using structured logging fields
	m.logger.Info("HTTP request",
		"client_ip", clientIP,
		"method", r.Method,
		"path", r.RequestURI,
		"protocol", r.Proto,
		"status", rw.statusCode,
		"size", rw.size,
		"referer", referer,
		"user_agent", userAgent,
		"duration_ms", duration.Milliseconds(),
	)
}

// getClientIP extracts the client IP from the request, checking proxy headers first
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For can contain multiple IPs, take the first one
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fall back to RemoteAddr
	if idx := strings.LastIndex(r.RemoteAddr, ":"); idx != -1 {
		return r.RemoteAddr[:idx]
	}

	return r.RemoteAddr
}

func getProviderConfig(provider string) types.Provider {
	switch provider {
	case "lmstudio":
		return lmstudio.NewProvider()
	case "llama-cpp":
		return llamacpp.NewProvider()
	case "openrouter":
		return openrouter.NewProvider()
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown provider %s, defaulting to lmstudio\n", provider)
		return lmstudio.NewProvider()
	}
}

func main() {
	Execute()
}
