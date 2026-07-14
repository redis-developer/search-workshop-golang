// Package httpapi is the HTTP layer of the workshop service. It is
// provided complete: the labs implement search behavior in
// internal/search and internal/embed, never HTTP plumbing.
package httpapi

import (
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"strconv"
	"strings"

	"github.com/redis-developer/search-workshop-golang/internal/search"
)

// Handler builds the service mux: the JSON API plus the embedded UI at /.
func Handler(svc *search.Service, ui fs.FS) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		status := map[string]any{"status": "ok", "index_ready": svc.Ready(r.Context())}
		if err := svc.Ping(r.Context()); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{
				"status": "degraded",
				"error":  "redis unreachable: " + err.Error(),
			})
			return
		}
		writeJSON(w, http.StatusOK, status)
	})

	mux.HandleFunc("GET /stats", func(w http.ResponseWriter, r *http.Request) {
		stats, err := svc.Stats(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, stats)
	})

	mux.HandleFunc("GET /products/{id}", func(w http.ResponseWriter, r *http.Request) {
		product, err := svc.Product(r.Context(), r.PathValue("id"))
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		if len(product) == 0 {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "product not found"})
			return
		}
		delete(product, search.FieldEmbedding) // raw vector bytes are not JSON-friendly
		writeJSON(w, http.StatusOK, product)
	})

	mux.HandleFunc("GET /search", func(w http.ResponseWriter, r *http.Request) {
		req, err := parseSearchRequest(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if !svc.Ready(r.Context()) {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{
				"error": "index not ready — run `make prep` and restart the service (or run `make reindex`)",
			})
			return
		}
		resp, err := svc.Search(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	})

	mux.HandleFunc("GET /facets", func(w http.ResponseWriter, r *http.Request) {
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		facets, err := svc.Facets(r.Context(), limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"facets": facets})
	})

	mux.Handle("GET /", http.FileServerFS(ui))
	return mux
}

func parseSearchRequest(r *http.Request) (search.Request, error) {
	q := r.URL.Query()
	req := search.Request{
		Query:    strings.TrimSpace(q.Get("query")),
		Strategy: strings.TrimSpace(q.Get("strategy")),
		Class:    strings.TrimSpace(q.Get("class")),
	}
	if req.Query == "" {
		return req, errors.New(`missing required parameter "query"`)
	}
	if v := q.Get("min_rating"); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return req, errors.New("min_rating must be a number")
		}
		req.MinRating = f
	}
	if v := q.Get("min_reviews"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return req, errors.New("min_reviews must be an integer")
		}
		req.MinReviews = n
	}
	if v := q.Get("k"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return req, errors.New("k must be a positive integer")
		}
		req.K = n
	}
	return req, nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v) //nolint:errcheck // client gone is not actionable
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
