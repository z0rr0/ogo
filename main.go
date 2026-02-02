// Package main implements a lightweight HTTP file server with OpenBSD-specific security features.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/z0rr0/ogo/middleware"
	"github.com/z0rr0/ogo/sandbox"
)

const (
	fatalDirCode = iota + 1
	fatalSandboxCode
	fatalServerCode
)

const serverTimeout = 3 * time.Second

func main() {
	var (
		dir     = "."
		addr    = ":8080"
		timeout = 5 * time.Second
		verbose bool
	)

	flag.StringVar(&addr, "a", addr, "listen address")
	flag.StringVar(&dir, "d", dir, "directory to show files")
	flag.DurationVar(&timeout, "t", timeout, "shutdown timeout")
	flag.BoolVar(&verbose, "v", verbose, "enable debug logging")
	flag.Parse()

	setupLogger(os.Stdout, verbose)

	absDir, err := checkDirectory(dir)
	if err != nil {
		fatal(fatalDirCode, err, "invalid directory")
	}

	// apply OpenBSD-specific security restrictions if available
	if err = setupSecurity(absDir); err != nil {
		fatal(fatalSandboxCode, err, "failed to setup security restrictions")
	}

	fileServer := http.FileServerFS(os.DirFS(absDir))
	loggingServer := middleware.Logging(fileServer)
	http.Handle("/", loggingServer)

	server := &http.Server{
		Addr:              addr,
		Handler:           http.DefaultServeMux,
		ReadHeaderTimeout: serverTimeout,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	errCh := make(chan error, 1)

	go func() {
		slog.Info("starting file server", "address", addr, "directory", absDir)

		listenErr := server.ListenAndServe()
		if listenErr != nil && !errors.Is(listenErr, http.ErrServerClosed) {
			errCh <- listenErr

			close(errCh)
		}
	}()

	select {
	case err = <-errCh:
		fatal(fatalServerCode, err, "server failed")
	case <-stop:
		slog.Info("shutting down server", "timeout", timeout)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err = server.Shutdown(ctx); err != nil {
		slog.Error("failed to shutdown server gracefully", "error", err)
	}

	slog.Info("stopped")
}

// checkDirectory verifies that the provided path is a valid directory.
func checkDirectory(dir string) (string, error) {
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

// setupSecurity applies OpenBSD-specific security restrictions using unveil and pledge.
func setupSecurity(absDir string) error {
	err := sandbox.Unveil(absDir, "r")
	if err != nil {
		return fmt.Errorf("failed to unveil directory: %w", err)
	}

	if err = sandbox.UnveilBlock(); err != nil {
		return fmt.Errorf("failed to block unveil: %w", err)
	}

	if err = sandbox.Pledge("stdio rpath inet"); err != nil {
		return fmt.Errorf("failed to pledge: %w", err)
	}

	return nil
}

// setupLogger configures the global logger with the specified output and verbosity.
func setupLogger(w io.Writer, verbose bool) {
	level := slog.LevelInfo
	timeFormat := time.RFC3339

	if verbose {
		level = slog.LevelDebug
		timeFormat = time.RFC3339Nano
	}

	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: verbose,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			switch a.Key {
			case slog.SourceKey:
				if src, ok := a.Value.Any().(*slog.Source); ok {
					src.File = filepath.Base(src.File)
					return slog.Any(slog.SourceKey, src)
				}
			case slog.TimeKey:
				t := a.Value.Time()
				return slog.String(slog.TimeKey, t.Format(timeFormat))
			}
			return a
		},
	}
	logger := slog.New(slog.NewTextHandler(w, opts))
	slog.SetDefault(logger)
}

// fatal logs the error message and exits the program with the specified code.
func fatal(code int, err error, msg string) {
	slog.Error(msg, "error", err)
	os.Exit(code)
}
