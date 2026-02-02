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
	flag.BoolVar(&verbose, "v", false, "enable debug logging")
	flag.Parse()

	setupLogger(os.Stdout, verbose)

	absDir, err := checkDirectory(dir)
	if err != nil {
		fatal(err, "invalid directory")
	}

	// apply OpenBSD-specific security restrictions if available
	if err = setupSecurity(absDir); err != nil {
		fatal(err, "failed to setup security restrictions")
	}

	fileServer := http.FileServerFS(os.DirFS(absDir))
	loggingServer := middleware.Logging(fileServer)
	http.Handle("/", loggingServer)

	server := &http.Server{
		Addr:    addr,
		Handler: http.DefaultServeMux,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// start the server in a goroutine
	go func() {
		slog.Info("starting file server", "address", addr, "directory", absDir)
		if listenErr := server.ListenAndServe(); listenErr != nil && !errors.Is(listenErr, http.ErrServerClosed) {
			slog.Error("failed to start file server", "error", listenErr)
		}
	}()

	// wait for interrupt signal
	<-stop
	slog.Info("shutting down server", "timeout", timeout)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err = server.Shutdown(ctx); err != nil {
		slog.Error("failed to shutdown server gracefully", "error", err)
	}

	slog.Info("stopped")
}

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
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
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

func fatal(err error, msg string) {
	slog.Error(msg, "error", err)
	os.Exit(1)
}
