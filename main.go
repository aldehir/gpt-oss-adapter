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

var rootCmd = &cobra.Command{
	Use:   "gpt-oss-adapter",
	Short: "gpt-oss adapter to inject reasoning from tool calls",
	Long:  "gpt-oss adapter to inject reasoning from tool calls",
	Run: func(cmd *cobra.Command, args []string) {
		listen, _ := cmd.Flags().GetString("listen")
		target, _ := cmd.Flags().GetString("target")
		verbose, _ := cmd.Flags().GetBool("verbose")
		reasoningFrom, _ := cmd.Flags().GetString("reasoning-from")
		reasoningTo, _ := cmd.Flags().GetString("reasoning-to")
		if target == "" {
			fmt.Fprintf(os.Stderr, "Error: target argument is required\n")
			os.Exit(1)
		}
		startServer(listen, target, verbose, reasoningFrom, reasoningTo)
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func startServer(addr, target string, verbose bool, reasoningFrom, reasoningTo string) {
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
		Addr:    addr,
		Handler: adapter,
	}

	go func() {
		logger.Info("Starting server", "addr", addr)
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
	rootCmd.Flags().StringP("listen", "l", ":8005", "Address to listen on")
	rootCmd.Flags().StringP("target", "t", "", "Target URL to proxy requests to (required)")
	rootCmd.Flags().BoolP("verbose", "v", false, "Enable debug output")
	rootCmd.Flags().String("reasoning-from", "reasoning_content", "Field name to use when reading reasoning from target server")
	rootCmd.Flags().String("reasoning-to", "reasoning", "Field name to use when sending reasoning to user")
}

func main() {
	Execute()
}
