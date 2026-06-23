package ipc

import (
	"context"
	"encoding/json"
	"net/http"
)

type Backend interface {
	Status() Status
	Download(ctx context.Context, r DownloadRequest) (DownloadResult, error)
}

func NewServer(token string, b Backend) http.Handler {
	mux := http.NewServeMux()
	auth := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-Token") != token {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next(w, r)
		}
	}
	mux.HandleFunc("/status", auth(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, b.Status())
	}))
	mux.HandleFunc("/download", auth(func(w http.ResponseWriter, r *http.Request) {
		var req DownloadRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		res, err := b.Download(r.Context(), req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, res)
	}))
	return mux
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
