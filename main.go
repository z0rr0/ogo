package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/z0rr0/ogo/middleware"
	"github.com/z0rr0/ogo/sandbox"
)

var (
	loggerDebug = log.New(io.Discard, "DEBUG: ", log.Ldate|log.Ltime|log.Lmicroseconds|log.Llongfile)
	loggerInfo  = log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	loggerError = log.New(os.Stderr, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)
)

func main() {
	var (
		dir     string        = "."
		addr                  = ":8080"
		timeout time.Duration = 5 * time.Second
		verbose bool
	)

	flag.StringVar(&addr, "a", addr, "listen address")
	flag.StringVar(&dir, "d", dir, "directory to show files")
	flag.DurationVar(&timeout, "t", timeout, "shutdown timeout")
	flag.BoolVar(&verbose, "v", false, "enable debug logging")
	flag.Parse()

	if verbose {
		loggerDebug.SetOutput(os.Stdout)
	}

	if dir == "" {
		loggerError.Println("Error: -d flag is required")
		flag.Usage()
		os.Exit(1)
	}

	absDir, err := checkDirectory(dir)
	if err != nil {
		loggerError.Fatalf("failed directory: %v", err)
	}

	// apply OpenBSD-specific security restrictions if available
	if err := setupSecurity(absDir); err != nil {
		loggerError.Fatalf("failed to setup security: %v", err)
	}

	fileServer := http.FileServerFS(os.DirFS(absDir))
	loggingServer := middleware.Logging(fileServer, loggerDebug, loggerInfo)
	http.Handle("/", loggingServer)

	server := &http.Server{
		Addr:    addr,
		Handler: http.DefaultServeMux,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// start the server in a goroutine
	go func() {
		log.Printf("starting file server on %s serving %s", addr, absDir)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			loggerError.Fatalf("server error: %v", err)
		}
	}()

	// wait for interrupt signal
	<-stop
	log.Printf("shutting down server, timeout: %s", timeout)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		loggerError.Fatalf("server shutdown failed: %v", err)
	}

	log.Println("server stopped")
}

func checkDirectory(dir string) (string, error) {
	if dir == "" {
		return "", errors.New("directory cannot be empty")
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	info, err := os.Stat(absDir)
	if err != nil {
		return "", fmt.Errorf("failed to stat directory: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory", absDir)
	}

	return absDir, nil
}

func setupSecurity(absDir string) error {
	err := sandbox.Unveil(absDir, "r")
	if err != nil {
		return fmt.Errorf("failed to unveil directory: %w", err)
	}

	if err = sandbox.UnveilBlock(); err != nil {
		return fmt.Errorf("failed to block unveil: %w", err)
	}

	if err = sandbox.Pledge("stdio rpath inet dns"); err != nil {
		return fmt.Errorf("failed to pledge: %w", err)
	}

	return nil
}
