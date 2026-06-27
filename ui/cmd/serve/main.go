package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	fs := http.FileServer(http.Dir("public"))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := "public" + r.URL.Path
		if _, err := os.Stat(path); os.IsNotExist(err) {
			w.Header().Set("Cache-Control", "no-cache")
			http.ServeFile(w, r, "public/index.html")
			return
		}
		// Hashed assets get long cache; index.html gets no-cache
		if r.URL.Path == "/" || r.URL.Path == "/index.html" {
			w.Header().Set("Cache-Control", "no-cache")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		fs.ServeHTTP(w, r)
	})

	addr := ":" + port
	fmt.Printf("Convocate UI server listening on %s\n", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
