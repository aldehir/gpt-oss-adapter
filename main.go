package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/spf13/cobra"
)

var version = "dev"

var (
	listen        string
	target        string
	verbose       bool
	reasoningFrom string
	reasoningTo   string
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
	adapter := NewAdapter(target, cache, logger, reasoningFrom, reasoningTo)
	server := &http.Server{
		Addr:    listen,
		Handler: adapter,
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
	rootCmd.Flags().StringVar(&reasoningFrom, "reasoning-from", "reasoning_content", "Field name to use when reading reasoning from target server")
	rootCmd.Flags().StringVar(&reasoningTo, "reasoning-to", "reasoning", "Field name to use when sending reasoning to user")
}

func main() {
	Execute()
}
