package main

import (
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"
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
	cookieSecret := os.Getenv("COOKIE_SECRET") // Empty = random per restart

	// Initialize storage
	storage, err := NewStorage(dataDir)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}

	// Initialize auth
	auth := NewAuth(cookieSecret)

	// Load templates
	templates, err := LoadTemplates()
	if err != nil {
		log.Fatalf("Failed to load templates: %v", err)
	}

	// Initialize handlers
	handlers := NewHandlers(storage, auth, baseURL)

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
	http.HandleFunc("/api/share/", func(w http.ResponseWriter, r *http.Request) {
		// Route to appropriate handler based on path
		if strings.HasSuffix(r.URL.Path, "/download") {
			handlers.HandleDownload(w, r)
		} else if strings.HasSuffix(r.URL.Path, "/unlock") {
			handlers.HandleUnlock(w, r)
		} else {
			handlers.HandleShareInfo(w, r)
		}
	})

	log.Printf("Starting kiss-drop on :%s", port)
	log.Printf("Data directory: %s", dataDir)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}
