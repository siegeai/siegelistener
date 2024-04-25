package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/siegeai/siegelistener/_stress/fake"
)

func main() {
	mux := http.NewServeMux()
	mux.Handle("/", handler())

	server := http.Server{Addr: "0.0.0.0:8080", Handler: mux}
	err := server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		panic(err)
	}
}

func handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, err := io.Copy(io.Discard, r.Body)
		if err != nil {
			panic(err)
		}

		obj := fake.JSON()
		buf := &bytes.Buffer{}
		if err = json.NewEncoder(buf).Encode(&obj); err != nil {
			panic(err)
		}

		gz := gzip.NewWriter(w)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Encoding", "gzip")
		w.WriteHeader(http.StatusOK)

		_, _ = gz.Write(buf.Bytes())
		_ = gz.Close()
		//_, _ = buf.WriteTo(gz)

		slog.Info("completed response", "url", r.URL)
	}
}
