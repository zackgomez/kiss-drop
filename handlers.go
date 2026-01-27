package main

import (
	"encoding/json"
	"log"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Handlers holds HTTP handlers and their dependencies
type Handlers struct {
	storage       *Storage
	auth          *Auth
	baseURL       string
	defaultExpiry time.Duration
}

// NewHandlers creates a new Handlers instance
func NewHandlers(storage *Storage, auth *Auth, baseURL string, defaultExpiry time.Duration) *Handlers {
	return &Handlers{
		storage:       storage,
		auth:          auth,
		baseURL:       strings.TrimSuffix(baseURL, "/"),
		defaultExpiry: defaultExpiry,
	}
}

// sanitizeFileName cleans up a filename for safe storage
func sanitizeFileName(name string) string {
	// Remove path components
	name = filepath.Base(name)

	// Replace problematic characters
	reg := regexp.MustCompile(`[^a-zA-Z0-9._-]`)
	name = reg.ReplaceAllString(name, "_")

	// Limit length
	if len(name) > 200 {
		ext := filepath.Ext(name)
		name = name[:200-len(ext)] + ext
	}

	if name == "" || name == "." || name == ".." {
		name = "file"
	}

	return name
}

// HandleUpload handles POST /api/upload
func (h *Handlers) HandleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart form (max 10GB)
	if err := r.ParseMultipartForm(10 << 30); err != nil {
		log.Printf("Error parsing form: %v", err)
		http.Error(w, "Error parsing upload", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		log.Printf("Error getting file: %v", err)
		http.Error(w, "No file provided", http.StatusBadRequest)
		return
	}
	defer file.Close()

	fileName := sanitizeFileName(header.Filename)

	// Handle optional password
	var passwordHash string
	if password := r.FormValue("password"); password != "" {
		hash, err := HashPassword(password)
		if err != nil {
			log.Printf("Error hashing password: %v", err)
			http.Error(w, "Error processing password", http.StatusInternalServerError)
			return
		}
		passwordHash = hash
	}

	// Handle expiration
	var expiresAt *time.Time
	expiresInStr := r.FormValue("expires_in")
	if expiresInStr == "" || expiresInStr == "default" {
		// Use default expiry
		if h.defaultExpiry > 0 {
			t := time.Now().Add(h.defaultExpiry)
			expiresAt = &t
		}
	} else if expiresInStr != "never" {
		// Parse as days
		days, err := strconv.Atoi(expiresInStr)
		if err == nil && days > 0 {
			t := time.Now().Add(time.Duration(days) * 24 * time.Hour)
			expiresAt = &t
		}
	}
	// "never" means no expiration (expiresAt stays nil)

	// Create the share
	meta, err := h.storage.CreateShare(file, fileName, header.Size, expiresAt, passwordHash)
	if err != nil {
		log.Printf("Error creating share: %v", err)
		http.Error(w, "Error saving file", http.StatusInternalServerError)
		return
	}

	// Return response
	response := map[string]string{
		"id":  meta.ID,
		"url": h.baseURL + "/s/" + meta.ID,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ShareInfoResponse is the JSON response for share metadata
type ShareInfoResponse struct {
	ID               string  `json:"id"`
	FileName         string  `json:"fileName"`
	FileSize         int64   `json:"fileSize"`
	ExpiresAt        *string `json:"expiresAt,omitempty"`
	PasswordRequired bool    `json:"passwordRequired"`
}

// HandleShareInfo handles GET /api/share/:id
func (h *Handlers) HandleShareInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/share/")
	if id == "" || strings.Contains(id, "/") {
		http.Error(w, "Invalid share ID", http.StatusBadRequest)
		return
	}

	meta, err := h.storage.GetShare(id)
	if err != nil {
		log.Printf("Error getting share: %v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	if meta == nil {
		http.Error(w, "Share not found", http.StatusNotFound)
		return
	}

	response := ShareInfoResponse{
		ID:               meta.ID,
		FileName:         meta.FileName,
		FileSize:         meta.FileSize,
		PasswordRequired: meta.PasswordHash != "",
	}
	if meta.ExpiresAt != nil {
		exp := meta.ExpiresAt.Format("2006-01-02T15:04:05Z")
		response.ExpiresAt = &exp
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleUnlock handles POST /api/share/:id/unlock
func (h *Handlers) HandleUnlock(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract ID from path like /api/share/abc123/unlock
	path := strings.TrimPrefix(r.URL.Path, "/api/share/")
	path = strings.TrimSuffix(path, "/unlock")
	id := path

	if id == "" || strings.Contains(id, "/") {
		http.Error(w, "Invalid share ID", http.StatusBadRequest)
		return
	}

	meta, err := h.storage.GetShare(id)
	if err != nil {
		log.Printf("Error getting share: %v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	if meta == nil {
		http.Error(w, "Share not found", http.StatusNotFound)
		return
	}

	// Parse password from JSON body
	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Verify password
	if meta.PasswordHash == "" || !VerifyPassword(body.Password, meta.PasswordHash) {
		http.Error(w, "Invalid password", http.StatusUnauthorized)
		return
	}

	// Set unlock cookie
	h.auth.SetUnlockCookie(w, id)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// HandleDownload handles GET /api/share/:id/download
func (h *Handlers) HandleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract ID from path like /api/share/abc123/download
	path := strings.TrimPrefix(r.URL.Path, "/api/share/")
	path = strings.TrimSuffix(path, "/download")
	id := path

	if id == "" || strings.Contains(id, "/") {
		http.Error(w, "Invalid share ID", http.StatusBadRequest)
		return
	}

	meta, err := h.storage.GetShare(id)
	if err != nil {
		log.Printf("Error getting share: %v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	if meta == nil {
		http.Error(w, "Share not found", http.StatusNotFound)
		return
	}

	// Check password protection
	if meta.PasswordHash != "" && !h.auth.IsUnlocked(r, id) {
		http.Error(w, "Password required", http.StatusUnauthorized)
		return
	}

	filePath := h.storage.GetFilePath(id, meta.FileName)

	// Set headers for download
	w.Header().Set("Content-Disposition", "attachment; filename=\""+meta.FileName+"\"")
	w.Header().Set("Content-Type", "application/octet-stream")

	http.ServeFile(w, r, filePath)
}
