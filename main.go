package main

import (
	"log"
	"net/http"
	"os"
)

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func main() {
	port := getEnv("PORT", "8080")
	dataDir := getEnv("DATA_DIR", "./data")
	baseURL := getEnv("BASE_URL", "http://localhost:"+port)

	// Initialize storage
	storage, err := NewStorage(dataDir)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}

	// Initialize handlers
	handlers := NewHandlers(storage, baseURL)

	// Routes
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("kiss-drop is running"))
	})

	http.HandleFunc("/api/upload", handlers.HandleUpload)

	log.Printf("Starting kiss-drop on :%s", port)
	log.Printf("Data directory: %s", dataDir)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}
