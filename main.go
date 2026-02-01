package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

const (
	addr           = ":8080"
	defaultTimeout = 5 * time.Second
)

func main() {
	var (
		dir     string
		timeout time.Duration
	)

	flag.StringVar(&dir, "d", "", "directory to show files (required)")
	flag.DurationVar(&timeout, "t", defaultTimeout, "shutdown timeout")
	flag.Parse()

	if dir == "" {
		fmt.Fprintln(os.Stderr, "Error: -d flag is required")
		flag.Usage()
		os.Exit(1)
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		log.Fatalf("failed to get absolute path: %v", err)
	}

	info, err := os.Stat(absDir)
	if err != nil {
		log.Fatalf("failed to stat directory: %v", err)
	}
	if !info.IsDir() {
		log.Fatalf("%s is not a directory", absDir)
	}

	// Apply OpenBSD-specific security restrictions if available
	if err := setupSecurity(absDir); err != nil {
		log.Fatalf("failed to setup security: %v", err)
	}

	fileServer := http.FileServer(http.Dir(absDir))
	http.Handle("/", fileServer)

	server := &http.Server{
		Addr:    addr,
		Handler: http.DefaultServeMux,
	}

	// Channel to listen for interrupt signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Start server in a goroutine
	go func() {
		log.Printf("Starting file server on %s serving %s", addr, absDir)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	<-stop
	log.Println("Shutting down server...")

	// Create context with timeout for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("server shutdown failed: %v", err)
	}

	log.Println("Server stopped")
}
