// Command searchd is the workshop's product-search service: the JSON
// search API and the web UI on one port.
//
// On startup it makes the configured index queryable (embedding and
// loading the prepared WANDS sample if needed), then serves. All tuning
// comes from config.yaml; -algorithm/-model/-provider exist so the Lab 8
// index matrix can be scripted without editing the file.
//
// Usage:
//
//	go run ./cmd/searchd                     # ensure index, serve UI + API
//	go run ./cmd/searchd -reindex            # force rebuild, then serve
//	go run ./cmd/searchd -reindex-only       # force rebuild, then exit
//	go run ./cmd/searchd -reindex-only -algorithm hnsw
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis-developer/search-workshop-golang/internal/config"
	"github.com/redis-developer/search-workshop-golang/internal/httpapi"
	"github.com/redis-developer/search-workshop-golang/internal/search"
	"github.com/redis-developer/search-workshop-golang/web"
)

func main() {
	var (
		configPath  = flag.String("config", "config.yaml", "path to the workshop config file")
		reindex     = flag.Bool("reindex", false, "force a rebuild of the index before serving")
		reindexOnly = flag.Bool("reindex-only", false, "force a rebuild of the index, then exit")
		algorithm   = flag.String("algorithm", "", "override index.algorithm (flat|hnsw|svs-vamana)")
		model       = flag.String("model", "", "override embedding.model")
		provider    = flag.String("provider", "", "override embedding.provider (hf|openai)")
	)
	flag.Parse()

	if err := run(*configPath, *reindex, *reindexOnly, *algorithm, *model, *provider); err != nil {
		fmt.Fprintln(os.Stderr, "searchd:", err)
		os.Exit(1)
	}
}

func run(configPath string, reindex, reindexOnly bool, algorithm, model, provider string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	applyOverrides(cfg, algorithm, model, provider)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// A failure from a not-yet-implemented lab must not stop the show:
	// the UI and /healthz keep serving, and every search endpoint
	// reports which lab to open next. Reindex runs still fail loudly.
	var labErr error
	svc, err := search.New(ctx, cfg)
	if err != nil {
		if reindexOnly {
			return err
		}
		labErr = err
		svc = nil
	}
	if svc != nil {
		defer svc.Close() //nolint:errcheck // shutdown path
		if err := svc.EnsureIndex(ctx, reindex || reindexOnly); err != nil {
			if reindexOnly {
				return err
			}
			labErr = err
		}
	}
	if reindexOnly {
		return nil
	}
	if labErr != nil {
		fmt.Fprintf(os.Stderr, "searchd starting without search: %v\n", labErr)
	}

	server := &http.Server{
		Addr:              cfg.Server.Addr,
		Handler:           httpapi.Handler(svc, web.FS, labErr),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		fmt.Printf("searchd ready: UI and API on http://localhost%s (index %s)\n",
			cfg.Server.Addr, cfg.IndexName())
		errCh <- server.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		return server.Close()
	}
	return nil
}

func applyOverrides(cfg *config.Config, algorithm, model, provider string) {
	if algorithm != "" {
		cfg.Index.Algorithm = algorithm
	}
	if provider != "" {
		cfg.Embedding.Provider = provider
	}
	if model != "" {
		cfg.Embedding.Model = model
	}
}
