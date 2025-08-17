package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "gpt-oss-adapter",
	Short: "A CLI application built with Cobra",
	Long:  "A longer description of your CLI application and what it does.",
	Run: func(cmd *cobra.Command, args []string) {
		listen, _ := cmd.Flags().GetString("listen")
		startServer(listen)
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func startServer(addr string) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	adapter := NewAdapter()
	server := &http.Server{
		Addr:    addr,
		Handler: adapter,
	}

	go func() {
		log.Printf("Starting server on %s", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("Shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}

func init() {
	rootCmd.Flags().StringP("listen", "l", ":8080", "Address to listen on")
}

func main() {
	Execute()
}
