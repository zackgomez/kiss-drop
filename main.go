package main

import (
	"io/fs"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func parseDuration(s string, defaultDays int) time.Duration {
	// Parse as number of days (e.g., "30" or "30d")
	s = strings.TrimSuffix(s, "d")
	days, err := strconv.Atoi(s)
	if err != nil || days <= 0 {
		days = defaultDays
	}
	return time.Duration(days) * 24 * time.Hour
}

func startCleanupWorker(storage *Storage, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		// Run immediately on startup
		deleted, err := storage.CleanupExpired()
		if err != nil {
			log.Printf("Cleanup error: %v", err)
		} else if deleted > 0 {
			log.Printf("Cleaned up %d expired share(s)", deleted)
		}

		for range ticker.C {
			deleted, err := storage.CleanupExpired()
			if err != nil {
				log.Printf("Cleanup error: %v", err)
			} else if deleted > 0 {
				log.Printf("Cleaned up %d expired share(s)", deleted)
			}
		}
	}()
}

func main() {
	port := getEnv("PORT", "8080")
	dataDir := getEnv("DATA_DIR", "./data")
	baseURL := getEnv("BASE_URL", "http://localhost:"+port)
	defaultExpiry := parseDuration(getEnv("DEFAULT_EXPIRY", "30d"), 30)

	// Initialize storage
	storage, err := NewStorage(dataDir)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}

	// Initialize upload manager
	uploads, err := NewUploadManager(dataDir)
	if err != nil {
		log.Fatalf("Failed to initialize upload manager: %v", err)
	}

	// Start cleanup worker (runs every hour)
	startCleanupWorker(storage, time.Hour)

	// Load templates
	templates, err := LoadTemplates()
	if err != nil {
		log.Fatalf("Failed to load templates: %v", err)
	}

	// Initialize handlers
	handlers := NewHandlers(storage, uploads, baseURL, defaultExpiry)

	// Serve static files
	staticContent, err := fs.Sub(staticFS, "static")
	if err != nil {
		log.Fatalf("Failed to get static fs: %v", err)
	}
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticContent))))

	// UI Routes
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		handlers.HandleUploadPage(w, r, templates)
	})

	http.HandleFunc("/s/", func(w http.ResponseWriter, r *http.Request) {
		handlers.HandleDownloadPage(w, r, templates)
	})

	// API Routes
	http.HandleFunc("/api/upload", handlers.HandleUpload)
	http.HandleFunc("/api/upload/init", handlers.HandleUploadInit)
	http.HandleFunc("/api/upload/", func(w http.ResponseWriter, r *http.Request) {
		// Route chunked upload endpoints
		path := r.URL.Path
		if strings.Contains(path, "/chunk/") {
			handlers.HandleUploadChunk(w, r)
		} else if strings.HasSuffix(path, "/complete") {
			handlers.HandleUploadComplete(w, r)
		} else {
			http.NotFound(w, r)
		}
	})

	http.HandleFunc("/api/share/", func(w http.ResponseWriter, r *http.Request) {
		// Route to appropriate handler based on path
		if strings.HasSuffix(r.URL.Path, "/download") {
			handlers.HandleDownload(w, r)
		} else {
			handlers.HandleShareInfo(w, r)
		}
	})

	log.Printf("Starting kiss-drop on :%s", port)
	log.Printf("Data directory: %s", dataDir)
	log.Printf("Default expiry: %s", defaultExpiry)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}
