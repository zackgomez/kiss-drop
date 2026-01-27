package main

import (
	"encoding/json"
	"log"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
)

// Handlers holds HTTP handlers and their dependencies
type Handlers struct {
	storage *Storage
	baseURL string
}

// NewHandlers creates a new Handlers instance
func NewHandlers(storage *Storage, baseURL string) *Handlers {
	return &Handlers{
		storage: storage,
		baseURL: strings.TrimSuffix(baseURL, "/"),
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

	// Create the share (no password or expiration for now)
	meta, err := h.storage.CreateShare(file, fileName, header.Size, nil, "")
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

	// TODO: Check password in Phase 4

	filePath := h.storage.GetFilePath(id, meta.FileName)

	// Set headers for download
	w.Header().Set("Content-Disposition", "attachment; filename=\""+meta.FileName+"\"")
	w.Header().Set("Content-Type", "application/octet-stream")

	http.ServeFile(w, r, filePath)
}
